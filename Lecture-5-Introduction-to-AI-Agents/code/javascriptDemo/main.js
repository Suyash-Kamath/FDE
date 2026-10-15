// JavaScript port of pythonDemo/main.py
// Two tool-calling agents built on the official `openai` SDK (Chat Completions API):
//   1. chat()            -> calculator / weather / currency-exchange tools
//   2. generateWebsite() -> filesystem tools that build a static website in ./generated-sites

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import dotenv from "dotenv";
import OpenAI from "openai";

// Load OPENAI_API_KEY / WEATHER_API_KEY from javascriptDemo/.env
const __dirname = path.dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: path.join(__dirname, ".env"), quiet: true });

const client = new OpenAI();
const model = "gpt-4o-mini";
const websiteWorkspace = path.resolve("generated-sites");
const chatHistory = [];
const websiteHistory = [];

const CHAT_SYSTEM_PROMPT = `You are a helpful AI assistant with access to external tools.

Follow these rules:
1. For arithmetic calculations, ALWAYS use the calculator tool.
2. Always use calculator tool for even trivial calculation
3. For current weather, ALWAYS use the currentWeather tool.
4. For currency conversion or exchange rates, ALWAYS use the convertCurrency tool.
5. You may call multiple tools when solving a multi-step request.
6. After receiving tool results, explain the answer naturally.
7. Never invent current weather or exchange-rate information.`;

const WEBSITE_SYSTEM_PROMPT = `You are an expert frontend website developer.

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

export function calculate(operation, a, b) {
  console.log("Calculator tool called");

  if (operation === "add") return a + b;
  if (operation === "subtract") return a - b;
  if (operation === "multiply") return a * b;
  if (operation === "divide") {
    if (b === 0) throw new Error("Cannot divide by 0");
    return a / b;
  }
  if (operation === "mod") {
    if (b === 0) throw new Error("Cannot calculate mod by 0");
    // Python's % takes the sign of the divisor, JS's % takes the sign of the dividend
    return ((a % b) + b) % b;
  }
  if (operation === "power") return Math.pow(a, b);
  throw new Error(`Unsupported operation ${operation}`);
}

async function currentWeather(city) {
  console.log("Weather tool called");
  const query = new URLSearchParams({ key: process.env.WEATHER_API_KEY, q: city });
  const response = await fetch(`https://api.weatherapi.com/v1/current.json?${query}`);
  if (!response.ok) throw new Error(`HTTP Error ${response.status}: ${await response.text()}`);
  return await response.text();
}

async function getExchangeRate(fromCurrency, toCurrency) {
  console.log("Currency Exchange tool called");
  const url = `https://api.frankfurter.dev/v2/rate/${encodeURIComponent(fromCurrency)}/${encodeURIComponent(toCurrency)}`;
  const response = await fetch(url);
  if (!response.ok) throw new Error(`HTTP Error ${response.status}: ${await response.text()}`);
  return await response.text();
}

function safePath(relativePath) {
  const resolved = path.resolve(websiteWorkspace, relativePath);
  if (resolved !== websiteWorkspace && !resolved.startsWith(websiteWorkspace + path.sep)) {
    throw new Error("Access outside generated-sites is not allowed");
  }
  return resolved;
}

// The Python version only catches OSError (filesystem errors) and turns them into a
// message for the model. Anything else (like a path escaping the workspace) is re-thrown.
function isFsError(error) {
  return typeof error?.code === "string";
}

function createDirectory(dirPath) {
  try {
    fs.mkdirSync(safePath(dirPath), { recursive: true });
    return `Directory created successfully: ${dirPath}`;
  } catch (error) {
    if (!isFsError(error)) throw error;
    return `Failed to create directory: ${error.message}`;
  }
}

function writeFile(filePath, content) {
  try {
    const file = safePath(filePath);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, content, "utf-8");
    return `File written successfully: ${filePath}`;
  } catch (error) {
    if (!isFsError(error)) throw error;
    return `Failed to write file: ${error.message}`;
  }
}

