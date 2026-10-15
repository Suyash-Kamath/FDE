// JavaScript port of pythonDemo/src/pythondemo/main.py
// (FastAPI + AsyncOpenAI -> Express + official `openai` SDK)

import { fileURLToPath } from "node:url";

import dotenv from "dotenv";
import express from "express";
import OpenAI from "openai";

// 1. Load .env first (from this javascriptDemo folder)
dotenv.config({
  path: fileURLToPath(new URL(".env", import.meta.url)),
  quiet: true,
});

// 2. Check whether API key exists
if (!process.env.OPENAI_API_KEY) {
  throw new Error("OPENAI_API_KEY is missing");
}

console.log("OPENAI_API_KEY found");

// 3. Create OpenAI client (reads OPENAI_API_KEY from the environment)
const client = new OpenAI();

const app = express();

async function summarize(ticket) {
  const completion = await client.chat.completions.create({
    model: "gpt-4o-mini",
    messages: [
      {
        role: "user",
        content: "Summarize this support ticket in 2 lines:\n\n" + ticket,
      },
    ],
  });

  if (!completion.choices || completion.choices.length === 0) {
    throw new Error("no choices returned from model");
  }

  const content = completion.choices[0].message.content;

  if (content === null || content === undefined) {
    throw new Error("model returned empty content");
  }

  return content;
}

// Read the raw request body as bytes (whatever the Content-Type is),
// just like `await request.body()` in FastAPI.
app.post(
  "/api/summarize",
  express.raw({ type: () => true }),
  async (req, res) => {
    let ticket;

    try {
      const body = Buffer.isBuffer(req.body) ? req.body : Buffer.alloc(0);
      // fatal: true -> throw on invalid UTF-8 (like Python's UnicodeDecodeError)
      ticket = new TextDecoder("utf-8", { fatal: true }).decode(body);
    } catch {
      return res.status(400).type("text/plain").send("invalid request body");
    }

    try {
      const summary = await summarize(ticket);

      return res.type("text/plain").send(summary);
    } catch (error) {
      console.log("OpenAI error:", error.message);

      return res
        .status(500)
        .type("text/plain")
        .send("failed to summarize ticket");
    }
  },
);

// FastAPI answers other methods on a POST-only route with 405; do the same.
app.all("/api/summarize", (req, res) => {
  res.status(405).json({ detail: "Method Not Allowed" });
});

// The Python app is served by uvicorn, whose default port is 8000.
const PORT = Number(process.env.PORT ?? 8000);

app.listen(PORT, () => {
  console.log(`Server running on http://localhost:${PORT}`);
});
