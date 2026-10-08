"""Chat pipeline (advanced RAG):

  user picks a category (or "others" = no filter)
  question -> rewrite into standalone query (only when there is history)
           -> embed query
           -> [semantic] Pinecone top-N, category filter, similarity threshold
           -> [keyword]  BM25 top-N over chunks.json, same category filter
           -> nothing from either?  -> canned fallback (LLM never called)
           -> Reciprocal Rank Fusion (k = 60)
           -> rerank the fused pool (LLM as reranker, query+chunk read together)
           -> keep chunks above the rerank cut-off, top FINAL_TOP_K
           -> answer from that context only
"""

from __future__ import annotations

import json
from dataclasses import replace

from common import connect_index, embed_texts, namespace, new_openai_client
from hybrid import BM25Index, Candidate, fuse_rrf
from index import CHUNK_STORE_PATH, categories_of, load_chunk_store

CHAT_MODEL = "gpt-5.4-mini"

# Retrieval
CANDIDATE_POOL_K = 20  # how many chunks EACH search fetches (wide net)
SIMILARITY_THRESHOLD = 0.75  # min cosine score for a semantic hit; text-embedding-3 scores run low, tune this
RRF_K = 60  # standard RRF constant

# Reranking
ENABLE_RERANK = True  # turn off for small/simple corpora to save a model call
RERANK_MIN_SCORE = 0.5  # chunks the reranker scores below this are dropped
RERANK_DOC_CHARS = 1500  # chunk text sent to the reranker is truncated to this

# What finally reaches the answering LLM
FINAL_TOP_K = 4

MAX_HISTORY_TURNS = 6  # question/answer pairs kept in chat history
OTHER_CATEGORY = "others"  # "others" = search across every category

FALLBACK_ANSWER = "I could not find the relevant information about this query."

REWRITE_PROMPT = """You are a query rewriting expert.
Based on the provided chat history, rephrase the follow-up user question
into a complete, standalone question that can be understood without chat history.
Only output the rewritten question and nothing else."""

RERANK_PROMPT = """You are a reranking model.
You will get a user query and a numbered list of documents.
For EACH document independently, judge how useful it is for answering the query
and give it a relevance score between 0 and 1:
  1.0 = directly answers the query
  0.5 = partially relevant / useful background
  0.0 = unrelated
Return ONLY a JSON object, no markdown and no other text, in exactly this form:
{"scores":[{"id":1,"score":0.92},{"id":2,"score":0.10}]}
Include every document id exactly once."""

ANSWER_PROMPT_TEMPLATE = """You have to behave like a Data Structure and Algorithm Expert.
You will be given a context of relevant information and a user question.
Your task is to answer the user's question based ONLY on the provided context.
If the answer is not in the context, you must say:
"I could not find the relevant information about this query."
Keep your answers clear, concise, and educational.

Context:
{context}"""


# ==========================================================
# RAGChat
# ==========================================================


