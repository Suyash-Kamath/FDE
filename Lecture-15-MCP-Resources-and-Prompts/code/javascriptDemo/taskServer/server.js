// Task MCP server (JS port of pythonDemo/taskServer/server.py)
//
// Lecture 14 only had tools. This server shows all three MCP building blocks:
//   Tools     -> create_task, list_tasks, complete_task  (the model calls these)
//   Resources -> task://guidelines                       (data the client reads)
//   Prompts   -> plan_day                                (reusable prompt templates)
//
// Served over Streamable HTTP at http://127.0.0.1:8081/mcp. Like the Python
// default, the server is stateful: each client gets an MCP session id
// (Mcp-Session-Id header) and responses are streamed as SSE.

import { randomUUID } from "node:crypto";

import express from "express";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { isInitializeRequest } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";

import { planDayPrompt } from "./taskPrompts.js";
import { getGuidelines } from "./taskResources.js";
import { completeTaskValue, createTaskValue, listTaskValues } from "./taskTools.js";

// Python's MCP SDK returns a str result as one text block and also as
// structuredContent {"result": ...}. We do the same so every client sees
// identical results from the Python, JS and Go servers.
function textResult(text) {
  return {
    content: [{ type: "text", text }],
    structuredContent: { result: text },
  };
}

// One McpServer per session (the JS SDK connects a server to one transport).
// The task list lives in taskTools.js, so all sessions share the same tasks.
function createServer() {
  const mcp = new McpServer({ name: "task-mcp-server", version: "1.0.0" });

  // ---------- Tools ----------

  mcp.registerTool(
    "create_task",
    {
      description: "Create a new task. The title is required.",
      inputSchema: { title: z.string() },
      outputSchema: { result: z.string() },
    },
    async ({ title }) => textResult(createTaskValue(title)),
  );

  mcp.registerTool(
    "list_tasks",
    {
      description: "List all pending tasks.",
      outputSchema: { result: z.array(z.string()) },
    },
    async () => {
      const tasks = listTaskValues();
      return {
        // Python turns a list[str] into one text block per task.
        content: tasks.map((task) => ({ type: "text", text: task })),
        structuredContent: { result: tasks },
      };
    },
  );

  mcp.registerTool(
    "complete_task",
    {
      description: "Complete a task using its exact title.",
      inputSchema: { title: z.string() },
      outputSchema: { result: z.string() },
    },
    async ({ title }) => textResult(completeTaskValue(title)),
  );

  // ---------- Resources ----------

  mcp.registerResource(
    "task-guidelines",
    "task://guidelines",
    {
      description: "Guidelines for managing and prioritizing tasks",
      mimeType: "text/plain",
    },
    async (uri) => ({
      contents: [{ uri: uri.href, mimeType: "text/plain", text: getGuidelines() }],
    }),
  );

  // ---------- Prompts ----------

  mcp.registerPrompt(
    "plan_day",
    {
      description: "Plan pending tasks for the available time",
      argsSchema: { availableHours: z.string() },
    },
    ({ availableHours }) => ({
      description: "Plan pending tasks for the available time",
      messages: [
        {
          role: "user",
          content: { type: "text", text: planDayPrompt(availableHours) },
        },
      ],
    }),
  );

  return mcp;
}

// ---------- Streamable HTTP with sessions ----------

const transports = {}; // session id -> transport

const app = express();
app.use(express.json());

app.post("/mcp", async (req, res) => {
  const sessionId = req.headers["mcp-session-id"];
  let transport = sessionId ? transports[sessionId] : undefined;

  if (!transport) {
    if (sessionId || !isInitializeRequest(req.body)) {
      return res.status(400).json({
        jsonrpc: "2.0",
        error: { code: -32000, message: "Bad Request: No valid session ID provided" },
        id: null,
      });
    }

    // New client: create a transport + server for its session.
    transport = new StreamableHTTPServerTransport({
      sessionIdGenerator: () => randomUUID(),
      onsessioninitialized: (id) => {
        transports[id] = transport;
      },
    });
    transport.onclose = () => {
      if (transport.sessionId) delete transports[transport.sessionId];
    };
    await createServer().connect(transport);
  }

  await transport.handleRequest(req, res, req.body);
});

// GET opens the SSE stream for server -> client messages, DELETE ends a session.
const handleSessionRequest = async (req, res) => {
  const transport = transports[req.headers["mcp-session-id"]];
  if (!transport) {
    return res.status(400).send("Invalid or missing session ID");
  }
  await transport.handleRequest(req, res);
};
app.get("/mcp", handleSessionRequest);
app.delete("/mcp", handleSessionRequest);

app.listen(8081, "127.0.0.1", () => {
  console.log("Task MCP server running on http://127.0.0.1:8081/mcp");
});
