// JavaScript port of pythonDemo/src/pythondemo/code-from-guruji/main.py
// (and goDemo/code-from-guruji/main.go).
//
//   node code-from-guruji/main.js "Can I return a product after 20 days?"
//
// 1. Reads every PDF in the knowledge folder, splits it into 300-token chunks
// 2. Embeds the chunks (1024-dim) and upserts them into Pinecone
// 3. Answers ONE question (from the command line) using the top-4 chunks

import { createHash } from "node:crypto";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import dotenv from "dotenv";
import { getEncoding } from "js-tiktoken";
import OpenAI from "openai";
import { Pinecone } from "@pinecone-database/pinecone";
import { PDFParse } from "pdf-parse";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

dotenv.config({ path: path.join(__dirname, "..", ".env") });

// The Python/Go originals point at "spring-code/src/main/resources/knowledge",
// which is not part of this repo. The same ShopSphere PDFs ship with the Go
// version in goDemo/code-from-guruji/knowledge, so we read them from there.
export const KNOWLEDGE_DIR = path.resolve(
  __dirname,
  "../../goDemo/code-from-guruji/knowledge",
);
const INDEX_NAME = process.env.PINECONE_INDEX_NAME || "shop-support";
const NAMESPACE = process.env.PINECONE_NAMESPACE || "policies";
const EMBEDDING_MODEL = "text-embedding-3-small";
const EMBEDDING_DIMENSIONS = 1024;
const CHAT_MODEL = "gpt-5-mini";
const CHUNK_SIZE = 300;
const TOP_K = 4;

function requireKey(name, fallbackName) {
  const value = process.env[name] || process.env[fallbackName];
  if (!value) {
    throw new Error(`Set ${name} (or ${fallbackName}) before running the program.`);
  }
  return value;
}

const openaiClient = new OpenAI({ apiKey: requireKey("OPENAI_API_KEY", "OWN_KEY") });
const pineconeClient = new Pinecone({ apiKey: requireKey("PINECONE_API_KEY", "OWN_KEY") });
const pineconeIndex = pineconeClient.index(INDEX_NAME);
const tokenizer = getEncoding("cl100k_base");

export function splitIntoTokenChunks(text, chunkSize = CHUNK_SIZE) {
  const tokens = tokenizer.encode(text);
  const chunks = [];

  for (let start = 0; start < tokens.length; start += chunkSize) {
    const chunk = tokenizer.decode(tokens.slice(start, start + chunkSize)).trim();
    if (chunk) {
      chunks.push(chunk);
    }
  }

  return chunks;
}

async function readPdfText(pdfPath) {
  const parser = new PDFParse({ data: await readFile(pdfPath) });
  try {
    const result = await parser.getText();
    return result.pages.map((page) => page.text || "").join("\n");
  } finally {
    await parser.destroy();
  }
}

export async function readKnowledgeBase() {
  const chunks = [];

  const pdfNames = (await readdir(KNOWLEDGE_DIR))
    .filter((name) => name.toLowerCase().endsWith(".pdf"))
    .sort();

  for (const pdfName of pdfNames) {
    const text = await readPdfText(path.join(KNOWLEDGE_DIR, pdfName));

    splitIntoTokenChunks(text).forEach((chunkText, chunkNumber) => {
      chunks.push({
        text: chunkText,
        metadata: {
          source: pdfName,
          chunk: chunkNumber,
        },
      });
    });
  }

  return chunks;
}

function stableId(chunk) {
  const rawId = `${chunk.metadata.source}:${chunk.metadata.chunk}:${chunk.text}`;
  return createHash("sha256").update(rawId, "utf8").digest("hex");
}

export async function loadKnowledgeBase() {
  const chunks = await readKnowledgeBase();
  const batchSize = 100;

  for (let start = 0; start < chunks.length; start += batchSize) {
    const batch = chunks.slice(start, start + batchSize);
    const embeddingResponse = await openaiClient.embeddings.create({
      model: EMBEDDING_MODEL,
      input: batch.map((chunk) => chunk.text),
      dimensions: EMBEDDING_DIMENSIONS,
    });

    const records = batch.map((chunk, i) => ({
      id: stableId(chunk),
      values: embeddingResponse.data[i].embedding,
      metadata: { ...chunk.metadata, text: chunk.text },
    }));

    await pineconeIndex.namespace(NAMESPACE).upsert(records);
  }

  return chunks.length;
}

export async function answerUserQuery(question) {
  const queryEmbedding = await openaiClient.embeddings.create({
    model: EMBEDDING_MODEL,
    input: question,
    dimensions: EMBEDDING_DIMENSIONS,
  });

  const searchResult = await pineconeIndex.namespace(NAMESPACE).query({
    vector: queryEmbedding.data[0].embedding,
    topK: TOP_K,
    includeMetadata: true,
  });

  const context = searchResult.matches
    .filter((match) => match.metadata && match.metadata.text)
    .map((match) => match.metadata.text)
    .join("\n\n");

  const instructions = `You are an AI customer support assistant for our e-commerce company.

Answer the customer using ONLY the company information provided below.

If the answer is not available in the provided information, say:
"I don't have that information in the company documents."

COMPANY INFORMATION
${context}`;

  const response = await openaiClient.responses.create({
    model: CHAT_MODEL,
    instructions,
    input: question,
  });
  return response.output_text;
}

async function main() {
  const question = process.argv.slice(2).join(" ") || "What is the return policy?";
  const chunkCount = await loadKnowledgeBase();
  console.log(`Loaded ${chunkCount} knowledge chunks.`);

  // Simple function call; no REST API or web framework is used.
  // (See server.js for a small web UI on top of answerUserQuery.)
  const answer = await answerUserQuery(question);
  console.log(answer);
}

// Only run main() when executed directly (server.js imports this file).
if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
