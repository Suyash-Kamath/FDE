# Lecture 16 - Vision Chat (Go port)

Go port of `../pythonDemo` using the official `github.com/openai/openai-go/v3` SDK and the standard
`net/http` server. Attach an image in the chat page and it is sent to OpenAI (`gpt-4o-mini` by default)
as a base64 `data:` URL inside the user message.

| File | Python original | What it is |
| --- | --- | --- |
| `main.go` | `app.py` | HTTP server: serves the page, `POST /api/chat` (multipart form with `message` + optional `image`), `DELETE /api/chat` (clear history). 10 MB request limit. |
| `chat_service.go` | `chat_service.py` | Keeps the shared in-memory history (guarded by a mutex) and calls `Chat.Completions.New` with text + image content parts. |
| `static/index.html` | `static/index.html` | The frontend (copied unchanged, embedded into the binary with `go:embed`). |
| `main_test.go` | `test_app.py` | `testing` + `net/http/httptest` test using a fake completions client (no API key needed). |

## Setup

```bash
cp .env.example .env   # then put your OPENAI_API_KEY in .env
```

## Run (from this folder)

```bash
go run .               # http://127.0.0.1:8080 (override with PORT)
go test ./...          # runs the tests, no API key required
```
