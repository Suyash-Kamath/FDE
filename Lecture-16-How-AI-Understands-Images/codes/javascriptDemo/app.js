import path from "node:path";
import { fileURLToPath } from "node:url";

import dotenv from "dotenv";
import express from "express";
import multer from "multer";
import OpenAI from "openai";

import { ChatService } from "./chatService.js";

const DIRECTORY = path.dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: path.join(DIRECTORY, ".env"), quiet: true });

const MAX_CONTENT_LENGTH = 10 * 1024 * 1024;
const TOO_LARGE_MESSAGE = "Maximum request size is 10 MB.";

export function createApp(chatService) {
  const app = express();
  const staticFolder = path.join(DIRECTORY, "static");

  if (chatService == null) {
    const apiKey = process.env.OPENAI_API_KEY;
    if (!apiKey) {
      throw new Error("Set OPENAI_API_KEY in .env before starting the demo.");
    }
    chatService = new ChatService(
      new OpenAI({ apiKey, timeout: 60_000, maxRetries: 0 }),
      process.env.OPENAI_MODEL || "gpt-4o-mini",
    );
  }

  // Uploaded images stay in memory (like Flask's FileStorage) and are capped at 10 MB.
  const upload = multer({
    storage: multer.memoryStorage(),
    limits: { fileSize: MAX_CONTENT_LENGTH, fieldSize: MAX_CONTENT_LENGTH },
  });

  // Same idea as Flask's MAX_CONTENT_LENGTH: reject oversized requests up front.
  function limitRequestSize(req, res, next) {
    if (Number(req.headers["content-length"] ?? 0) > MAX_CONTENT_LENGTH) {
      // Discard the body (without storing it) before answering, so the client
      // isn't cut off mid-upload and actually sees the 413 response.
      req.resume();
      req.once("end", () => {
        res.status(413).type("text/plain").send(TOO_LARGE_MESSAGE);
      });
      return;
    }
    next();
  }

  app.get(["/", "/index.html"], (req, res) => {
    res.sendFile(path.join(staticFolder, "index.html"));
  });
  app.use("/static", express.static(staticFolder));

  app.post(
    "/api/chat",
    limitRequestSize,
    // Accept both multipart (from the browser's FormData) and plain url-encoded forms.
    upload.single("image"),
    express.urlencoded({ extended: false, limit: MAX_CONTENT_LENGTH }),
    async (req, res) => {
      const message = req.body?.message;
      if (message === undefined) {
        res.status(400).type("text/plain").send("The message field is required.");
        return;
      }
      const image = req.file;
      if (image !== undefined && !image.mimetype.startsWith("image/")) {
        res.status(400).type("text/plain").send("Please upload an image file.");
        return;
      }
      try {
        const reply = await chatService.chat(message, image);
        res.type("text/plain").send(reply);
      } catch {
        // Avoid exposing credentials or provider internals to the frontend.
        res
          .status(502)
          .type("text/plain")
          .send("Unable to get an AI response. Check your API key and try again.");
      }
    },
  );

  app.delete("/api/chat", async (req, res) => {
    await chatService.clearHistory();
    res.status(200).end();
  });

  // Upload/body size errors from multer or express.urlencoded become a 413.
  app.use((err, req, res, next) => {
    if (
      err.status === 413 ||
      err.code === "LIMIT_FILE_SIZE" ||
      err.code === "LIMIT_FIELD_VALUE"
    ) {
      res.status(413).type("text/plain").send(TOO_LARGE_MESSAGE);
      return;
    }
    if (err instanceof multer.MulterError) {
      res.status(400).type("text/plain").send(err.message);
      return;
    }
    next(err);
  });

  return app;
}

// Equivalent of Python's `if __name__ == "__main__":`
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const port = Number(process.env.PORT || "8080");
  createApp().listen(port, "127.0.0.1", () => {
    console.log(`Running on http://127.0.0.1:${port}`);
  });
}
