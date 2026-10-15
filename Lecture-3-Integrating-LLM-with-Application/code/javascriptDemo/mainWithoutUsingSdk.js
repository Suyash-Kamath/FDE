// JavaScript port of goDemo/ticket-summarizer/main-without-using-sdk.go
// No SDK: we call the OpenAI Chat Completions HTTP API by hand with fetch.

import { fileURLToPath } from "node:url";

import dotenv from "dotenv";
import express from "express";

// --- request/response shapes for the OpenAI chat completions API ---
//
// request:  { model: string, messages: [{ role, content }] }
// response: { choices: [{ message: { role, content } }] }

// summarize mirrors SummarizeService.summarize(ticket)
async function summarize(ticket) {
  const apiKey = process.env.OPENAI_API_KEY;

  if (!apiKey) {
    throw new Error("OPENAI_API_KEY environment variable not set");
  }

  const reqBody = {
    model: "gpt-4o-mini",
    messages: [
      {
        role: "user",
        content: "Summarize this support ticket in 2 lines:\n\n" + ticket,
      },
    ],
  };

  const resp = await fetch("https://api.openai.com/v1/chat/completions", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + apiKey,
    },
    body: JSON.stringify(reqBody),
  });

  const body = await resp.text();

  if (resp.status !== 200) {
    throw new Error(
      `openai request failed with status ${resp.status}: ${body}`,
    );
  }

  const result = JSON.parse(body);

  if (!result.choices || result.choices.length === 0) {
    throw new Error("no choices returned from model");
  }

  return result.choices[0].message.content;
}

// summarizeHandler mirrors SummarizeController.summarize(ticket)
async function summarizeHandler(req, res) {
  if (req.method !== "POST") {
    return res.status(405).type("text/plain").send("method not allowed\n");
  }

  // express.raw() already read the whole body into a Buffer
  const body = Buffer.isBuffer(req.body) ? req.body : Buffer.alloc(0);

  if (body.length === 0) {
    return res.status(400).type("text/plain").send("ticket cannot be empty\n");
  }

  let summary;
  try {
    summary = await summarize(body.toString("utf-8"));
  } catch (err) {
    console.log("summarize error:", err.message);

    return res
      .status(500)
      .type("text/plain")
      .send("failed to summarize ticket\n");
  }

  res.set("Content-Type", "text/plain; charset=utf-8");
  res.status(200).send(summary);
}

function main() {
  // Load variables from .env
  const result = dotenv.config({
    path: fileURLToPath(new URL(".env", import.meta.url)),
    quiet: true,
  });

  if (result.error) {
    console.log(
      "Warning: .env file not found, using system environment variables",
    );
  }

  // Validate API key before starting server
  if (!process.env.OPENAI_API_KEY) {
    console.error("OPENAI_API_KEY is not set");
    process.exit(1);
  }

  console.log("OPENAI_API_KEY loaded successfully");

  const app = express();

  // app.all + method check mirrors Go's http.HandleFunc (which accepts every method)
  app.all("/api/summarize", express.raw({ type: () => true }), summarizeHandler);

  console.log("server listening on http://localhost:8080");

  app.listen(8080);
}

main();
