// JavaScript port of pythonDemo/src/pythondemo/app.py
// (plus the HTTP handling of ChatController.Chat in the Go demos)
// FastAPI -> express. POST /chat streams the reply back chunk by chunk.

import express from "express";

export function createApp(chatController) {
  const app = express();

  app.use(express.json());

  // Request body: { "message": "..." }  (the ChatRequest model in Python)
  app.post("/chat", async (req, res) => {
    const message = req.body?.message;

    if (typeof message !== "string" || message === "") {
      return res.status(400).type("text/plain").send("message is required\n");
    }

    // Same as StreamingResponse(..., media_type="text/event-stream")
    res.setHeader("Content-Type", "text/event-stream");
    res.setHeader("Cache-Control", "no-cache");
    res.setHeader("Connection", "keep-alive");
    res.flushHeaders();

    // If the browser goes away, cancel the OpenAI request (Go uses r.Context()).
    const abortController = new AbortController();
    res.on("close", () => {
      if (!res.writableFinished) {
        abortController.abort();
      }
    });

    try {
      for await (const delta of chatController.chat(message, abortController.signal)) {
        // Every chunk is written (and flushed) to the client immediately.
        res.write(delta);
      }
    } catch (err) {
      console.error(`chat error: ${err.message ?? err}`);
    } finally {
      res.end();
    }
  });

  // Anything other than POST on /chat
  app.all("/chat", (req, res) => {
    res.status(405).type("text/plain").send("method not allowed\n");
  });

  // Invalid JSON body
  app.use((err, req, res, next) => {
    if (err.type === "entity.parse.failed") {
      return res.status(400).type("text/plain").send("invalid request body\n");
    }
    next(err);
  });

  return app;
}
