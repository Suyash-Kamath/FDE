"""Hybrid retrieval building blocks:
  - Candidate : one chunk moving through the pipeline, carrying every score
  - BM25Index : in-memory keyword search over chunks.json
  - fuse_rrf  : Reciprocal Rank Fusion, merges semantic + keyword rankings

Why both searches? Semantic search matches meaning but can miss exact
terms (a product code like "AX204", or "Kadane" vs "maximum subarray").
Keyword search nails exact terms but knows nothing about meaning.
Their scores live on different scales (cosine vs BM25), so we don't add
scores; RRF only looks at each document's RANK in each list:

    RRF(d) = Σ  1 / (k + rank_i(d))      k = 60

A chunk ranked well by both searches beats one ranked #1 by only one.
"""

from __future__ import annotations

import math
import re
from collections import Counter
from dataclasses import dataclass, replace

from index import ChunkRecord


@dataclass
class Candidate:
    id: str
    text: str
    source: str = ""
    page: int = 0
    category: str = ""

    semantic_rank: int = 0  # 1-based rank in semantic results, 0 = not retrieved
    semantic_score: float = 0.0  # cosine similarity from Pinecone
    keyword_rank: int = 0  # 1-based rank in BM25 results, 0 = not retrieved
    keyword_score: float = 0.0  # raw BM25 score
    rrf_score: float = 0.0  # fused score
    rerank_score: float = -1.0  # 0..1 relevance from the reranker, -1 = not reranked


# ==========================================================
# Tokenizer
# ==========================================================

STOPWORDS = frozenset(
    """a an the and or but if then else of to in on at by for with
    from into over under about as is are was were be been being am do does did
    doing have has had having i me my we our you your he she it its they them
    their this that these those what which who whom whose when where why how
    can could should would will shall may might must not no yes so than too
    very just also there here all any some each such only own same other more
    most up down out off again further once""".split()
)

_WORD = re.compile(r"[^\W_]+")  # runs of letters/digits (underscore is a separator)


def stem(t: str) -> str:
    """Tiny plural stemmer: 'headphones' -> 'headphone', 'supports' -> 'support'.

    Codes like 'ax204' and words ending in ss/us/is ('class', 'radius',
    'analysis') are left alone.
    """
    if len(t) > 3 and t.endswith("s") and not t.endswith(("ss", "us", "is")):
        return t[:-1]
    return t


def tokenize(s: str) -> list[str]:
    out = []
    for word in _WORD.findall(s.lower()):
        if word in STOPWORDS:
            continue
        # drop single letters ("a", "n" from "O(n)") but keep single digits
        if len(word) < 2 and not word.isdigit():
            continue
        out.append(stem(word))
    return out


# ==========================================================
# BM25 keyword search
# ==========================================================

BM25_K1 = 1.5  # term-frequency saturation
BM25_B = 0.75  # document-length normalisation


class BM25Index:
    def __init__(self, docs: list[ChunkRecord]):
        self.docs = docs
        self.tf: list[Counter[str]] = []
        self.doc_len: list[int] = []
        self.df: Counter[str] = Counter()

        total = 0
        for d in docs:
            tokens = tokenize(d.text)
            counts = Counter(tokens)
            self.df.update(counts.keys())
            self.tf.append(counts)
            self.doc_len.append(len(tokens))
            total += len(tokens)
        self.avg_dl = total / len(docs) if docs and total else 1.0

    def search(self, query: str, k: int, category: str = "") -> list[Candidate]:
        """Up to k chunks ranked by BM25. Empty category = search everything."""
        terms = list(dict.fromkeys(tokenize(query)))  # unique, order kept
        if not terms:
            return []

        n = len(self.docs)
        hits: list[tuple[float, int]] = []
        for i, d in enumerate(self.docs):
            if category and d.category != category:
                continue  # metadata filter, same rule as the Pinecone filter
            dl = self.doc_len[i]
            score = 0.0
            for t in terms:
                f = self.tf[i][t]
                if f == 0:
                    continue
                df = self.df[t]
                idf = math.log(1 + (n - df + 0.5) / (df + 0.5))
                score += idf * (f * (BM25_K1 + 1)) / (f + BM25_K1 * (1 - BM25_B + BM25_B * dl / self.avg_dl))
            if score > 0:
                hits.append((score, i))

        hits.sort(key=lambda h: h[0], reverse=True)
        out = []
        for rank, (score, i) in enumerate(hits[:k], start=1):
            d = self.docs[i]
            out.append(
                Candidate(
                    id=d.id,
                    text=d.text,
                    source=d.source,
                    page=d.page,
                    category=d.category,
                    keyword_rank=rank,
                    keyword_score=score,
                )
            )
        return out


# ==========================================================
# Reciprocal Rank Fusion
# ==========================================================


def fuse_rrf(k: int, semantic: list[Candidate], keyword: list[Candidate]) -> list[Candidate]:
    by_id: dict[str, Candidate] = {}  # dicts keep insertion order

    def get(c: Candidate) -> Candidate:
        if c.id in by_id:
            existing = by_id[c.id]
            if not existing.text:
                existing.text = c.text
            return existing
        by_id[c.id] = replace(c)  # copy, never mutate the caller's objects
        return by_id[c.id]

    for c in semantic:
        m = get(c)
        m.semantic_rank, m.semantic_score = c.semantic_rank, c.semantic_score
        m.rrf_score += 1.0 / (k + c.semantic_rank)
    for c in keyword:
        m = get(c)
        m.keyword_rank, m.keyword_score = c.keyword_rank, c.keyword_score
        m.rrf_score += 1.0 / (k + c.keyword_rank)

    # sorted() is stable, like Go's sort.SliceStable
    return sorted(by_id.values(), key=lambda c: c.rrf_score, reverse=True)