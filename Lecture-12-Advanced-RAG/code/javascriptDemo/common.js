/**
 * Shared config, clients and the embedding call used by both commands.
 *
 * Keeping embeddings in ONE place guarantees the chunks (index) and the
 * questions (chat) are always embedded with the same model and dimensions.
 */

import OpenAI, { RateLimitError } from "openai";
import { Pinecone } from "@pinecone-database/pinecone";

// ==========================================================
// Shared config
// ==========================================================

export const EMBEDDING_MODEL = "text-embedding-3-small";
export const EMBEDDING_DIM = 1536; // native size of text-embedding-3-small; Pinecone index must be 1536-dim
export const MAX_RETRIES = 5; // SDK retries 429/5xx with exponential backoff

export function mustEnv(key) {
  const value = process.env[key];
  if (!value) {
    // same as Python's sys.exit("..."): message on stderr, exit code 1
    console.error(`missing environment variable ${key}`);
    process.exit(1);
  }
  return value;
}

/** PINECONE_NAMESPACE, or "" for the default namespace. */
export function namespace() {
  return process.env.PINECONE_NAMESPACE ?? "";
}

// ==========================================================
// Shared clients
// ==========================================================

export function newOpenAIClient() {
  return new OpenAI({ apiKey: mustEnv("OPENAI_API_KEY"), maxRetries: MAX_RETRIES });
}

/** Look up the index by name, check dimension + metric, return a connection. */
export async function connectIndex() {
  const pc = new Pinecone({ apiKey: mustEnv("PINECONE_API_KEY") });
  const indexName = mustEnv("PINECONE_INDEX_NAME");
  const desc = await pc.describeIndex(indexName);

  if (desc.dimension && Number(desc.dimension) !== EMBEDDING_DIM) {
    throw new Error(
      `index '${indexName}' has dimension ${desc.dimension} but embeddings are ` +
        `${EMBEDDING_DIM}; create a ${EMBEDDING_DIM}-dim index`,
    );
  }

  // The similarity threshold assumes higher score = more similar.
  const metric = String(desc.metric ?? "").toLowerCase();
  if (metric && !metric.includes("cosine") && !metric.includes("dotproduct")) {
    console.log(
      `⚠️ index metric is '${metric}'; scores are distances, so the ` +
        "similarity threshold in query.js will behave backwards. Use a cosine index.",
    );
  }

  return pc.index({ host: desc.host });
}

// ==========================================================
// Shared embedding call (used by both index and chat)
// ==========================================================
//
// - One request embeds all texts.
// - OpenAI returns L2-normalised vectors, so no manual normalise step.

export async function embedTexts(client, texts) {
  let resp;
  try {
    resp = await client.embeddings.create({
      model: EMBEDDING_MODEL,
      input: texts,
      dimensions: EMBEDDING_DIM,
    });
  } catch (err) {
    if (err instanceof RateLimitError) {
      throw new Error(
        `still rate limited after ${MAX_RETRIES} retries ` + "(check quota / lower the batch size)",
        { cause: err },
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
