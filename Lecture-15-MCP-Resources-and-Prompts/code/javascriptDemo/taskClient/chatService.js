// Chat logic (JS port of pythonDemo/taskClient/chat_service.py)
//
// chat(message)  -> reads the task://guidelines RESOURCE, puts it in the
//                   instructions, then lets the model use the MCP TOOLS
// planDay(hours) -> gets the plan_day PROMPT from the server and sends it to
//                   the model, which again can use the MCP TOOLS

import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import OpenAI from "openai";

const openai = new OpenAI();
const model = process.env.OPENAI_MODEL || "gpt-5.4-mini";
const mcpUrl = process.env.MCP_SERVER_URL || "http://127.0.0.1:8081/mcp";

// Python's `async with Client(mcp_url) as mcp_client:` - connect, run fn, close.
async function withMcpClient(fn) {
  const mcpClient = new Client({ name: "task-client", version: "1.0.0" });
  await mcpClient.connect(new StreamableHTTPClientTransport(new URL(mcpUrl)));
  try {
    return await fn(mcpClient);
  } finally {
    await mcpClient.close();
  }
}

function toOpenAITools(mcpTools) {
  return mcpTools.map((tool) => ({
    type: "function",
    name: tool.name,
    description: tool.description,
    parameters: tool.inputSchema,
  }));
}

function toolResultText(result) {
  const values = [];
  for (const block of result.content) {
    if (block.type === "text") {
      values.push(block.text);
    } else {
      values.push(JSON.stringify(block));
    }
  }
  return values.join("\n");
}

async function askModel(mcpClient, message, instructions = undefined) {
  const listedTools = await mcpClient.listTools();
  const tools = toOpenAITools(listedTools.tools);
  const inputs = [{ role: "user", content: message }];

  for (let i = 0; i < 10; i++) {
    const response = await openai.responses.create({
      model,
      instructions,
      tools,
      input: inputs,
    });

    inputs.push(...response.output);
    const toolCalls = response.output.filter((item) => item.type === "function_call");

    if (toolCalls.length === 0) {
      return response.output_text;
    }

    for (const toolCall of toolCalls) {
      const result = await mcpClient.callTool({
        name: toolCall.name,
        arguments: JSON.parse(toolCall.arguments),
      });
      inputs.push({
        type: "function_call_output",
        call_id: toolCall.call_id,
        output: toolResultText(result),
      });
    }
  }

  throw new Error("The model used too many tool-calling rounds.");
}

export async function chat(message) {
  return withMcpClient(async (mcpClient) => {
    const result = await mcpClient.readResource({ uri: "task://guidelines" });
    const content = result.contents[0];

    if (typeof content?.text !== "string") {
      throw new TypeError("The task guidelines resource must contain text.");
    }

    const instructions = `You are a task management assistant.

Use the following task management guidelines whenever they are relevant
to the user's question.

TASK GUIDELINES:

${content.text}`;

    return askModel(mcpClient, message, instructions);
  });
}

export async function planDay(hours) {
  return withMcpClient(async (mcpClient) => {
    const result = await mcpClient.getPrompt({
      name: "plan_day",
      arguments: { availableHours: hours },
    });
    const content = result.messages[0].content;

    if (content.type !== "text") {
      throw new TypeError("The plan_day prompt must contain text.");
    }

    return askModel(mcpClient, content.text);
  });
}
