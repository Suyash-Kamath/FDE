/**
 * Hybrid retrieval building blocks:
 *   - Candidate : one chunk moving through the pipeline, carrying every score
 *   - BM25Index : in-memory keyword search over chunks.json
 *   - fuseRRF   : Reciprocal Rank Fusion, merges semantic + keyword rankings
 *
 * Why both searches? Semantic search matches meaning but can miss exact
 * terms (a product code like "AX204", or "Kadane" vs "maximum subarray").
 * Keyword search nails exact terms but knows nothing about meaning.
 * Their scores live on different scales (cosine vs BM25), so we don't add
 * scores; RRF only looks at each document's RANK in each list:
 *
 *     RRF(d) = Σ  1 / (k + rank_i(d))      k = 60
 *
 * A chunk ranked well by both searches beats one ranked #1 by only one.
 */

/**
 * Build a Candidate (the Python dataclass) with the same defaults.
 */
export function newCandidate({
  id,
  text,
  source = "",
  page = 0,
  category = "",
  semanticRank = 0, // 1-based rank in semantic results, 0 = not retrieved
  semanticScore = 0.0, // cosine similarity from Pinecone
  keywordRank = 0, // 1-based rank in BM25 results, 0 = not retrieved
  keywordScore = 0.0, // raw BM25 score
  rrfScore = 0.0, // fused score
  rerankScore = -1.0, // 0..1 relevance from the reranker, -1 = not reranked
}) {
  return {
    id,
    text,
    source,
    page,
    category,
    semanticRank,
    semanticScore,
    keywordRank,
    keywordScore,
    rrfScore,
    rerankScore,
  };
}

// ==========================================================
// Tokenizer
// ==========================================================

const STOPWORDS = new Set(
  `a an the and or but if then else of to in on at by for with
    from into over under about as is are was were be been being am do does did
    doing have has had having i me my we our you your he she it its they them
    their this that these those what which who whom whose when where why how
    can could should would will shall may might must not no yes so than too
    very just also there here all any some each such only own same other more
    most up down out off again further once`.split(/\s+/),
);

const WORD = /[\p{L}\p{N}]+/gu; // runs of letters/digits (underscore and punctuation are separators)
const DIGIT = /^\p{Nd}$/u;

/**
 * Tiny plural stemmer: 'headphones' -> 'headphone', 'supports' -> 'support'.
 *
 * Codes like 'ax204' and words ending in ss/us/is ('class', 'radius',
 * 'analysis') are left alone.
 */
export function stem(t) {
  if (
    Array.from(t).length > 3 &&
    t.endsWith("s") &&
    !t.endsWith("ss") &&
    !t.endsWith("us") &&
    !t.endsWith("is")
  ) {
    return t.slice(0, -1);
  }
  return t;
}

export function tokenize(s) {
  const out = [];
  for (const word of s.toLowerCase().match(WORD) ?? []) {
    if (STOPWORDS.has(word)) continue;
    // drop single letters ("a", "n" from "O(n)") but keep single digits
    if (Array.from(word).length < 2 && !DIGIT.test(word)) continue;
    out.push(stem(word));
  }
  return out;
}

// ==========================================================
// BM25 keyword search
// ==========================================================

const BM25_K1 = 1.5; // term-frequency saturation
const BM25_B = 0.75; // document-length normalisation

export class BM25Index {
  constructor(docs) {
    this.docs = docs;
    this.tf = []; // term counts per chunk (Map term -> count)
    this.docLen = [];
    this.df = new Map(); // number of chunks containing each term

    let total = 0;
    for (const d of docs) {
      const tokens = tokenize(d.text);
      const counts = new Map();
      for (const t of tokens) counts.set(t, (counts.get(t) ?? 0) + 1);
      for (const t of counts.keys()) this.df.set(t, (this.df.get(t) ?? 0) + 1);
      this.tf.push(counts);
      this.docLen.push(tokens.length);
      total += tokens.length;
    }
    this.avgDL = docs.length && total ? total / docs.length : 1.0;
  }

  /** Up to k chunks ranked by BM25. Empty category = search everything. */
  search(query, k, category = "") {
    const terms = [...new Set(tokenize(query))]; // unique, order kept
    if (terms.length === 0) return [];

    const n = this.docs.length;
    const hits = []; // [score, docIndex]
    this.docs.forEach((d, i) => {
      if (category && d.category !== category) return; // metadata filter, same rule as the Pinecone filter
      const dl = this.docLen[i];
      let score = 0.0;
      for (const t of terms) {
        const f = this.tf[i].get(t) ?? 0;
        if (f === 0) continue;
        const df = this.df.get(t);
        const idf = Math.log(1 + (n - df + 0.5) / (df + 0.5));
        score += (idf * (f * (BM25_K1 + 1))) / (f + BM25_K1 * (1 - BM25_B + (BM25_B * dl) / this.avgDL));
      }
      if (score > 0) hits.push([score, i]);
    });

    hits.sort((a, b) => b[0] - a[0]); // Array.prototype.sort is stable
    return hits.slice(0, k).map(([score, i], r) => {
      const d = this.docs[i];
      return newCandidate({
        id: d.id,
        text: d.text,
        source: d.source,
        page: d.page,
        category: d.category,
        keywordRank: r + 1,
        keywordScore: score,
      });
    });
  }
}

// ==========================================================
// Reciprocal Rank Fusion
// ==========================================================

export function fuseRRF(k, semantic, keyword) {
  const byId = new Map(); // Maps keep insertion order

  const get = (c) => {
    if (byId.has(c.id)) {
      const existing = byId.get(c.id);
      if (!existing.text) existing.text = c.text;
      return existing;
    }
    byId.set(c.id, { ...c }); // copy, never mutate the caller's objects
    return byId.get(c.id);
  };

  for (const c of semantic) {
    const m = get(c);
    m.semanticRank = c.semanticRank;
    m.semanticScore = c.semanticScore;
    m.rrfScore += 1.0 / (k + c.semanticRank);
  }
  for (const c of keyword) {
    const m = get(c);
    m.keywordRank = c.keywordRank;
    m.keywordScore = c.keywordScore;
    m.rrfScore += 1.0 / (k + c.keywordRank);
  }

  // sort() is stable, like Go's sort.SliceStable
  return [...byId.values()].sort((a, b) => b.rrfScore - a.rrfScore);
}
