// Indexing: PDF -> pages -> recursive chunks -> OpenAI embeddings -> Pinecone
// (port of goDemo/index.go)
//
// Vectors are stored in the same shape LangChain's PineconeVectorStore uses
// (chunk text under the "text" metadata key, plus source/page), so a
// LangChain retriever on the same index keeps working.

import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { Document } from "@langchain/core/documents";
import { RecursiveCharacterTextSplitter } from "@langchain/textsplitters";
import { PDFParse } from "pdf-parse";

import { connectIndex, embedTexts, embeddingDim, embeddingModel, newOpenAIClient } from "./shared.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Go reads "./Dsa.pdf" from the working directory. The PDF lives next to
// the Python LangChain demo, so we point there instead of copying 1 MB.
export const pdfPath = path.resolve(__dirname, "../../pythonDemo/src/pythondemo/using-langchain/Dsa.pdf");
const chunkSize = 1000;
const chunkOverlap = 200;
const batchSize = 100; // 100 chunks ≈ 25k tokens, well under OpenAI's per-request limit

// ==========================================================
// Step 1 : Read PDF (one Document per page, like PyPDFLoader)
// ==========================================================

export async function loadPDF(filePath) {
  let parser;
  try {
    parser = new PDFParse({ data: await readFile(filePath) });
  } catch (err) {
    throw new Error(`open pdf: ${err.message ?? err}`);
  }

  try {
    const result = await parser.getText();
    return result.pages.map(
      (page) =>
        new Document({
          pageContent: page.text,
          metadata: {
            source: filePath,
            page: page.num - 1, // PyPDFLoader uses 0-indexed pages
          },
        }),
    );
  } finally {
    await parser.destroy();
  }
}

// ==========================================================
// Step 2 : Recursive Character Text Splitter
// ==========================================================
//
// The Go version hand-ports LangChain's RecursiveCharacterTextSplitter:
//   - try separators in order ("\n\n", "\n", " ", "")
//   - split on the first one present
//   - pieces still larger than chunkSize are split recursively
//   - small pieces are merged back into chunks of <= chunkSize with
//     chunkOverlap characters of overlap
// In JavaScript we can just use the real thing from @langchain/textsplitters.

export function newRecursiveSplitter(size, overlap) {
  return new RecursiveCharacterTextSplitter({
    chunkSize: size,
    chunkOverlap: overlap,
    separators: ["\n\n", "\n", " ", ""],
  });
}

// ==========================================================
// Step 3 : Upsert one batch into Pinecone
// ==========================================================

async function upsertBatch(idx, batch, vecs) {
  const vectors = batch.map((doc, i) => {
    // LangChain JS's splitter adds a nested "loc" object; Pinecone metadata
    // must be flat, so we keep only source/page (what the Go version stores).
    const { loc, ...metadata } = doc.metadata;
    return {
      id: randomUUID(),
      values: vecs[i],
      metadata: { text: doc.pageContent, ...metadata }, // "text" = LangChain's default textKey
    };
  });
  await idx.upsert(vectors);
}

// ==========================================================
// index command
// ==========================================================

export async function runIndex() {
  // Read PDF
  const rawDocs = await loadPDF(pdfPath);
  console.log("✅ PDF Loaded");
  console.log(`Pages Found : ${rawDocs.length}`);

  // Chunk
  const chunkedDocs = await newRecursiveSplitter(chunkSize, chunkOverlap).splitDocuments(rawDocs);
  if (chunkedDocs.length === 0) {
    throw new Error("no text extracted from PDF (is it scanned/image-only?)");
  }
  console.log("✅ Chunking Completed");
  console.log(`Chunks Created : ${chunkedDocs.length}\n`);
  console.log("First Chunk");
  console.log("----------------------------------------");
  console.log([...chunkedDocs[0].pageContent].slice(0, 300).join(""));
  console.log("----------------------------------------");

  // Clients
  const aiClient = newOpenAIClient();
  console.log(`✅ Embedding Model Configured (${embeddingModel}, ${embeddingDim}-dim)`);

  const idxConn = await connectIndex();
  console.log("✅ Pinecone Connected");

  let stats;
  try {
    stats = await idxConn.describeIndexStats();
  } catch (err) {
    throw new Error(`index stats: ${err.message ?? err}`);
  }
  console.log();
  console.log("Index Statistics");
  console.log(stats);

  // Embed + store in batches
  for (let i = 0; i < chunkedDocs.length; i += batchSize) {
    const end = Math.min(i + batchSize, chunkedDocs.length);
    const batch = chunkedDocs.slice(i, end);

    const texts = batch.map((d) => d.pageContent);

    let vecs;
    try {
      vecs = await embedTexts(aiClient, texts);
    } catch (err) {
      throw new Error(`embed batch starting at ${i}: ${err.message ?? err}`);
    }
    try {
      await upsertBatch(idxConn, batch, vecs);
    } catch (err) {
      throw new Error(`upsert batch starting at ${i}: ${err.message ?? err}`);
    }
    console.log(`Uploaded ${end} documents`);
  }

  console.log();
  console.log("===================================");
  console.log("🎉 Successfully Indexed Documents");
  console.log(`Stored ${chunkedDocs.length} Chunks`);
  console.log("===================================");
}