class RAGChat:
    def __init__(self) -> None:
        records = load_chunk_store(CHUNK_STORE_PATH)
        self.index = connect_index()
        self.ai = new_openai_client()
        self.bm25 = BM25Index(records)
        self.categories = categories_of(records)
        self.category = OTHER_CATEGORY
        self.history: list[dict] = []

    def filter_category(self) -> str:
        """The category to filter on, or "" for no filter."""
        return "" if self.category == OTHER_CATEGORY else self.category

    def remember(self, question: str, answer: str) -> None:
        self.history += [
            {"role": "user", "content": question},
            {"role": "assistant", "content": answer},
        ]
        self.history = self.history[-MAX_HISTORY_TURNS * 2 :]

    def complete(self, system: str, user: str, with_history: bool) -> str:
        """Call the chat model. with_history=False is used for the reranker,
        which must judge relevance without being swayed by the conversation."""
        messages = [{"role": "system", "content": system}]
        if with_history:
            messages += self.history
        messages.append({"role": "user", "content": user})

        resp = self.ai.chat.completions.create(model=CHAT_MODEL, messages=messages)
        if not resp.choices:
            raise RuntimeError("empty response from model")
        return (resp.choices[0].message.content or "").strip()

    def transform_query(self, question: str) -> str:
        if not self.history:
            return question  # nothing to resolve against, skip the LLM call
        try:
            rewritten = self.complete(REWRITE_PROMPT, question, with_history=True)
        except Exception as exc:
            raise RuntimeError(f"rewrite query: {exc}") from exc
        return rewritten or question

    # ------------------------------------------------------
    # Semantic search (Pinecone + metadata filter + threshold)
    # ------------------------------------------------------

    def semantic_search(self, vector: list[float]) -> list[Candidate]:
        request = {
            "vector": vector,
            "top_k": CANDIDATE_POOL_K,
            "include_metadata": True,
            "namespace": namespace(),
        }
        if cat := self.filter_category():
            request["filter"] = {"category": {"$eq": cat}}  # category == "<selected>"

        try:
            results = self.index.query(**request)
        except Exception as exc:
            raise RuntimeError(f"pinecone query: {exc}") from exc

        out: list[Candidate] = []
        rejected = 0
        for m in results.matches or []:
            md = m.metadata or {}
            text = md.get("text") or ""
            if not text.strip():
                continue
            if m.score is None or m.score < SIMILARITY_THRESHOLD:
                rejected += 1
                continue
            out.append(
                Candidate(
                    id=m.id,
                    text=text,
                    source=str(md.get("source", "")),
                    page=int(md.get("page", 0) or 0),  # Pinecone returns numbers as floats
                    category=str(md.get("category", "")),
                    semantic_rank=len(out) + 1,
                    semantic_score=float(m.score),
                )
            )

        print(f"Semantic : {len(out)} accepted, {rejected} below threshold {SIMILARITY_THRESHOLD:.2f}")
        return out

    # ------------------------------------------------------
    # Reranking (LLM as a cross-encoder style reranker)
    # ------------------------------------------------------
    #
    # The embedding model embeds the query and each chunk SEPARATELY, then
    # compares vectors. A reranker reads the query and the chunk TOGETHER and
    # scores "can this chunk answer this query?", which is far more precise.
    # It is slower, so it only runs on the small fused pool.

    def rerank(self, query: str, cands: list[Candidate]) -> list[Candidate]:
        parts = [f"Query: {query}\n"]
        for i, cand in enumerate(cands, start=1):
            parts.append(f"Document {i}:\n{truncate(cand.text, RERANK_DOC_CHARS)}\n")
        raw = self.complete(RERANK_PROMPT, "\n".join(parts), with_history=False)

        # Be forgiving about stray text / markdown fences around the JSON.
        start, end = raw.find("{"), raw.rfind("}")
        if start < 0 or end <= start:
            raise ValueError(f"reranker returned no JSON: {raw!r}")
        parsed = json.loads(raw[start : end + 1])

        # anything the model skipped counts as irrelevant
        out = [replace(c, rerank_score=0.0) for c in cands]
        for item in parsed.get("scores", []):
            idx = int(item.get("id", 0))
            if 1 <= idx <= len(out):
                out[idx - 1].rerank_score = min(max(float(item.get("score", 0)), 0.0), 1.0)

        out.sort(key=lambda c: c.rerank_score, reverse=True)
        return out

    # ------------------------------------------------------
    # Chat
    # ------------------------------------------------------

    def chat(self, question: str) -> str:
        # STEP 1: Rewrite question into a standalone query.
        query = self.transform_query(question)
        print("\n--- Rewritten Query ---")
        print(query)
        print(f"\n--- Retrieval (category: {self.category}) ---")

        # STEP 2: Generate query embedding.
        try:
            vector = embed_texts(self.ai, [query])[0]
        except Exception as exc:
            raise RuntimeError(f"embed query: {exc}") from exc

        # STEP 3: Semantic search (metadata filter + similarity threshold).
        semantic = self.semantic_search(vector)

        # STEP 4: Keyword search (BM25, same category filter).
        keyword = self.bm25.search(query, CANDIDATE_POOL_K, self.filter_category())
        print(f"Keyword  : {len(keyword)} BM25 hits")

        # STEP 5: Nothing relevant anywhere -> canned answer, LLM is never called.
        if not semantic and not keyword:
            print("\n⚠️ No relevant chunks found (threshold / category / keywords).")
            self.remember(question, FALLBACK_ANSWER)
            return FALLBACK_ANSWER

        # STEP 6: Merge both rankings with Reciprocal Rank Fusion.
        fused = fuse_rrf(RRF_K, semantic, keyword)[:CANDIDATE_POOL_K]
        print_candidates("Fused with RRF (k=60)", fused)

        # STEP 7: Rerank the fused pool.
        final = fused
        if ENABLE_RERANK:
            try:
                reranked = self.rerank(query, fused)
            except Exception as exc:
                # Reranking is an improvement, not a requirement: keep RRF order.
                print("⚠️ Rerank failed, falling back to RRF order:", exc)
            else:
                print_candidates("Reranked", reranked, show_rerank=True)
                final = [c for c in reranked if c.rerank_score >= RERANK_MIN_SCORE]

        # STEP 8: Keep the best few for the LLM.
        final = final[:FINAL_TOP_K]
        if not final:
            print(f"\n⚠️ Reranker judged every chunk irrelevant (< {RERANK_MIN_SCORE:.2f}).")
            self.remember(question, FALLBACK_ANSWER)
            return FALLBACK_ANSWER

        context = "\n\n---\n\n".join(
            f"[Source: {c.source}, page {c.page + 1}, category: {c.category}]\n{c.text}" for c in final
        )
        print(f"\n--- Context sent to LLM ({len(final)} chunks) ---")
        print(context)
        print("\n------------------------------------")

        # STEP 9: Generate answer from retrieved context.
        try:
            answer = self.complete(ANSWER_PROMPT_TEMPLATE.format(context=context), query, with_history=True)
        except Exception as exc:
            raise RuntimeError(f"generate answer: {exc}") from exc

        # STEP 10: Save conversation history.
        self.remember(question, answer)
        return answer


