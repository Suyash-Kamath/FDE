/**
 * Chat pipeline (advanced RAG):
 *
 *   user picks a category (or "others" = no filter)
 *   question -> rewrite into standalone query (only when there is history)
 *            -> embed query
 *            -> [semantic] Pinecone top-N, category filter, similarity threshold
 *            -> [keyword]  BM25 top-N over chunks.json, same category filter
 *            -> nothing from either?  -> canned fallback (LLM never called)
 *            -> Reciprocal Rank Fusion (k = 60)
 *            -> rerank the fused pool (LLM as reranker, query+chunk read together)
 *            -> keep chunks above the rerank cut-off, top FINAL_TOP_K
 *            -> answer from that context only
 */

import readline from "node:readline";

import { connectIndex, embedTexts, namespace, newOpenAIClient } from "./common.js";
import { BM25Index, fuseRRF, newCandidate } from "./hybrid.js";
import { CHUNK_STORE_PATH, categoriesOf, loadChunkStore } from "./index.js";

const CHAT_MODEL = "gpt-5.4-mini";

// Retrieval
const CANDIDATE_POOL_K = 20; // how many chunks EACH search fetches (wide net)
const SIMILARITY_THRESHOLD = 0.75; // min cosine score for a semantic hit; text-embedding-3 scores run low, tune this
const RRF_K = 60; // standard RRF constant

// Reranking
const ENABLE_RERANK = true; // turn off for small/simple corpora to save a model call
const RERANK_MIN_SCORE = 0.5; // chunks the reranker scores below this are dropped
const RERANK_DOC_CHARS = 1500; // chunk text sent to the reranker is truncated to this

// What finally reaches the answering LLM
const FINAL_TOP_K = 4;

const MAX_HISTORY_TURNS = 6; // question/answer pairs kept in chat history
const OTHER_CATEGORY = "others"; // "others" = search across every category

const FALLBACK_ANSWER = "I could not find the relevant information about this query.";

const REWRITE_PROMPT = `You are a query rewriting expert.
Based on the provided chat history, rephrase the follow-up user question
into a complete, standalone question that can be understood without chat history.
Only output the rewritten question and nothing else.`;

const RERANK_PROMPT = `You are a reranking model.
You will get a user query and a numbered list of documents.
For EACH document independently, judge how useful it is for answering the query
and give it a relevance score between 0 and 1:
  1.0 = directly answers the query
  0.5 = partially relevant / useful background
  0.0 = unrelated
Return ONLY a JSON object, no markdown and no other text, in exactly this form:
{"scores":[{"id":1,"score":0.92},{"id":2,"score":0.10}]}
Include every document id exactly once.`;

const answerPrompt = (context) => `You have to behave like a Data Structure and Algorithm Expert.
You will be given a context of relevant information and a user question.
Your task is to answer the user's question based ONLY on the provided context.
If the answer is not in the context, you must say:
"I could not find the relevant information about this query."
Keep your answers clear, concise, and educational.

Context:
${context}`;

// ==========================================================
// RAGChat
// ==========================================================

class RAGChat {
  // Connecting to Pinecone is async, so use `await RAGChat.create()` instead of `new`.
  static async create() {
    const chat = new RAGChat();
    const records = loadChunkStore(CHUNK_STORE_PATH);
    chat.index = await connectIndex();
    chat.ai = newOpenAIClient();
    chat.bm25 = new BM25Index(records);
    chat.categories = categoriesOf(records);
    chat.category = OTHER_CATEGORY;
    chat.history = [];
    return chat;
  }

  /** The category to filter on, or "" for no filter. */
  filterCategory() {
    return this.category === OTHER_CATEGORY ? "" : this.category;
  }

  remember(question, answer) {
    this.history.push({ role: "user", content: question }, { role: "assistant", content: answer });
    this.history = this.history.slice(-MAX_HISTORY_TURNS * 2);
  }

  /**
   * Call the chat model. withHistory=false is used for the reranker,
   * which must judge relevance without being swayed by the conversation.
   */
  async complete(system, user, withHistory) {
    const messages = [{ role: "system", content: system }];
    if (withHistory) messages.push(...this.history);
    messages.push({ role: "user", content: user });

    const resp = await this.ai.chat.completions.create({ model: CHAT_MODEL, messages });
    if (!resp.choices?.length) {
      throw new Error("empty response from model");
    }
    return (resp.choices[0].message.content ?? "").trim();
  }

  async transformQuery(question) {
    if (this.history.length === 0) {
      return question; // nothing to resolve against, skip the LLM call
    }
    let rewritten;
    try {
      rewritten = await this.complete(REWRITE_PROMPT, question, true);
    } catch (err) {
      throw new Error(`rewrite query: ${err.message}`, { cause: err });
    }
    return rewritten || question;
  }

  // ------------------------------------------------------
  // Semantic search (Pinecone + metadata filter + threshold)
  // ------------------------------------------------------

