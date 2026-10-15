// Small web front-end for code-from-guruji (no Python/Go equivalent).
//
// goDemo/code-from-guruji/static/index.html is a chat UI that calls
//   GET /api/v1/query?question=...   ->   { "answer": "..." }
// but neither main.py nor main.go serves it. This express server does,
// reusing answerUserQuery() from main.js.
//
//   node code-from-guruji/main.js        # run once to load the PDFs into Pinecone
//   node code-from-guruji/server.js      # then open http://localhost:8080

import path from "node:path";
import { fileURLToPath } from "node:url";

import express from "express";

import { answerUserQuery } from "./main.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const PORT = Number(process.env.PORT) || 8080;

const app = express();

// Serves static/index.html at "/"
app.use(express.static(path.join(__dirname, "static")));

app.get("/api/v1/query", async (req, res) => {
  const question = String(req.query.question || "").trim();

  if (!question) {
    return res.status(400).json({ error: "question is required" });
  }

  try {
    const answer = await answerUserQuery(question);
    res.json({ question, answer });
  } catch (error) {
    console.error(error);
    res.status(500).json({ error: String(error.message ?? error) });
  }
});

app.listen(PORT, () => {
  console.log(`ShopSphere support running on http://localhost:${PORT}`);
});
