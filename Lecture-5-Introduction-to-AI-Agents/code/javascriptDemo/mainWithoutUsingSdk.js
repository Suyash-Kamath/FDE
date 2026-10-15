// JavaScript port of goDemo/main_without_using_sdk.go
// Same two agents as main.js, but WITHOUT the OpenAI SDK: we call
// https://api.openai.com/v1/chat/completions ourselves using the built-in fetch.

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import dotenv from "dotenv";

// Load OPENAI_API_KEY / WEATHER_API_KEY / OPENAI_MODEL from javascriptDemo/.env
const __dirname = path.dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: path.join(__dirname, ".env"), quiet: true });

const defaultModel = "gpt-4o-mini";

const chatSystemPrompt = `You are a helpful AI assistant with access to external tools.

Follow these rules:
1. For arithmetic calculations, ALWAYS use the calculator tool.
2. Always use calculator tool for even trivial calculation
3. For current weather, ALWAYS use the currentWeather tool.
4. For currency conversion or exchange rates, ALWAYS use the convertCurrency tool.
5. You may call multiple tools when solving a multi-step request.
6. After receiving tool results, explain the answer naturally.
7. Never invent current weather or exchange-rate information.`;

const websiteSystemPrompt = `You are an expert frontend website developer.

Your job is to create complete static websites using the available tools.

Follow these rules:
1. Create a separate directory for every website.
2. Create index.html.
3. Create style.css.
4. Create script.js when JavaScript is useful.
5. Build modern, beautiful and responsive websites.
6. Use only HTML, CSS and vanilla JavaScript.
7. Do not just return website code in your response. Actually create the files using tools.
8. After creating the website, list the project files.
9. Read important files again if needed and fix obvious problems.
10. Finish only when the complete website has been created.`;

const chatHistory = [];
const websiteHistory = [];

// ------------------------------------------------------------
// OpenAI client (plain HTTP)
// ------------------------------------------------------------

export class OpenAIClient {
  constructor() {
    this.apiKey = process.env.OPENAI_API_KEY ?? "";
    this.model = process.env.OPENAI_MODEL || defaultModel;
    this.timeoutMs = 120 * 1000;
  }

  async chatCompletion(messages, tools) {
    if (!this.apiKey) {
      throw new Error("OPENAI_API_KEY is not set");
    }

    const requestBody = {
      model: this.model,
      messages,
      tools,
    };

    let resp;
    try {
      resp = await fetch("https://api.openai.com/v1/chat/completions", {
        method: "POST",
        headers: {
          Authorization: "Bearer " + this.apiKey,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(requestBody),
        signal: AbortSignal.timeout(this.timeoutMs),
      });
    } catch (err) {
      throw new Error(`OpenAI request failed: ${err.message}`);
    }

    const responseBody = await resp.text();

    if (resp.status < 200 || resp.status >= 300) {
      throw new Error(`OpenAI API returned ${resp.status}: ${responseBody}`);
    }

    try {
      return JSON.parse(responseBody);
    } catch (err) {
      throw new Error(`decode OpenAI response: ${err.message}`);
    }
  }
}

// ------------------------------------------------------------
// Tool helpers
// ------------------------------------------------------------

function functionTool(name, description, properties) {
  return {
    type: "function",
    function: {
      name,
      description,
      parameters: {
        type: "object",
        properties,
        required: Object.keys(properties),
      },
    },
  };
}

// ------------------------------------------------------------
// Calculator
// ------------------------------------------------------------

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
      if (b === 0) throw new Error("cannot divide by 0");
      return a / b;
    case "mod":
      if (b === 0) throw new Error("cannot calculate mod by 0");
      return a % b; // same semantics as Go's math.Mod
    case "power":
      return Math.pow(a, b);
    default:
      throw new Error(`unsupported operation "${operation}"`);
  }
}

// ------------------------------------------------------------
// Weather
// ------------------------------------------------------------

export async function currentWeather(city) {
  console.log("Weather tool called");

  const apiKey = process.env.WEATHER_API_KEY;

  if (!apiKey) {
    throw new Error("WEATHER_API_KEY is not set");
  }

  const params = new URLSearchParams({ key: apiKey, q: city });
  const requestURL = "https://api.weatherapi.com/v1/current.json?" + params;

  let resp;
  try {
    resp = await fetch(requestURL);
  } catch (err) {
    throw new Error(`weather request failed: ${err.message}`);
  }

  const body = await resp.text();

  if (resp.status < 200 || resp.status >= 300) {
    throw new Error(`weather API returned ${resp.status}: ${body}`);
  }

  return body;
}