  async semanticSearch(vector) {
    const request = {
      vector,
      topK: CANDIDATE_POOL_K,
      includeMetadata: true,
      namespace: namespace(),
    };
    const cat = this.filterCategory();
    if (cat) {
      request.filter = { category: { $eq: cat } }; // category == "<selected>"
    }

    let results;
    try {
      results = await this.index.query(request);
    } catch (err) {
      throw new Error(`pinecone query: ${err.message}`, { cause: err });
    }

    const out = [];
    let rejected = 0;
    for (const m of results.matches ?? []) {
      const md = m.metadata ?? {};
      const text = md.text || "";
      if (!String(text).trim()) continue;
      if (m.score == null || m.score < SIMILARITY_THRESHOLD) {
        rejected += 1;
        continue;
      }
      out.push(
        newCandidate({
          id: m.id,
          text,
          source: String(md.source ?? ""),
          page: Math.trunc(Number(md.page || 0)), // Pinecone returns numbers as floats
          category: String(md.category ?? ""),
          semanticRank: out.length + 1,
          semanticScore: Number(m.score),
        }),
      );
    }

    console.log(
      `Semantic : ${out.length} accepted, ${rejected} below threshold ${SIMILARITY_THRESHOLD.toFixed(2)}`,
    );
    return out;
  }

  // ------------------------------------------------------
  // Reranking (LLM as a cross-encoder style reranker)
  // ------------------------------------------------------
  //
  // The embedding model embeds the query and each chunk SEPARATELY, then
  // compares vectors. A reranker reads the query and the chunk TOGETHER and
  // scores "can this chunk answer this query?", which is far more precise.
  // It is slower, so it only runs on the small fused pool.

  async rerank(query, cands) {
    const parts = [`Query: ${query}\n`];
    cands.forEach((cand, i) => {
      parts.push(`Document ${i + 1}:\n${truncate(cand.text, RERANK_DOC_CHARS)}\n`);
    });
    const raw = await this.complete(RERANK_PROMPT, parts.join("\n"), false);

    // Be forgiving about stray text / markdown fences around the JSON.
    const start = raw.indexOf("{");
    const end = raw.lastIndexOf("}");
    if (start < 0 || end <= start) {
      throw new Error(`reranker returned no JSON: ${JSON.stringify(raw)}`);
    }
    const parsed = JSON.parse(raw.slice(start, end + 1));

    // anything the model skipped counts as irrelevant
    const out = cands.map((c) => ({ ...c, rerankScore: 0.0 }));
    for (const item of parsed.scores ?? []) {
      const idx = Math.trunc(Number(item.id ?? 0));
      if (idx >= 1 && idx <= out.length) {
        out[idx - 1].rerankScore = Math.min(Math.max(Number(item.score ?? 0), 0.0), 1.0);
      }
    }

    out.sort((a, b) => b.rerankScore - a.rerankScore);
    return out;
  }

  // ------------------------------------------------------
  // Chat
  // ------------------------------------------------------

  async chat(question) {
    // STEP 1: Rewrite question into a standalone query.
    const query = await this.transformQuery(question);
    console.log("\n--- Rewritten Query ---");
    console.log(query);
    console.log(`\n--- Retrieval (category: ${this.category}) ---`);

    // STEP 2: Generate query embedding.
    let vector;
    try {
      [vector] = await embedTexts(this.ai, [query]);
    } catch (err) {
      throw new Error(`embed query: ${err.message}`, { cause: err });
    }

    // STEP 3: Semantic search (metadata filter + similarity threshold).
    const semantic = await this.semanticSearch(vector);

    // STEP 4: Keyword search (BM25, same category filter).
    const keyword = this.bm25.search(query, CANDIDATE_POOL_K, this.filterCategory());
    console.log(`Keyword  : ${keyword.length} BM25 hits`);

    // STEP 5: Nothing relevant anywhere -> canned answer, LLM is never called.
    if (semantic.length === 0 && keyword.length === 0) {
      console.log("\n⚠️ No relevant chunks found (threshold / category / keywords).");
      this.remember(question, FALLBACK_ANSWER);
      return FALLBACK_ANSWER;
    }

    // STEP 6: Merge both rankings with Reciprocal Rank Fusion.
    const fused = fuseRRF(RRF_K, semantic, keyword).slice(0, CANDIDATE_POOL_K);
    printCandidates("Fused with RRF (k=60)", fused);

    // STEP 7: Rerank the fused pool.
    let final = fused;
    if (ENABLE_RERANK) {
      let reranked = null;
      try {
        reranked = await this.rerank(query, fused);
      } catch (err) {
        // Reranking is an improvement, not a requirement: keep RRF order.
        console.log("⚠️ Rerank failed, falling back to RRF order:", err.message);
      }
      if (reranked) {
        printCandidates("Reranked", reranked, true);
        final = reranked.filter((c) => c.rerankScore >= RERANK_MIN_SCORE);
      }
    }

    // STEP 8: Keep the best few for the LLM.
    final = final.slice(0, FINAL_TOP_K);
    if (final.length === 0) {
      console.log(`\n⚠️ Reranker judged every chunk irrelevant (< ${RERANK_MIN_SCORE.toFixed(2)}).`);
      this.remember(question, FALLBACK_ANSWER);
      return FALLBACK_ANSWER;
    }

    const context = final
      .map((c) => `[Source: ${c.source}, page ${c.page + 1}, category: ${c.category}]\n${c.text}`)
      .join("\n\n---\n\n");
    console.log(`\n--- Context sent to LLM (${final.length} chunks) ---`);
    console.log(context);
    console.log("\n------------------------------------");

    // STEP 9: Generate answer from retrieved context.
    let answer;
    try {
      answer = await this.complete(answerPrompt(context), query, true);
    } catch (err) {
      throw new Error(`generate answer: ${err.message}`, { cause: err });
    }

    // STEP 10: Save conversation history.
    this.remember(question, answer);
    return answer;
  }
}

