# Lecture 14 - MCP Server Explained (Go)

Go port of `../pythonDemo`. Same tools, port, routes and behaviour.

| Go file | Python file | What it is |
| --- | --- | --- |
| `taskServer/main.go` | `taskServer/server.py` | MCP server with `create_task`, `list_tasks`, `complete_task` tools. Streamable HTTP, stateless, JSON responses, on `http://127.0.0.1:8081/mcp` |
| `taskClient/main.go` | `taskClient/app.py` | HTTP server on `http://127.0.0.1:8080` with `GET /ask?message=...`. A "Task Assistant" that uses the MCP server's tools |

Libraries: the official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk/mcp`),
`github.com/openai/openai-go/v3`, `github.com/joho/godotenv`.

The Python client uses the OpenAI Agents SDK, which has no Go version. The Go
client does the same job by hand: it connects to the MCP server once, caches
the tool list, and runs the tool-calling loop itself with the Responses API
(same instructions, same default model `gpt-5.6-luna`, max 10 turns).

## Setup

```bash
cp .env.example .env   # then put your OPENAI_API_KEY in .env
```

## Run

Run from this `goDemo` folder (so `.env` is found). Start the server first,
then the client in a second terminal:

```bash
go run ./taskServer
go run ./taskClient
```

Try it:

```bash
curl "http://127.0.0.1:8080/ask?message=Add%20a%20task%20to%20buy%20milk"
curl "http://127.0.0.1:8080/ask?message=What%20are%20my%20tasks%3F"
```

The servers are interchangeable: this client also works with the Python or JS
server, and the Python/JS clients work with this server.
