//  https://certain-mechanic-42c.notion.site/RAG-System-23c3a78e0e22801caa04d16f95df1825
//
// JavaScript port of pythonDemo/src/pythondemo/using-langchain/index.py
//
// PDF -> LangChain Documents -> RecursiveCharacterTextSplitter
//     -> Gemini embeddings (768-dim) -> Pinecone (via PineconeStore)

import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import dotenv from "dotenv";

// -------------------------------
// LangChain Document type
// -------------------------------
import { Document } from "@langchain/core/documents";

// -------------------------------
// LangChain Text Splitter
// -------------------------------
import { RecursiveCharacterTextSplitter } from "@langchain/textsplitters";

// -------------------------------
// Gemini Embedding Model
// -------------------------------
import { GoogleGenerativeAIEmbeddings } from "@langchain/google-genai";

// -------------------------------
// Pinecone Client
// -------------------------------
import { Pinecone } from "@pinecone-database/pinecone";

// -------------------------------
// LangChain Pinecone Integration
// -------------------------------
import { PineconeStore } from "@langchain/pinecone";

// -------------------------------
// PDF text extraction (used by our PyPDFLoader stand-in below)
// -------------------------------
import { PDFParse } from "pdf-parse";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// ==========================================================
// Load Environment Variables
// ==========================================================

dotenv.config({ path: path.join(__dirname, "..", ".env") });

// ==========================================================
// Helper : PyPDFLoader equivalent
// ==========================================================

/*
  Python uses langchain_community's PyPDFLoader. The JS equivalent lives in
  @langchain/community, but that package drags in a huge list of optional
  peer dependencies that conflict with the rest of this project. So we do the
  same thing by hand: ONE LangChain Document per PDF page, with the same
  metadata PyPDFLoader sets (source path + 0-indexed page number).
*/

export async function loadPdfAsDocuments(pdfPath) {
  const parser = new PDFParse({ data: await readFile(pdfPath) });

  try {
    const result = await parser.getText();

    return result.pages.map(
      (page) =>
        new Document({
          pageContent: page.text,
          metadata: {
            source: pdfPath,
            page: page.num - 1, // PyPDFLoader uses 0-indexed pages
            total_pages: result.total,
          },
        }),
    );
  } finally {
    await parser.destroy();
  }
}

// ==========================================================
// Helper : Retry a batch upload on Gemini 429 rate limit errors
// ==========================================================

/*
  The Gemini free tier caps embedding calls at a small number
  of requests per minute. A burst of batches can trip that limit
  even though our own BATCH_SIZE is small.

  Google's error response tells us exactly how long to wait
  (e.g. "Please retry in 5.48s"), so we parse that and wait a
  little longer than asked, then retry the SAME batch. Since the
  embedding call fails before anything is written to Pinecone,
  retrying is safe and won't create duplicate vectors.
*/

async function uploadBatchWithBackoff(vectorStore, batch, maxRetries = 5) {
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      await vectorStore.addDocuments(batch);
      return;
    } catch (e) {
      const message = String(e?.message ?? e);

      const isRateLimit = message.includes("RESOURCE_EXHAUSTED") || message.includes("429");

      if (!isRateLimit) {
        throw e;
      }

      const match = message.match(/retry in ([\d.]+)s/);

      const waitSeconds = match ? parseFloat(match[1]) + 2 : attempt * 10;

      console.log(
        `⏳ Rate limited by Gemini API. Waiting ${waitSeconds.toFixed(1)}s ` +
          `(attempt ${attempt}/${maxRetries})...`,
      );

      await new Promise((resolve) => setTimeout(resolve, waitSeconds * 1000));
    }
  }

  throw new Error(
    "Gave up after repeated rate-limit errors. " + "Wait a minute and rerun, or lower BATCH_SIZE.",
  );
}

// ==========================================================
// Embeddings wrapper : force 768-dim output
// ==========================================================

/*
  gemini-embedding-001 outputs 3072 dimensions by default. Our Pinecone
  index was created with dimension 768, so we ask for a truncated
  768-dim output. This is safe because the model is trained with
  Matryoshka Representation Learning (MRL): a shorter prefix of the
  full embedding is still a valid, meaningful embedding on its own.

  We pass outputDimensionality AND truncate manually in the wrapper
  below, so this keeps working even if a given @langchain/google-genai
  version doesn't honor the constructor argument on its own.
*/

export class Gemini768Embeddings extends GoogleGenerativeAIEmbeddings {
  async embedDocuments(texts) {
    const vectors = await super.embedDocuments(texts);
    return vectors.map((v) => v.slice(0, 768));
  }

