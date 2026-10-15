// Task client (JS port of pythonDemo/taskClient/app.py)
//
// An Express app on http://127.0.0.1:8080 with one route:
//   GET /ask?message=...  -> runs an OpenAI Agent that can use the MCP task tools
//
// The agent talks to the Task MCP server (taskServer/server.js, or the Python
// / Go server - they are interchangeable) over Streamable HTTP.

import path from "node:path";
import { fileURLToPath } from "node:url";

import dotenv from "dotenv";
import express from "express";
import { Agent, MCPServerStreamableHttp, run } from "@openai/agents";

// Load .env from the javascriptDemo folder (one level up from this file).
const __dirname = path.dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: path.join(__dirname, "..", ".env"), quiet: true });

if (!process.env.OPENAI_API_KEY) {
  throw new Error("Set OPENAI_API_KEY in the .env file");
}

const mcpServer = new MCPServerStreamableHttp({
  name: "Task MCP server",
  url: process.env.MCP_SERVER_URL || "http://localhost:8081/mcp",
  cacheToolsList: true,
});

const agent = new Agent({
  name: "Task Assistant",
  instructions: "Use the MCP task tools to handle every task-management request.",
  mcpServers: [mcpServer],
});

const app = express();

app.get("/ask", async (req, res) => {
  const message = req.query.message;

  // Same validation as FastAPI's Query(min_length=1).
  if (typeof message !== "string" || message.length < 1) {
    return res.status(422).json({ detail: "message must be a non-empty string" });
  }

  try {
    const result = await run(agent, message);
    res.type("text/plain").send(String(result.finalOutput ?? ""));
  } catch (error) {
    console.error(error);
    res.status(500).json({ detail: "Unable to process the request" });
  }
});

// "lifespan": connect to the MCP server before serving, close it on shutdown.
await mcpServer.connect();

const httpServer = app.listen(8080, "127.0.0.1", () => {
  console.log("Task client running on http://127.0.0.1:8080");
});

const shutdown = async () => {
  httpServer.close();
  await mcpServer.close();
  process.exit(0);
};
process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);
