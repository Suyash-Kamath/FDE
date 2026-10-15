# Lecture 15 - MCP Resources and Prompts (JavaScript)

JavaScript port of `../pythonDemo`. Same tools, resource, prompt, ports, routes and behaviour.

| JS file | Python file | What it is |
| --- | --- | --- |
| `taskServer/server.js` | `taskServer/server.py` | MCP server on `http://127.0.0.1:8081/mcp` (Streamable HTTP, with sessions). Registers the tools, the resource and the prompt |
| `taskServer/taskTools.js` | `taskServer/task_tools.py` | In-memory task list: create / list / complete |
| `taskServer/taskResources.js` | `taskServer/task_resources.py` | Text of the `task://guidelines` resource |
| `taskServer/taskPrompts.js` | `taskServer/task_promnpts.py` | Text of the `plan_day` prompt |
| `taskClient/main.js` | `taskClient/main.py` | Express app on `http://127.0.0.1:8000` with `GET /ask?message=...` and `GET /plan?hours=...` |
| `taskClient/chatService.js` | `taskClient/chat_service.py` | MCP client + OpenAI Responses API tool-calling loop |
| `taskClient/env.js` | - | Loads `.env` before anything else runs |

What the server exposes:

- Tools: `create_task(title)`, `list_tasks()`, `complete_task(title)`
- Resource: `task://guidelines` (name `task-guidelines`, `text/plain`)
- Prompt: `plan_day(availableHours)`

How the client uses them:

- `/ask` reads the `task://guidelines` resource, puts it in the model's
  instructions, and lets the model call the task tools.
- `/plan` gets the `plan_day` prompt from the server and sends it to the model,
  which calls `list_tasks` and answers with a plan.

Libraries: `@modelcontextprotocol/sdk` + `zod`, `openai`, `express`, `dotenv`.

## Setup

```bash
npm install
cp .env.example .env   # then put your OPENAI_API_KEY in .env
```

## Run

Start the server first, then the client (in a second terminal):

```bash
npm run server   # node taskServer/server.js
npm run client   # node taskClient/main.js
```

Try it:

```bash
curl "http://127.0.0.1:8000/ask?message=Add%20a%20task%3A%20write%20the%20report"
curl "http://127.0.0.1:8000/plan?hours=4"
```

The servers are interchangeable: this client also works with the Python or Go
server, and the Python/Go clients work with this server.
