// Task client web app (JS port of pythonDemo/taskClient/main.py)
//
//   GET /ask?message=...  -> chat with the assistant (uses the guidelines resource + tools)
//   GET /plan?hours=...   -> plan the day (uses the plan_day prompt + tools)
//
// The Python app is started with `uvicorn main:app`, which listens on
// http://127.0.0.1:8000 by default - we use the same address.

import "./env.js"; // must be first: loads .env before chatService.js runs

import express from "express";

import { chat, planDay } from "./chatService.js";

const app = express();

// FastAPI makes `message` / `hours` required query params (422 if missing).
function requireQuery(req, res, name) {
  const value = req.query[name];
  if (typeof value !== "string") {
    res.status(422).json({ detail: `Missing query parameter: ${name}` });
    return undefined;
  }
  return value;
}

app.get("/ask", async (req, res, next) => {
  const message = requireQuery(req, res, "message");
  if (message === undefined) return;
  try {
    res.type("text/plain").send(await chat(message));
  } catch (error) {
    next(error);
  }
});

app.get("/plan", async (req, res, next) => {
  const hours = requireQuery(req, res, "hours");
  if (hours === undefined) return;
  try {
    res.type("text/plain").send(await planDay(hours));
  } catch (error) {
    next(error);
  }
});

app.listen(8000, "127.0.0.1", () => {
  console.log("Task client running on http://127.0.0.1:8000");
});
