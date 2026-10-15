# Lecture 15 - MCP Resources and Prompts (Go)

Go port of `../pythonDemo`. Same tools, resource, prompt, ports, routes and behaviour.

| Go file | Python file | What it is |
| --- | --- | --- |
| `taskServer/main.go` | `taskServer/server.py` | MCP server on `http://127.0.0.1:8081/mcp` (Streamable HTTP, with sessions). Registers the tools, the resource and the prompt |
| `taskServer/task_tools.go` | `taskServer/task_tools.py` | In-memory task list: create / list / complete |
| `taskServer/task_resources.go` | `taskServer/task_resources.py` | Text of the `task://guidelines` resource |
| `taskServer/task_prompts.go` | `taskServer/task_promnpts.py` | Text of the `plan_day` prompt |
| `taskClient/main.go` | `taskClient/main.py` | HTTP server on `http://127.0.0.1:8000` with `GET /ask?message=...` and `GET /plan?hours=...` |
| `taskClient/chat_service.go` | `taskClient/chat_service.py` | MCP client + OpenAI Responses API tool-calling loop |

What the server exposes:

- Tools: `create_task(title)`, `list_tasks()`, `complete_task(title)`
- Resource: `task://guidelines` (name `task-guidelines`, `text/plain`)
- Prompt: `plan_day(availableHours)`

How the client uses them:

- `/ask` reads the `task://guidelines` resource, puts it in the model's
  instructions, and lets the model call the task tools.
- `/plan` gets the `plan_day` prompt from the server and sends it to the model,
  which calls `list_tasks` and answers with a plan.

Libraries: the official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk/mcp`),
`github.com/openai/openai-go/v3`, `github.com/joho/godotenv`.

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
curl "http://127.0.0.1:8000/ask?message=Add%20a%20task%3A%20write%20the%20report"
curl "http://127.0.0.1:8000/plan?hours=4"
```

The servers are interchangeable: this client also works with the Python or JS
server, and the Python/JS clients work with this server.
