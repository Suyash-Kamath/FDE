/**
 * Indexing: knowledge/*.pdf -> pages -> recursive chunks -> category metadata
 *           -> OpenAI embeddings -> Pinecone  (+ local chunk store for BM25)
 *
 * Every chunk is stored twice, under the SAME ID:
 *   1. Pinecone   : dense vector + metadata (text, source, page, category)
 *                   -> semantic search, filterable by category
 *   2. chunks.json: chunk text + the same metadata
 *                   -> BM25 keyword search at query time
 *
 * Category comes from the PDF file name: knowledge/graphs.pdf -> "graphs".
 *
 * IDs, metadata and chunks.json use exactly the same format as the Python and
 * Go versions, so an index built by any one of them can be queried by the others.
 */

import fs from "node:fs";
import path from "node:path";
import { PDFParse } from "pdf-parse";

import {
  EMBEDDING_DIM,
  EMBEDDING_MODEL,
  connectIndex,
  embedTexts,
  namespace,
  newOpenAIClient,
} from "./common.js";

export const KNOWLEDGE_DIR = "./knowledge"; // every *.pdf in here; file name (without .pdf) becomes the category
export const KNOWLEDGE_GLOB = `${KNOWLEDGE_DIR}/*.pdf`;
export const CHUNK_STORE_PATH = "./chunks.json"; // local copy of every chunk, used for BM25
const CHUNK_SIZE = 1000;
const CHUNK_OVERLAP = 200;
const EMBED_BATCH_SIZE = 100; // 100 chunks ≈ 25k tokens, well under OpenAI's per-request limit
const UPSERT_BATCH_SIZE = 50; // 1536 floats as JSON ≈ 20 KB/vector; keeps requests under Pinecone's 2 MB cap

/**
 * A Document mirrors LangChain's Document: text plus metadata, plus a stable ID.
 *   { pageContent: string, metadata: { source, page, category }, id: string }
 *
 * A ChunkRecord is one chunk as persisted in chunks.json:
 *   { id, text, source, page, category }
 */

// Lengths are counted in characters (code points), like Python's len(),
// not in UTF-16 units like JavaScript's String.length.
const charLen = (s) => Array.from(s).length;

// ==========================================================
// Step 1 : Read PDFs (one Document per page, like PyPDFLoader)
// ==========================================================

/** './knowledge/Dynamic Programming.pdf' -> 'dynamic_programming'. */
export function categoryFromPath(p) {
  const base = path.basename(p, path.extname(p)).toLowerCase();
  let out = "";
  let lastUnderscore = false;
  for (const ch of base) {
    if (/[\p{L}\p{N}]/u.test(ch)) {
      out += ch;
      lastUnderscore = false;
    } else if (!lastUnderscore) {
      out += "_";
      lastUnderscore = true;
    }
  }
  return out.replace(/^_+|_+$/g, "") || "general";
}

async function loadPdf(p) {
  let parser;
  let result;
  try {
    parser = new PDFParse({ data: fs.readFileSync(p) });
    result = await parser.getText(); // one entry per page in result.pages
  } catch (err) {
    throw new Error(`open pdf ${p}: ${err.message}`, { cause: err });
  } finally {
    await parser?.destroy();
  }

  const category = categoryFromPath(p);
  return result.pages.map((page, i) => ({
    pageContent: page.text || "",
    metadata: {
      source: p,
      page: i, // 0-indexed, like PyPDFLoader
      category,
    },
    id: "",
  }));
}

async function loadKnowledgeBase(dir) {
  // Equivalent of sorted(glob.glob("./knowledge/*.pdf"))
  const paths = fs.existsSync(dir)
    ? fs
        .readdirSync(dir)
        .filter((f) => f.endsWith(".pdf"))
        .map((f) => `${dir}/${f}`)
        .sort()
    : [];
  if (paths.length === 0) {
    throw new Error(
      `no PDFs match '${KNOWLEDGE_GLOB}' (put your PDFs in ./knowledge; ` +
        "the file name becomes the category)",
    );
  }

  const allDocs = [];
  for (const p of paths) {
    const docs = await loadPdf(p);
    console.log(
      `  📄 ${path.basename(p).padEnd(35)} category=${categoryFromPath(p).padEnd(20)} pages=${docs.length}`,
    );
    allDocs.push(...docs);
  }
  return allDocs;
}

