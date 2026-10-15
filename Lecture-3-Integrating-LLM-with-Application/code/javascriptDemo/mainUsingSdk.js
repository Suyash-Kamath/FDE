// JavaScript port of goDemo/ticket-summarizer/main-using-go-sdk.go
// Uses the official `openai` Node SDK to call the Chat Completions API.

import { fileURLToPath } from "node:url";

import dotenv from "dotenv";
import express from "express";
import OpenAI from "openai";

let client;

async function summarize(ticket) {
  const completion = await client.chat.completions.create({
    messages: [
      {
        role: "user",
        content: "Summarize this support ticket in 2 lines:\n\n" + ticket,
      },
    ],
    model: "gpt-4o-mini",
  });

  if (completion.choices.length === 0) {
    throw new Error("no choices returned from model");
  }

  return completion.choices[0].message.content ?? "";
}

async function summarizeHandler(req, res) {
  if (req.method !== "POST") {
    return res.status(405).type("text/plain").send("method not allowed\n");
  }

  // express.raw() already read the whole body into a Buffer
  const body = Buffer.isBuffer(req.body) ? req.body : Buffer.alloc(0);

  try {
    const summary = await summarize(body.toString("utf-8"));
    res.type("text/plain").send(summary);
  } catch (err) {
    console.log("OpenAI error:", err.message);
    res.status(500).type("text/plain").send("failed to summarize ticket\n");
  }
}

function main() {
  // 1. Load .env FIRST
  const result = dotenv.config({
    path: fileURLToPath(new URL(".env", import.meta.url)),
    quiet: true,
  });
  if (result.error) {
    console.log("Warning: .env file not found");
  }

  // 2. Check whether key exists
  if (!process.env.OPENAI_API_KEY) {
    console.error("OPENAI_API_KEY is missing");
    process.exit(1);
  }

  console.log("OPENAI_API_KEY found");

  // 3. NOW create the client
  client = new OpenAI();

  const app = express();

  // app.all + method check mirrors Go's http.HandleFunc (which accepts every method)
  app.all("/api/summarize", express.raw({ type: () => true }), summarizeHandler);

  console.log("listening on :8080");
  app.listen(8080);
}

main();