  async embedQuery(text) {
    const vector = await super.embedQuery(text);
    return vector.slice(0, 768);
  }
}

// ==========================================================
// Main Function
// ==========================================================

async function indexDocument() {
  // ==========================================================
  // Step 1 : Read PDF
  // ==========================================================

  // Python uses "./Dsa.pdf" next to index.py; we read that same file.
  const PDF_PATH = path.resolve(__dirname, "../../pythonDemo/src/pythondemo/using-langchain/Dsa.pdf");

  const rawDocs = await loadPdfAsDocuments(PDF_PATH);

  console.log("✅ PDF Loaded");
  console.log(`Pages Found : ${rawDocs.length}`);

  // ==========================================================
  // Step 2 : Chunk Documents
  // ==========================================================

  /*
    Why Chunking?

    LLMs and Embedding Models cannot efficiently process
    huge documents.

    Therefore we divide every document into smaller pieces.

    Chunk Size = 1000 characters
    Overlap = 200 characters

    Example

    Chunk 1
    1 -----------------------> 1000

    Chunk 2
    801 ---------------------> 1800

    Notice that characters 801 -> 1000 are present in BOTH chunks.
    This prevents context loss.
  */

  const textSplitter = new RecursiveCharacterTextSplitter({
    chunkSize: 1000,
    chunkOverlap: 200,
  });

  const chunkedDocs = await textSplitter.splitDocuments(rawDocs);

  console.log("✅ Chunking Completed");
  console.log(`Chunks Created : ${chunkedDocs.length}`);
  console.log();
  console.log("First Chunk");
  console.log("----------------------------------------");
  console.log(chunkedDocs[0].pageContent.slice(0, 300));
  console.log("----------------------------------------");

  // ==========================================================
  // Step 3 : Configure Embedding Model
  // ==========================================================

  /*
    Embeddings convert text into vectors.

    Here we only configure the model.
    Actual embedding generation happens later when
    addDocuments() is called.
  */

  const embeddings = new Gemini768Embeddings({
    apiKey: process.env.GEMINI_API_KEY,
    model: "models/gemini-embedding-001",
    outputDimensionality: 768,
  });

  console.log("✅ Embedding Model Configured (768-dim)");

  // ==========================================================
  // Step 4 : Connect to Pinecone
  // ==========================================================

  const pinecone = new Pinecone({
    apiKey: process.env.PINECONE_API_KEY,
  });

  const indexName = process.env.PINECONE_INDEX_NAME;

  console.log();
  console.log("Index Name :", indexName);

  const pineconeIndex = pinecone.index(indexName);

  console.log("✅ Pinecone Connected");

  const stats = await pineconeIndex.describeIndexStats();

  console.log();
  console.log("Index Statistics");
  console.log(stats);

  // ==========================================================
  // Step 5 : Create Vector Store
  // ==========================================================

  /*
    PineconeStore is simply a wrapper around Pinecone.

    It knows
    1. Which embedding model to use.
    2. Which Pinecone index to use.

    Whenever we insert documents, LangChain automatically does:

    Text -> Embedding -> Pinecone

    We never manually generate vectors.
  */

  const vectorStore = new PineconeStore(embeddings, {
    pineconeIndex,
  });

  console.log("✅ Vector Store Created");

  // ==========================================================
  // Step 6 : Store Documents
  // ==========================================================

  /*
    Internally LangChain performs

    Chunk -> Gemini Embedding API -> Numerical Vector -> Pinecone

    along with metadata. Every chunk gets stored as one vector.

    We upload in batches instead of all at once so a failure
    partway through doesn't lose all the work, and so we don't
    send oversized requests to the embedding / Pinecone APIs.

    Every batch is wrapped in retry-with-backoff to survive
    the Gemini free tier's requests-per-minute limit.
  */

  const BATCH_SIZE = 20;

  for (let i = 0; i < chunkedDocs.length; i += BATCH_SIZE) {
    const batch = chunkedDocs.slice(i, i + BATCH_SIZE);
    await uploadBatchWithBackoff(vectorStore, batch);
    console.log(`Uploaded ${i + batch.length} documents`);
  }

  console.log();
  console.log("===================================");
  console.log("🎉 Successfully Indexed Documents");
  console.log(`Stored ${chunkedDocs.length} Chunks`);
  console.log("===================================");
}

// ==========================================================
// Entry Point
// ==========================================================

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  await indexDocument();
}
