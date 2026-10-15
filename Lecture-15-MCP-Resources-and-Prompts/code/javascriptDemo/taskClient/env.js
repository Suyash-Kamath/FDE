// Loads .env from the javascriptDemo folder.
// main.js imports this FIRST, so the variables are set before chatService.js
// creates the OpenAI client and reads OPENAI_MODEL / MCP_SERVER_URL.

import path from "node:path";
import { fileURLToPath } from "node:url";

import dotenv from "dotenv";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: path.join(__dirname, "..", ".env"), quiet: true });
