// JavaScript port of goDemo/main_using_sdk.go
// Tool-calling chat agent built on the official `openai` SDK, using the
// Responses API (client.responses.create) instead of Chat Completions.

import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import dotenv from "dotenv";
import OpenAI from "openai";

// In Go, main_using_sdk.go shares a package with main_without_using_sdk.go,
// so the weather / exchange tools live there. We import them the same way.
import { currentWeather, getExchangeRate } from "./mainWithoutUsingSdk.js";

// Load OPENAI_API_KEY / WEATHER_API_KEY from javascriptDemo/.env
const __dirname = path.dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: path.join(__dirname, ".env"), quiet: true });

const model = "gpt-4o-mini";

const client = new OpenAI();

// Builds a Responses API function tool. (Note: in the Responses API the
// name/description/parameters sit at the top level, not under "function".)
function functionTool(name, description, properties, required) {
  return {
    type: "function",
    name,
    description,
    parameters: {
      type: "object",
      properties,
      required,
    },
  };
}

export function calculate(operation, a, b) {
  console.log("Calculator tool called");

  switch (operation) {
    case "add":
      return a + b;

    case "subtract":
      return a - b;

    case "multiply":
      return a * b;

    case "divide":
      if (b === 0) {
        throw new Error("cannot divide by 0");
      }
      return a / b;

    case "mod":
      if (b === 0) {
        throw new Error("cannot calculate mod by 0");
      }
      return a % b; // same semantics as Go's math.Mod

    case "power":
      return Math.pow(a, b);

    default:
      throw new Error(`unsupported operation: ${operation}`);
  }
}

const chatTools = [
  functionTool(
    "calculate",
    "Performs arithmetic calculations. Supported operations: add, subtract, multiply, divide, mod, power.",
    {
      operation: {
        type: "string",
        description: "Operation: add, subtract, multiply, divide, mod, power",
      },
      a: {
        type: "number",
        description: "First number",
      },
      b: {
        type: "number",
        description: "Second number",
      },
    },
    ["operation", "a", "b"],
  ),

  functionTool(
    "currentWeather",
    "Get the current weather of a city.",
    {
      city: {
        type: "string",
        description: "Name of the city",
      },
    },
    ["city"],
  ),

  functionTool(
    "getExchangeRate",
    "Gets the latest exchange rate between two currencies.",
    {
      from: {
        type: "string",
        description: "Source currency code, for example USD",
      },
      to: {
        type: "string",
        description: "Target currency code, for example INR",
      },
    },
    ["from", "to"],
  ),
];

const chatSystemPrompt = `
You are a helpful AI assistant with access to external tools.

Follow these rules:
1. For arithmetic calculations, ALWAYS use the calculator tool.
2. Always use calculator tool for even trivial calculation.
3. For current weather, ALWAYS use the currentWeather tool.
4. For currency conversion or exchange rates, ALWAYS use the getExchangeRate tool.
5. You may call multiple tools when solving a multi-step request.
6. After receiving tool results, explain the answer naturally.
7. Never invent current weather or exchange-rate information.
`;

// `handlers` maps a tool name to a function that receives the parsed JSON
// arguments and returns (or resolves to) a string.
export async function chat(message, handlers) {
  let response = await client.responses.create({
    model,

    input: chatSystemPrompt + "\n\nUser: " + message,

    tools: chatTools,
  });

  while (true) {
    const toolOutputs = [];
    let hasToolCall = false;

    for (const item of response.output) {
      if (item.type !== "function_call") {
        continue;
      }

      hasToolCall = true;

      const handler = handlers[item.name];

      if (!handler) {
        throw new Error(`unknown tool: ${item.name}`);
      }

      let result;
      try {
        result = await handler(JSON.parse(item.arguments));
      } catch (err) {
        result = `Tool execution failed: ${err.message}`;
      }

      toolOutputs.push({
        type: "function_call_output",
        call_id: item.call_id,
        output: result,
      });
    }

    if (!hasToolCall) {
      return response.output_text;
    }

    // Continue the same conversation on the server by pointing at the previous
    // response, and send back only the tool outputs.
    // (Like the Go version, `tools` is not re-sent on this follow-up call.)
    response = await client.responses.create({
      model,
      previous_response_id: response.id,

      input: toolOutputs,
    });
  }
}

// main_using_sdk.go has no main(); the caller passes in the tool handlers.
// When this file is run directly (`node mainUsingSdk.js`) we wire up the
// handlers and try the same chat examples as the Go demo.
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const handlers = {
    calculate: (args) => String(calculate(args.operation, args.a, args.b)),
    currentWeather: (args) => currentWeather(args.city),
    getExchangeRate: (args) => getExchangeRate(args.from, args.to),
  };

  console.log("AI:", await chat("What is 25 * 8?", handlers));
  console.log("AI:", await chat("What is the weather in Mumbai?", handlers));
  console.log("AI:", await chat("Convert USD to INR.", handlers));
}
