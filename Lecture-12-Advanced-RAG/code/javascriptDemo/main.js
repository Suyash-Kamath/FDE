/**
 * JavaScript RAG system using OpenAI + Pinecone (port of the Python / Go versions).
 *
 *     node main.js index   knowledge/*.pdf -> chunks -> embeddings -> Pinecone + chunks.json
 *     node main.js chat    category -> rewrite -> semantic + BM25 -> RRF -> rerank -> answer
 *
 * Files:
 *     common.js  shared config, clients, embedding call
 *     index.js   PDF loading, chunking, upload, chunk store
 *     hybrid.js  Candidate, BM25 keyword search, Reciprocal Rank Fusion
 *     query.js   the chat pipeline
 */

import path from "node:path";
import { fileURLToPath } from "node:url";
import dotenv from "dotenv";

const USAGE = `Usage:
  node main.js index   Index the PDFs in ./knowledge into Pinecone
  node main.js chat    Start the RAG chat loop`;

async function main() {
  // Work relative to this folder, so ./knowledge, ./chunks.json and .env
  // are found no matter where `node main.js` is started from.
  process.chdir(path.dirname(fileURLToPath(import.meta.url)));
  dotenv.config({ quiet: true }); // optional; real env vars also work

  const command = process.argv[2];
  if (command !== "index" && command !== "chat") {
    console.error(USAGE);
    return 2;
  }

  // Ctrl+C, like Python's KeyboardInterrupt handler
  process.on("SIGINT", () => {
    console.log("\nBye!");
    process.exit(0);
  });

  try {
    if (command === "index") {
      const { runIndex } = await import("./index.js");
      await runIndex();
    } else {
      const { runChat } = await import("./query.js");
      await runChat();
    }
  } catch (err) {
    // same behaviour as Go's log.Fatal(err)
    console.error(`❌ ${err.message ?? err}`);
    return 1;
  }
  return 0;
}

process.exitCode = await main();