// ------------------------------------------------------------
// Currency Exchange
// ------------------------------------------------------------

export async function getExchangeRate(fromCurrency, toCurrency) {
  console.log("Currency Exchange tool called");

  const requestURL = `https://api.frankfurter.dev/v2/rate/${encodeURIComponent(fromCurrency)}/${encodeURIComponent(toCurrency)}`;

  let resp;
  try {
    resp = await fetch(requestURL);
  } catch (err) {
    throw new Error(`exchange request failed: ${err.message}`);
  }

  const body = await resp.text();

  if (resp.status < 200 || resp.status >= 300) {
    throw new Error(`exchange API returned ${resp.status}: ${body}`);
  }

  return body;
}

// ------------------------------------------------------------
// Safe filesystem workspace
// ------------------------------------------------------------

export class WebsiteWorkspace {
  constructor() {
    this.root = path.resolve("generated-sites");
  }

  safePath(relativePath) {
    // path.join also normalises things like ./foo/../bar
    // and makes the path absolute relative to the workspace.
    const resolved = path.resolve(path.join(this.root, relativePath));

    const relative = path.relative(this.root, resolved);

    // Prevent:
    // ../../something
    // ../secret
    // etc.
    if (relative === ".." || relative.startsWith(".." + path.sep)) {
      throw new Error("access outside generated-sites is not allowed");
    }

    return resolved;
  }

  // ------------------------------------------------------------
  // createDirectory
  // ------------------------------------------------------------

  createDirectory(relativePath) {
    const dir = this.safePath(relativePath);

    try {
      fs.mkdirSync(dir, { recursive: true });
    } catch (err) {
      throw new Error(`failed to create directory: ${err.message}`);
    }

    return `Directory created successfully: ${relativePath}`;
  }

  // ------------------------------------------------------------
  // writeFile
  // ------------------------------------------------------------

  writeFile(relativePath, content) {
    const file = this.safePath(relativePath);

    try {
      fs.mkdirSync(path.dirname(file), { recursive: true });
    } catch (err) {
      throw new Error(`failed to create parent directory: ${err.message}`);
    }

    try {
      fs.writeFileSync(file, content);
    } catch (err) {
      throw new Error(`failed to write file: ${err.message}`);
    }

    return `File written successfully: ${relativePath}`;
  }

  // ------------------------------------------------------------
  // readFile
  // ------------------------------------------------------------

  readFile(relativePath) {
    const file = this.safePath(relativePath);

    try {
      return fs.readFileSync(file, "utf-8");
    } catch (err) {
      throw new Error(`failed to read file: ${err.message}`);
    }
  }

  // ------------------------------------------------------------
  // listFiles
  // ------------------------------------------------------------

  listFiles(relativePath) {
    const directory = this.safePath(relativePath);

    if (!fs.existsSync(directory)) {
      return `Directory does not exist: ${relativePath}`;
    }

    try {
      // Sorted, like Go's filepath.Walk (lexical order)
      return fs
        .readdirSync(directory, { recursive: true })
        .sort()
        .map((item) => path.relative(this.root, path.join(directory, item)))
        .join("\n");
    } catch (err) {
      throw new Error(`failed to list files: ${err.message}`);
    }
  }
}

// ------------------------------------------------------------
// Tool definitions
// ------------------------------------------------------------

const chatTools = [
  functionTool(
    "calculate",
    "Performs arithmetic calculations. Supported operations: add, subtract, multiply, divide, mod, power.",
    {
      operation: { type: "string", description: "Operation: add, subtract, multiply, divide, mod, power" },
      a: { type: "number", description: "First number" },
      b: { type: "number", description: "Second number" },
    },
  ),

  functionTool("currentWeather", "Get the current weather of a city.", {
    city: { type: "string", description: "Name of the city" },
  }),

  functionTool("getExchangeRate", "Gets the latest exchange rate between two currencies.", {
    from: { type: "string", description: "Source currency code, for example USD" },
    to: { type: "string", description: "Target currency code, for example INR" },
  }),
];

