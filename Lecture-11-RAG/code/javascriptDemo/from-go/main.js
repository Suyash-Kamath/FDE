// https://certain-mechanic-42c.notion.site/RAG-System-23c3a78e0e22801caa04d16f95df1825
//
// JavaScript port of goDemo/main.go (+ index.go, query.go):
// a RAG system using OpenAI + Pinecone.
//
//   node from-go/main.js index   -> PDF -> chunks -> OpenAI embeddings (1536-dim) -> Pinecone   (index.js)
//   node from-go/main.js chat    -> question -> rewrite -> embed -> Pinecone top-K -> answer   (query.js)
//
// What both commands share (config, clients, the embedding call) lives in
// shared.js. In Go it all sits in main.go; we split it out to avoid a
// circular import between main.js and index.js/query.js.

import { runIndex } from "./index.js";
import { runChat } from "./query.js";

function usage() {
  console.error(`Usage:
  node from-go/main.js index   Index the PDF into Pinecone
  node from-go/main.js chat    Start the RAG chat loop`);
}

async function main() {
  const command = process.argv[2];

  if (!command) {
    usage();
    process.exit(2);
  }

  try {
    switch (command) {
      case "index":
        await runIndex();
        break;
      case "chat":
        await runChat();
        break;
      default:
        usage();
        process.exit(2);
    }
  } catch (err) {
    console.error(err.message ?? err);
    process.exit(1);
  }
}

await main();