# ==========================================================
# Helpers
# ==========================================================


def truncate(s: str, n: int) -> str:
    return s if len(s) <= n else s[:n] + "…"


def print_candidates(title: str, cands: list[Candidate], show_rerank: bool = False) -> None:
    print(f"\n--- {title} ---")
    header = f"{'#':<3} {'chunk id':<28} {'semantic':<10} {'keyword':<10} {'rrf':<8}"
    print(header + (" rerank" if show_rerank else ""))

    for i, c in enumerate(cands, start=1):
        sem = f"#{c.semantic_rank} {c.semantic_score:.2f}" if c.semantic_rank else "-"
        kw = f"#{c.keyword_rank} {c.keyword_score:.1f}" if c.keyword_rank else "-"
        line = f"{i:<3} {truncate(c.id, 27):<28} {sem:<10} {kw:<10} {c.rrf_score:.4f}"
        if show_rerank:
            mark = "✅" if c.rerank_score >= RERANK_MIN_SCORE else "❌"
            line += f"  {c.rerank_score:.2f} {mark}"
        print(line)


def choose_category(categories: list[str]) -> str:
    """Category menu. A category narrows both searches; "others" searches everything."""
    print("\nWhat is your question about?")
    for i, cat in enumerate(categories, start=1):
        print(f"  {i}) {cat}")
    print(f"  {len(categories) + 1}) {OTHER_CATEGORY} (search everything)")

    while True:
        try:
            choice = input("Choose a number [Enter = others]--> ").strip()
        except EOFError:
            return OTHER_CATEGORY

        if not choice or choice.lower() == OTHER_CATEGORY:
            return OTHER_CATEGORY
        if choice.isdigit():
            n = int(choice)
            if 1 <= n <= len(categories):
                return categories[n - 1]
            if n == len(categories) + 1:
                return OTHER_CATEGORY
        for cat in categories:
            if choice.lower() == cat.lower():
                return cat
        print("Invalid choice, try again.")


# ==========================================================
# Chat command
# ==========================================================


def run_chat() -> None:
    chat = RAGChat()

    chat.category = choose_category(chat.categories)
    print(f"Category set to: {chat.category}")
    print("Commands: /category to switch category, /exit to quit.")

    while True:
        try:
            question = input(f"\n[{chat.category}] Ask me anything--> ").strip()
        except EOFError:
            print()
            return

        command = question.lower()
        if not question:
            continue
        if command in ("/exit", "/quit"):
            return
        if command == "/category":
            chat.category = choose_category(chat.categories)
            print(f"Category set to: {chat.category}")
            continue

        try:
            answer = chat.chat(question)
        except Exception as exc:
            print("⚠️ Something went wrong:", exc)
        else:
            print(f"\nAnswer:\n{answer}")