// Entry point for the SDK version.
// Port of goDemo/main_using_sdk.go (and of running app.py with FastAPI).

import path from "node:path";
import { fileURLToPath } from "node:url";

import dotenv from "dotenv";

import { createApp } from "./app.js";
import { ChatController } from "./chatController.js";
import { ChatService } from "./chatService.js";

// Load .env (from the javascriptDemo folder)
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const { error } = dotenv.config({ path: path.join(__dirname, ".env"), quiet: true });

if (error) {
  console.log("Warning: .env file could not be loaded");
}

if (!process.env.OPENAI_API_KEY) {
  console.error("OPENAI_API_KEY is not present in your .env file.");
  process.exit(1);
}

// Official OpenAI SDK client lives inside ChatService
const chatService = new ChatService();
const chatController = new ChatController(chatService);

const app = createApp(chatController);

app.listen(8080, () => {
  console.log("Server listening on http://localhost:8080");
});
