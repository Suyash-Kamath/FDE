// JavaScript port of pythonDemo/src/pythondemo/using-langchain/query.py
//
// question -> rewrite into a standalone query (Gemini)
//          -> embed (Gemini, 768-dim) -> Pinecone top-10
//          -> answer from retrieved context only (Gemini)

import path from "node:path";
import readline from "node:readline/promises";
import { fileURLToPath } from "node:url";

import dotenv from "dotenv";

import { GoogleGenerativeAIEmbeddings } from "@langchain/google-genai";
import { Pinecone } from "@pinecone-database/pinecone";
import { GoogleGenAI } from "@google/genai";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

dotenv.config({ path: path.join(__dirname, "..", ".env") });

// Python's genai.Client() picks up GEMINI_API_KEY from the environment;
// we pass it explicitly so the behaviour is the same.
const ai = new GoogleGenAI({ apiKey: process.env.GEMINI_API_KEY });
const History = [];

async function transformQuery(question) {
  History.push({
    role: "user",
    parts: [{ text: question }],
  });

  const response = await ai.models.generateContent({
    model: "gemini-3.5-flash",
    contents: History,
    config: {
      systemInstruction: `You are a query rewriting expert. Based on the provided chat history, rephrase the "Follow Up user Question" into a complete, standalone question that can be understood without the chat history.
    Only output the rewritten question and nothing else.
      `,
    },
  });

  History.pop();

  return response.text;
}

// ==========================================================
// Embeddings wrapper : force 768-dim output
// ==========================================================

/*
  Same fix as index.js: gemini-embedding-001 returns 3072 dims by
  default, but the Pinecone index is 768-dim. The query vector has to
  be truncated the exact same way the stored vectors were, or the
  Pinecone query call itself rejects it with a dimension error.
*/

class Gemini768Embeddings extends GoogleGenerativeAIEmbeddings {
  async embedDocuments(texts) {
    const vectors = await super.embedDocuments(texts);
    return vectors.map((v) => v.slice(0, 768));
  }

  async embedQuery(text) {
    const vector = await super.embedQuery(text);
    return vector.slice(0, 768);
  }
}

// Created once, outside the loop - no need to spin up a new embeddings
// client and a new Pinecone connection on every single question.
const embeddings = new Gemini768Embeddings({
  apiKey: process.env.GEMINI_API_KEY,
  model: "gemini-embedding-001", // must match whatever index.js used to embed the stored chunks
  outputDimensionality: 768,
});

const pinecone = new Pinecone({ apiKey: process.env.PINECONE_API_KEY });
const pineconeIndex = pinecone.index(process.env.PINECONE_INDEX_NAME);

async function chatting(question) {
  const queries = await transformQuery(question);

  // convert this question into vector
  const queryVector = await embeddings.embedQuery(queries);

  // search pinecone
  const searchResults = await pineconeIndex.query({
    topK: 10,
    vector: queryVector,
    includeMetadata: true,
  });

  console.log("\n--- Raw Pinecone matches (JSON) ---\n");
  console.log(JSON.stringify(searchResults, null, 2));

  // `?.text ?? ""` - a match with no `text` key just contributes "".
  const context = searchResults.matches
    .map((match) => match.metadata?.text ?? "")
    .join("\n\n---\n\n");

  console.log("\n--- Retrieved context (readable) ---\n");
  console.log(context || "(no matches found)");
  console.log("\n------------------------------------\n");

  History.push({
    role: "user",
    parts: [{ text: queries }],
  });

  const response = await ai.models.generateContent({
    model: "gemini-3.5-flash",
    contents: History,
    config: {
      systemInstruction: `You have to behave like a Data Structure and Algorithm Expert.
    You will be given a context of relevant information and a user question.
    Your task is to answer the user's question based ONLY on the provided context.
    If the answer is not in the context, you must say "I could not find the answer in the provided document."
    Keep your answers clear, concise, and educational.

      Context: ${context}
      `,
    },
  });

  History.push({
    role: "model",
    parts: [{ text: response.text }],
  });

  console.log("\n");
  console.log(response.text);

  return context;
}

async function main() {
  // A loop instead of calling main() recursively - same behaviour,
  // no stack growth.
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });

  // Ctrl+D / end of input closes the interface -> stop the loop.
  let closed = false;
  rl.on("close", () => {
    closed = true;
  });

  while (!closed) {
    let userProblem;
    try {
      userProblem = await rl.question("Ask me anything--> ");
    } catch {
      break; // readline was closed while waiting for input
    }

    try {
      await chatting(userProblem);
    } catch (error) {
      console.log("⚠️  Something went wrong:", error?.message ?? error);
    }
  }
}

await main();
