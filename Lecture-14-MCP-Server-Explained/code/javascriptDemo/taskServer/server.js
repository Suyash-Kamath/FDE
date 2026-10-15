// Task MCP server (JS port of pythonDemo/taskServer/server.py)
//
// Exposes three tools over Streamable HTTP at http://127.0.0.1:8081/mcp:
//   create_task   -> add a task
//   list_tasks    -> list pending tasks
//   complete_task -> remove a task by its exact title
//
// Like the Python version it runs in *stateless* mode with plain JSON
// responses (stateless_http=True, json_response=True): every POST is handled
// on its own, no MCP session id is issued, and no SSE stream is opened.

import express from "express";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { z } from "zod";

// In-memory task list. It lives at module level, so it survives across
// requests even though each request gets a fresh McpServer instance.
const tasks = [];

// Small helper: Python's MCP SDK returns a str result as one text block and
// also as structuredContent {"result": ...}. We do the same so every client
// sees identical results from the Python, JS and Go servers.
function textResult(text) {
  return {
    content: [{ type: "text", text }],
    structuredContent: { result: text },
  };
}

function createServer() {
  const mcp = new McpServer({ name: "task-mcp-server", version: "1.0.0" });

  mcp.registerTool(
    "create_task",
    {
      description: "Create a new task",
      inputSchema: {
        title: z.string().describe("Title of the task"),
      },
      outputSchema: { result: z.string() },
    },
    async ({ title }) => {
      tasks.push(title);
      return textResult(`Task created: ${title}`);
    },
  );

  mcp.registerTool(
    "list_tasks",
    {
      description: "List all pending tasks",
      outputSchema: { result: z.array(z.string()) },
    },
    async () => ({
      // Python turns a list[str] into one text block per task.
      content: tasks.map((task) => ({ type: "text", text: task })),
      structuredContent: { result: [...tasks] },
    }),
  );

  mcp.registerTool(
    "complete_task",
    {
      description: "Complete a task using its exact title",
      inputSchema: {
        title: z.string().describe("Exact title of the task"),
      },
      outputSchema: { result: z.string() },
    },
    async ({ title }) => {
      const index = tasks.indexOf(title);
      if (index === -1) {
        return textResult(`Task not found: ${title}`);
      }
      tasks.splice(index, 1);
      return textResult(`Task completed: ${title}`);
    },
  );

  return mcp;
}

const app = express();
app.use(express.json());

app.post("/mcp", async (req, res) => {
  // Stateless mode: a new server + transport per request.
  const mcp = createServer();
  const transport = new StreamableHTTPServerTransport({
    sessionIdGenerator: undefined, // no sessions (stateless_http=True)
    enableJsonResponse: true, // plain JSON instead of SSE (json_response=True)
  });

  res.on("close", () => {
    transport.close();
    mcp.close();
  });

  try {
    await mcp.connect(transport);
    await transport.handleRequest(req, res, req.body);
  } catch (error) {
    console.error("Error handling MCP request:", error);
    if (!res.headersSent) {
      res.status(500).json({
        jsonrpc: "2.0",
        error: { code: -32603, message: "Internal server error" },
        id: null,
      });
    }
  }
});

// A stateless server has no SSE stream (GET) and no session to end (DELETE).
const methodNotAllowed = (_req, res) => {
  res.status(405).json({
    jsonrpc: "2.0",
    error: { code: -32000, message: "Method not allowed." },
    id: null,
  });
};
app.get("/mcp", methodNotAllowed);
app.delete("/mcp", methodNotAllowed);

app.listen(8081, "127.0.0.1", () => {
  console.log("Task MCP server running on http://127.0.0.1:8081/mcp");
});
