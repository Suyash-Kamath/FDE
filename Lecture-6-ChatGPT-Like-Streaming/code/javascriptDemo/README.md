# Lecture 6 - ChatGPT-Like Streaming (JavaScript)

JavaScript (Node 20+, ESM, express) port of `../pythonDemo` and `../goDemo`.
A sarcastic chatbot server: `POST /chat` with `{"message": "..."}` streams the reply back
chunk by chunk (`text/event-stream`, raw text deltas) and remembers the conversation history.

| File | Port of | What it does |
| --- | --- | --- |
| `chatService.js` | `pythondemo/chat_service.py` (and `ChatService` in `main_using_sdk.go`) | Streams from the OpenAI Responses API using the `openai` SDK. An async generator that yields each delta. |
| `chatServiceWithoutSdk.js` | `ChatService` in `main_without_using_sdk.go` | Same thing with raw `fetch` to `https://api.openai.com/v1/responses` (`"stream": true`), parsing the SSE `data:` lines by hand. |
| `chatController.js` | `pythondemo/chat_controller.py` | Thin layer between the HTTP route and the service. |
| `app.js` | `pythondemo/app.py` (+ HTTP part of `ChatController.Chat` in Go) | express app with `POST /chat` that writes every delta to the response as it arrives. |
| `mainUsingSdk.js` | `goDemo/main_using_sdk.go` | Entry point: loads `.env`, wires `ChatService` (SDK), listens on `:8080`. |
| `mainWithoutUsingSdk.js` | `goDemo/main_without_using_sdk.go` | Entry point: same, using `ChatServiceWithoutSdk` (no SDK). |

## Setup

```bash
npm install
cp .env.example .env   # fill in OPENAI_API_KEY
```

## Run

```bash
npm start              # node mainUsingSdk.js       -> http://localhost:8080
npm run without-sdk    # node mainWithoutUsingSdk.js -> http://localhost:8080
```

Try it (`-N` turns off curl's buffering so you can watch the stream):

```bash
curl -N -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Tell me a joke"}'
```

## About `../index.html`

Like the Python and Go servers, this server does **not** serve `../index.html`; it only exposes
`POST /chat`. Note that the page, as written, calls `http://localhost:8080/api/chat` with a
`text/plain` body, while all three servers expose `/chat` and expect JSON `{"message": ...}`.