// ==========================================================
// Step 2 : Recursive Character Text Splitter
// ==========================================================
//
// Port of LangChain's RecursiveCharacterTextSplitter:
//   - try separators in order ("\n\n", "\n", " ", "")
//   - split on the first one present, keeping the separator at the start
//     of each following piece
//   - pieces still larger than chunkSize are split recursively
//   - small pieces are merged back into chunks of <= chunkSize with
//     chunkOverlap characters of overlap

export class RecursiveSplitter {
  constructor(chunkSize, chunkOverlap, separators = null) {
    this.chunkSize = chunkSize;
    this.chunkOverlap = chunkOverlap;
    this.separators = separators ?? ["\n\n", "\n", " ", ""];
  }

  splitDocuments(docs) {
    const out = [];
    for (const d of docs) {
      for (const chunk of this.#splitText(d.pageContent, this.separators)) {
        out.push({ pageContent: chunk, metadata: { ...d.metadata }, id: "" });
      }
    }
    return out;
  }

  #splitText(text, separators) {
    let separator = separators[separators.length - 1];
    let remaining = [];
    for (let i = 0; i < separators.length; i++) {
      const sep = separators[i];
      if (sep === "") {
        separator = "";
        break;
      }
      if (text.includes(sep)) {
        separator = sep;
        remaining = separators.slice(i + 1);
        break;
      }
    }

    let splits;
    if (separator === "") {
      splits = Array.from(text);
    } else {
      splits = [];
      text.split(separator).forEach((part, i) => {
        if (i > 0) part = separator + part;
        if (part) splits.push(part);
      });
    }

