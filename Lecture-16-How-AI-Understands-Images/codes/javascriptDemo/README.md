# Lecture 16 - Vision Chat (JavaScript port)

JavaScript (Node 20+, ESM) port of `../pythonDemo`. A chat page where you can attach an image;
the image is sent to OpenAI (`gpt-4o-mini` by default) as a base64 `data:` URL inside the user message.

| File | Python original | What it is |
| --- | --- | --- |
| `app.js` | `app.py` | Express server: serves the page, `POST /api/chat` (multipart form with `message` + optional `image`), `DELETE /api/chat` (clear history). 10 MB request limit. |
| `chatService.js` | `chat_service.py` | Keeps the shared in-memory history and calls `client.chat.completions.create` with text + image content parts. |
| `static/index.html` | `static/index.html` | The frontend (copied unchanged). |
| `test/app.test.js` | `test_app.py` | `node:test` + `supertest` test using a fake OpenAI client (no API key needed). |

## Setup

```bash
npm install
cp .env.example .env   # then put your OPENAI_API_KEY in .env
```

## Run

```bash
npm start              # http://127.0.0.1:8080 (override with PORT)
npm test               # runs the tests, no API key required
```
