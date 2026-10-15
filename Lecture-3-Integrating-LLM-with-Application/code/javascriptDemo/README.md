# Ticket Summarizer (JavaScript)

JavaScript (Node 20+, ESM) port of the Lecture 3 ticket summarizer. Every
program exposes the same endpoint:

```text
POST /api/summarize
Content-Type: text/plain
```

The request body is the raw ticket text; the response is a 2-line summary
(plain text) produced by `gpt-4o-mini`.

## Files

| JavaScript file          | Ported from                                          | Port | Notes                                                    |
| ------------------------ | ---------------------------------------------------- | ---- | -------------------------------------------------------- |
| `main.js`                | `pythonDemo/src/pythondemo/main.py`                  | 8000 | Express + official `openai` SDK (FastAPI + AsyncOpenAI)  |
| `mainUsingSdk.js`        | `goDemo/ticket-summarizer/main-using-go-sdk.go`      | 8080 | Express + official `openai` SDK                          |
| `mainWithoutUsingSdk.js` | `goDemo/ticket-summarizer/main-without-using-sdk.go` | 8080 | No SDK: calls `https://api.openai.com/v1/chat/completions` with `fetch` |

`main.js` listens on 8000 because the Python app is served with uvicorn, whose
default port is 8000 (override with `PORT=...`). The two Go ports use 8080, like
the Go programs.

## Setup

```bash
npm install
cp .env.example .env   # then put your real OPENAI_API_KEY in .env
```

## Run

```bash
npm start              # main.js                (port 8000)
npm run start:sdk      # mainUsingSdk.js        (port 8080)
npm run start:no-sdk   # mainWithoutUsingSdk.js (port 8080)
```

Send a ticket as plain text (use port 8000 for `main.js`):

```bash
curl -X POST http://localhost:8080/api/summarize \
  -H "Content-Type: text/plain" \
  --data "The printer is offline and restarting it did not fix the issue."
```