const websiteTools = [
  functionTool("createDirectory", "Creates a new directory inside the website workspace.", {
    path: { type: "string", description: "Relative directory path, for example brewlab" },
  }),

  functionTool(
    "writeFile",
    "Creates or overwrites a text file inside the website workspace. Use this to create HTML, CSS and JavaScript files.",
    {
      path: { type: "string", description: "Relative file path, for example brewlab/index.html" },
      content: { type: "string", description: "Complete content that should be written into the file" },
    },
  ),

  functionTool("readFile", "Reads the contents of an existing file from the website workspace.", {
    path: { type: "string", description: "Relative file path" },
  }),

  functionTool("listFiles", "Lists all files and directories inside a website project.", {
    path: { type: "string", description: "Relative directory path, for example brewlab" },
  }),
];

// ------------------------------------------------------------
// Generic agent completion loop
// ------------------------------------------------------------

async function complete(client, systemPrompt, history, tools, handlers, userMessage) {
  // Same behavior as Python:
  //
  // history.append({
  //     "role": "user",
  //     "content": message
  // })
  history.push({ role: "user", content: userMessage });

  // Build local request history.
  //
  // Important:
  // Tool messages are NOT persisted into chatHistory/websiteHistory.
  // This matches the Python implementation.
  const messages = [{ role: "system", content: systemPrompt }, ...history];

  while (true) {
    const response = await client.chatCompletion(messages, tools);

    if (!response.choices?.length) {
      throw new Error("OpenAI returned no choices");
    }

    const reply = response.choices[0].message;

    // No tool calls -> final response.
    if (!reply.tool_calls?.length) {
      history.push({ role: "assistant", content: reply.content });
      return reply.content;
    }

    // Add assistant message containing tool calls.
    messages.push(reply);

    // Execute every tool call returned by the model.
    for (const call of reply.tool_calls) {
      const handler = handlers[call.function.name];

      if (!handler) {
        throw new Error(`unknown tool: ${call.function.name}`);
      }

      let result;
      try {
        result = await handler(JSON.parse(call.function.arguments));
      } catch (err) {
        result = `Tool execution failed: ${err.message}`;
      }

      // Add tool result to current agent turn.
      messages.push({
        role: "tool",
        tool_call_id: call.id,
        content: result,
      });
    }
  }
}

// ------------------------------------------------------------
// Chat agent
// ------------------------------------------------------------

export function chat(client, message) {
  return complete(client, chatSystemPrompt, chatHistory, chatTools, {
    calculate: (args) => String(calculate(args.operation, args.a, args.b)),
    currentWeather: (args) => currentWeather(args.city),
    getExchangeRate: (args) => getExchangeRate(args.from, args.to),
  }, message);
}

// ------------------------------------------------------------
// Website generation agent
// ------------------------------------------------------------

export function generateWebsite(client, workspace, message) {
  try {
    fs.mkdirSync(workspace.root, { recursive: true });
  } catch (err) {
    throw new Error(`create website workspace: ${err.message}`);
  }

  return complete(client, websiteSystemPrompt, websiteHistory, websiteTools, {
    createDirectory: (args) => workspace.createDirectory(args.path),
    writeFile: (args) => workspace.writeFile(args.path, args.content),
    readFile: (args) => workspace.readFile(args.path),
    listFiles: (args) => workspace.listFiles(args.path),
  }, message);
}

// ------------------------------------------------------------
// Example main
// ------------------------------------------------------------

async function main() {
  const client = new OpenAIClient();
  const workspace = new WebsiteWorkspace();

  // Example 1:
  let answer = await chat(client, "What is 25 * 8?");
  console.log("AI:", answer);

  // Example 2:
  answer = await chat(client, "What is the weather in Mumbai?");
  console.log("AI:", answer);

  // Example 3:
  answer = await chat(client, "Convert USD to INR.");
  console.log("AI:", answer);

  // Example 4:
  answer = await generateWebsite(client, workspace, "Create a modern coffee shop website called BrewLab.");
  console.log("Website Agent:", answer);
}

// Only run the examples when this file is executed directly
// (mainUsingSdk.js imports the weather / exchange tools from here).
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