// ==========================================================
// Helpers
// ==========================================================

/** Cut to n characters (code points, like Python slicing) and add "…". */
function truncate(s, n) {
  const chars = Array.from(s);
  return chars.length <= n ? s : chars.slice(0, n).join("") + "…";
}

function printCandidates(title, cands, showRerank = false) {
  console.log(`\n--- ${title} ---`);
  const header = `${"#".padEnd(3)} ${"chunk id".padEnd(28)} ${"semantic".padEnd(10)} ${"keyword".padEnd(10)} ${"rrf".padEnd(8)}`;
  console.log(header + (showRerank ? " rerank" : ""));

  cands.forEach((c, i) => {
    const sem = c.semanticRank ? `#${c.semanticRank} ${c.semanticScore.toFixed(2)}` : "-";
    const kw = c.keywordRank ? `#${c.keywordRank} ${c.keywordScore.toFixed(1)}` : "-";
    let line = `${String(i + 1).padEnd(3)} ${truncate(c.id, 27).padEnd(28)} ${sem.padEnd(10)} ${kw.padEnd(10)} ${c.rrfScore.toFixed(4)}`;
    if (showRerank) {
      const mark = c.rerankScore >= RERANK_MIN_SCORE ? "✅" : "❌";
      line += `  ${c.rerankScore.toFixed(2)} ${mark}`;
    }
    console.log(line);
  });
}

// ----------------------------------------------------------
// Terminal input (Python's input()). Returns null on EOF (Ctrl+D / end of piped input),
// which is where Python raises EOFError.
// ----------------------------------------------------------

const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
const lines = rl[Symbol.asyncIterator]();
rl.on("SIGINT", () => {
  // Ctrl+C, same as Python's KeyboardInterrupt handler in main.py
  console.log("\nBye!");
  process.exit(0);
});

async function input(prompt) {
  rl.setPrompt(prompt);
  rl.prompt();
  const { value, done } = await lines.next();
  return done ? null : value;
}

/** Category menu. A category narrows both searches; "others" searches everything. */
async function chooseCategory(categories) {
  console.log("\nWhat is your question about?");
  categories.forEach((cat, i) => console.log(`  ${i + 1}) ${cat}`));
  console.log(`  ${categories.length + 1}) ${OTHER_CATEGORY} (search everything)`);

  while (true) {
    const line = await input("Choose a number [Enter = others]--> ");
    if (line === null) return OTHER_CATEGORY; // EOF
    const choice = line.trim();

    if (!choice || choice.toLowerCase() === OTHER_CATEGORY) return OTHER_CATEGORY;
    if (/^\d+$/.test(choice)) {
      const n = Number(choice);
      if (n >= 1 && n <= categories.length) return categories[n - 1];
      if (n === categories.length + 1) return OTHER_CATEGORY;
    }
    for (const cat of categories) {
      if (choice.toLowerCase() === cat.toLowerCase()) return cat;
    }
    console.log("Invalid choice, try again.");
  }
}

// ==========================================================
// Chat command
// ==========================================================

export async function runChat() {
  try {
    const chat = await RAGChat.create();

    chat.category = await chooseCategory(chat.categories);
    console.log(`Category set to: ${chat.category}`);
    console.log("Commands: /category to switch category, /exit to quit.");

    while (true) {
      console.log(); // blank line before the prompt (readline prompts should be single-line)
      const line = await input(`[${chat.category}] Ask me anything--> `);
      if (line === null) {
        console.log();
        return;
      }
      const question = line.trim();

      const command = question.toLowerCase();
      if (!question) continue;
      if (command === "/exit" || command === "/quit") return;
      if (command === "/category") {
        chat.category = await chooseCategory(chat.categories);
        console.log(`Category set to: ${chat.category}`);
        continue;
      }

      try {
        const answer = await chat.chat(question);
        console.log(`\nAnswer:\n${answer}`);
      } catch (err) {
        console.log("⚠️ Something went wrong:", err.message);
      }
    }
  } finally {
    rl.close(); // let the process exit
  }
}
