import path from "node:path";
import { fileURLToPath } from "node:url";

import dotenv from "dotenv";

// javascriptDemo/ (one level above src/) - this is where the .env file lives.
export const PROJECT_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

// Load .env from the project root. Real environment variables still take precedence.
dotenv.config({ path: path.join(PROJECT_ROOT, ".env"), quiet: true });

class Settings {
    constructor() {
        // Required - like the pydantic `openai_api_key: str` field with no default.
        this.openaiApiKey = process.env.OPENAI_API_KEY;
        if (!this.openaiApiKey) {
            throw new Error("OPENAI_API_KEY is not set (add it to javascriptDemo/.env)");
        }

        this.embeddingModel = process.env.EMBEDDING_MODEL || "text-embedding-3-small";
    }
}

export const settings = new Settings();