function readFile(filePath) {
  try {
    return fs.readFileSync(safePath(filePath), "utf-8");
  } catch (error) {
    if (!isFsError(error)) throw error;
    return `Failed to read file: ${error.message}`;
  }
}

function listFiles(dirPath) {
  try {
    const directory = safePath(dirPath);
    if (!fs.existsSync(directory)) {
      return `Directory does not exist: ${dirPath}`;
    }
    return fs
      .readdirSync(directory, { recursive: true })
      .map((item) => path.relative(websiteWorkspace, path.join(directory, item)))
      .join("\n");
  } catch (error) {
    if (!isFsError(error)) throw error;
    return `Failed to list files: ${error.message}`;
  }
}

function tool(name, description, properties) {
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

const CHAT_TOOLS = [
  tool("calculate", "Performs arithmetic calculations. Supported operations: add, subtract, multiply, divide, mod, power.", {
    operation: { type: "string", description: "Operation: add, subtract, multiply, divide, mod, power" },
    a: { type: "number", description: "First number" },
    b: { type: "number", description: "Second number" },
  }),
  tool("currentWeather", "Get the current weather of a city.", {
    city: { type: "string", description: "Name of the city" },
  }),
  tool("getExchangeRate", "Gets the latest exchange rate between two currencies.", {
    from: { type: "string", description: "Source currency code, for example USD" },
    to: { type: "string", description: "Target currency code, for example INR" },
  }),
];

const WEBSITE_TOOLS = [
  tool("createDirectory", "Creates a new directory inside the website workspace.", {
    path: { type: "string", description: "Relative directory path, for example brewlab" },
  }),
  tool("writeFile", "Creates or overwrites a text file inside the website workspace. Use this to create HTML, CSS and JavaScript files.", {
    path: { type: "string", description: "Relative file path, for example brewlab/index.html" },
    content: { type: "string", description: "Complete content that should be written into the file" },
  }),
  tool("readFile", "Reads the contents of an existing file from the website workspace.", {
    path: { type: "string", description: "Relative file path" },
  }),
  tool("listFiles", "Lists all files and directories inside a website project.", {
    path: { type: "string", description: "Relative directory path, for example brewlab" },
  }),
];

async function complete(systemPrompt, history, tools, functions, message) {
  history.push({ role: "user", content: message });
  const messages = [{ role: "system", content: systemPrompt }, ...history];

  while (true) {
    const response = await client.chat.completions.create({ model, messages, tools });
    const reply = response.choices[0].message;
    if (!reply.tool_calls?.length) {
      history.push({ role: "assistant", content: reply.content });
      return reply.content;
    }

    messages.push(reply);
    for (const call of reply.tool_calls) {
      const args = JSON.parse(call.function.arguments);
      const result = await functions[call.function.name](args);
      messages.push({
        role: "tool",
        tool_call_id: call.id,
        content: typeof result === "string" ? result : JSON.stringify(result),
      });
    }
  }
}

export function chat(message) {
  return complete(CHAT_SYSTEM_PROMPT, chatHistory, CHAT_TOOLS, {
    calculate: ({ operation, a, b }) => calculate(operation, a, b),
    currentWeather: ({ city }) => currentWeather(city),
    getExchangeRate: (args) => getExchangeRate(args.from, args.to),
  }, message);
}

export function generateWebsite(message) {
  fs.mkdirSync(websiteWorkspace, { recursive: true });
  return complete(WEBSITE_SYSTEM_PROMPT, websiteHistory, WEBSITE_TOOLS, {
    createDirectory: ({ path }) => createDirectory(path),
    writeFile: ({ path, content }) => writeFile(path, content),
    readFile: ({ path }) => readFile(path),
    listFiles: ({ path }) => listFiles(path),
  }, message);
}

// main.py only defines chat() / generate_website() (you call them from a REPL).
// When this file is run directly (`node main.js`) we try the same examples as the Go demo.
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  console.log("AI:", await chat("What is 25 * 8?"));
  console.log("AI:", await chat("What is the weather in Mumbai?"));
  console.log("AI:", await chat("Convert USD to INR."));
  console.log("Website Agent:", await generateWebsite("Create a modern coffee shop website called BrewLab."));
}