    const final = [];
    let good = [];
    for (const piece of splits) {
      if (charLen(piece) < this.chunkSize) {
        good.push(piece);
        continue;
      }
      if (good.length) {
        final.push(...this.#mergeSplits(good, ""));
        good = [];
      }
      if (remaining.length === 0) {
        final.push(piece);
      } else {
        final.push(...this.#splitText(piece, remaining));
      }
    }
    if (good.length) {
      final.push(...this.#mergeSplits(good, ""));
    }
    return final;
  }

  #mergeSplits(splits, separator) {
    const sepLen = charLen(separator);
    const docs = [];
    const current = [];
    let total = 0;

    const joinCurrent = () => {
      const doc = current.join(separator).trim();
      if (doc) docs.push(doc);
    };
    const extraSep = () => (current.length ? sepLen : 0);

    for (const d of splits) {
      const length = charLen(d);
      if (total + length + extraSep() > this.chunkSize && current.length) {
        joinCurrent();
        while (
          current.length &&
          (total > this.chunkOverlap || (total + length + extraSep() > this.chunkSize && total > 0))
        ) {
          total -= charLen(current[0]) + (current.length > 1 ? sepLen : 0);
          current.shift();
        }
      }
      current.push(d);
      total += length;
      if (current.length > 1) total += sepLen;
    }
    joinCurrent();
    return docs;
  }
}

/**
 * Deterministic IDs like 'graphs-p12-c3'.
 *
 * Re-indexing overwrites vectors instead of duplicating them, and
 * Pinecone + chunks.json always agree on IDs (needed to fuse results).
 */
export function assignChunkIds(docs) {
  const counters = new Map();
  for (const d of docs) {
    const key = `${d.metadata.category}-p${d.metadata.page}`;
    const n = counters.get(key) ?? 0;
    d.id = `${key}-c${n}`;
    counters.set(key, n + 1);
  }
}

// ==========================================================
// Step 3 : Local chunk store (for BM25 keyword search)
// ==========================================================

export function saveChunkStore(p, docs) {
  const records = docs.map((d) => ({
    id: d.id,
    text: d.pageContent,
    source: String(d.metadata.source ?? ""),
    page: Math.trunc(Number(d.metadata.page ?? 0)),
    category: String(d.metadata.category ?? ""),
  }));
  const tmp = p + ".tmp";
  fs.writeFileSync(tmp, JSON.stringify(records, null, 2), "utf-8");
  fs.renameSync(tmp, p); // atomic: a crash never leaves a half-written store
}

export function loadChunkStore(p) {
  let raw;
  try {
    raw = fs.readFileSync(p, "utf-8");
  } catch (err) {
    if (err.code === "ENOENT") {
      throw new Error(`${p} not found, run the index command first`);
    }
    throw new Error(`read chunk store: ${err.message}`, { cause: err });
  }
  let records;
  try {
    records = JSON.parse(raw);
  } catch (err) {
    throw new Error(`decode chunk store: ${err.message}`, { cause: err });
  }
  return records.map((r) => ({
    id: r.id,
    text: r.text,
    source: r.source,
    page: r.page,
    category: r.category,
  }));
}

/** Sorted, unique categories present in the store. */
export function categoriesOf(records) {
  return [...new Set(records.map((r) => r.category).filter(Boolean))].sort();
}

// ==========================================================
// Step 4 : Upsert one batch into Pinecone
// ==========================================================

async function upsertBatch(index, batch, vecs) {
  const vectors = batch.map((doc, i) => ({
    id: doc.id,
    values: vecs[i],
    // "text" is LangChain's default text_key
    metadata: { text: doc.pageContent, ...doc.metadata },
  }));
  for (let i = 0; i < vectors.length; i += UPSERT_BATCH_SIZE) {
    await index.upsert({ records: vectors.slice(i, i + UPSERT_BATCH_SIZE), namespace: namespace() });
  }
}

// ==========================================================
// index command
// ==========================================================

export async function runIndex() {
  // Read PDFs
  console.log("Loading knowledge base...");
  const rawDocs = await loadKnowledgeBase(KNOWLEDGE_DIR);
  console.log("✅ PDFs Loaded");
  console.log(`Pages Found : ${rawDocs.length}`);

  // Chunk
  const chunked = new RecursiveSplitter(CHUNK_SIZE, CHUNK_OVERLAP).splitDocuments(rawDocs);
  if (chunked.length === 0) {
    throw new Error("no text extracted from PDFs (are they scanned/image-only?)");
  }
  assignChunkIds(chunked);

  console.log("✅ Chunking Completed");
  console.log(`Chunks Created : ${chunked.length}`);
  const perCategory = new Map();
  for (const d of chunked) {
    perCategory.set(d.metadata.category, (perCategory.get(d.metadata.category) ?? 0) + 1);
  }
  for (const cat of [...perCategory.keys()].sort()) {
    console.log(`  • ${cat.padEnd(20)} ${perCategory.get(cat)} chunks`);
  }

  console.log();
  console.log(`First Chunk (id=${chunked[0].id})`);
  console.log("----------------------------------------");
  console.log(Array.from(chunked[0].pageContent).slice(0, 300).join(""));
  console.log("----------------------------------------");

  // Clients
  const ai = newOpenAIClient();
  console.log(`✅ Embedding Model Configured (${EMBEDDING_MODEL}, ${EMBEDDING_DIM}-dim)`);

  const index = await connectIndex();
  console.log("✅ Pinecone Connected");
  console.log();
  console.log("Index Statistics");
  console.log(await index.describeIndexStats());

  // Embed + store in batches
  for (let i = 0; i < chunked.length; i += EMBED_BATCH_SIZE) {
    const batch = chunked.slice(i, i + EMBED_BATCH_SIZE);
    let vecs;
    try {
      vecs = await embedTexts(ai, batch.map((d) => d.pageContent));
    } catch (err) {
      throw new Error(`embed batch starting at ${i}: ${err.message}`, { cause: err });
    }
    try {
      await upsertBatch(index, batch, vecs);
    } catch (err) {
      throw new Error(`upsert batch starting at ${i}: ${err.message}`, { cause: err });
    }
    console.log(`Uploaded ${i + batch.length} documents`);
  }

  // Local copy for keyword (BM25) search
  saveChunkStore(CHUNK_STORE_PATH, chunked);
  console.log(`✅ Chunk store written to ${CHUNK_STORE_PATH}`);

  console.log();
  console.log("===================================");
  console.log("🎉 Successfully Indexed Documents");
  console.log(`Stored ${chunked.length} Chunks across ${perCategory.size} categories`);
  console.log("===================================");
}
