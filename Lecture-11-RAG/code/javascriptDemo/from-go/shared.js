// Shared config, clients and the embedding call for the from-go port
// (the "shared" half of goDemo/main.go).
//
// Keeping embeddings in ONE place guarantees the chunks and the queries are
// always embedded with the same model and dimensions.

import path from "node:path";
import { fileURLToPath } from "node:url";

import dotenv from "dotenv";
import OpenAI from "openai";
import { Pinecone } from "@pinecone-database/pinecone";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// .env is optional; real env vars also work
dotenv.config({ path: path.join(__dirname, "..", ".env") });

// ==========================================================
// Shared config
// ==========================================================

export const embeddingModel = "text-embedding-3-small";
export const embeddingDim = 1536; // native size of text-embedding-3-small; Pinecone index must be 1536-dim
export const maxRetries = 5; // SDK retries 429/5xx with exponential backoff

export function mustEnv(key) {
  const v = process.env[key];
  if (!v) {
    console.error(`missing environment variable ${key}`);
    process.exit(1);
  }
  return v;
}

// ==========================================================
// Shared clients
// ==========================================================

export function newOpenAIClient() {
  return new OpenAI({
    apiKey: mustEnv("OPENAI_API_KEY"),
    maxRetries,
  });
}

// connectIndex looks up the index host by name, checks its dimension matches
// our embeddings, and opens a connection to PINECONE_NAMESPACE ("" = default).
export async function connectIndex() {
  const pc = new Pinecone({ apiKey: mustEnv("PINECONE_API_KEY") });

  const indexName = mustEnv("PINECONE_INDEX_NAME");

  let idxModel;
  try {
    idxModel = await pc.describeIndex(indexName);
  } catch (err) {
    throw new Error(`describe index "${indexName}": ${err.message ?? err}`);
  }

  if (idxModel.dimension != null && idxModel.dimension !== embeddingDim) {
    throw new Error(
      `index "${indexName}" has dimension ${idxModel.dimension} but embeddings are ${embeddingDim}` +
        ` — create a ${embeddingDim}-dim index`,
    );
  }

  return pc.index(indexName, idxModel.host).namespace(process.env.PINECONE_NAMESPACE ?? "");
}

// ==========================================================
// Shared embedding call (used by both index and chat)
// ==========================================================
//
// - One request embeds all texts (input = array of strings).
// - OpenAI returns L2-normalized vectors, so no manual normalize step.

export async function embedTexts(client, texts) {
  let resp;
  try {
    resp = await client.embeddings.create({
      model: embeddingModel,
      input: texts,
      dimensions: embeddingDim,
    });
  } catch (err) {
    if (err instanceof OpenAI.APIError && err.status === 429) {
      throw new Error(
        `still rate limited after ${maxRetries} retries (check quota / lower batch size): ${err.message}`,
      );
    }
    throw err;
  }

  if (resp.data.length !== texts.length) {
    throw new Error(`expected ${texts.length} embeddings, got ${resp.data.length}`);
  }

  // Results carry an index; sort by it so vectors line up with inputs.
  return [...resp.data].sort((a, b) => a.index - b.index).map((d) => d.embedding);
}
