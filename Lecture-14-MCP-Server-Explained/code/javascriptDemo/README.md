# Lecture 14 - MCP Server Explained (JavaScript)

JavaScript port of `../pythonDemo`. Same tools, port, routes and behaviour.

| JS file | Python file | What it is |
| --- | --- | --- |
| `taskServer/server.js` | `taskServer/server.py` | MCP server with `create_task`, `list_tasks`, `complete_task` tools. Streamable HTTP, stateless, JSON responses, on `http://127.0.0.1:8081/mcp` |
| `taskClient/app.js` | `taskClient/app.py` | Express app on `http://127.0.0.1:8080` with `GET /ask?message=...`. Runs an OpenAI Agent (`@openai/agents`) that uses the MCP server's tools |

Libraries: `@modelcontextprotocol/sdk` + `zod` (server), `@openai/agents` (client), `express`, `dotenv`.

## Setup

```bash
npm install
cp .env.example .env   # then put your OPENAI_API_KEY in .env
```

## Run

Start the server first, then the client (in a second terminal):

```bash
npm run server   # node taskServer/server.js
npm run client   # node taskClient/app.js
```

Try it:

```bash
curl "http://127.0.0.1:8080/ask?message=Add%20a%20task%20to%20buy%20milk"
curl "http://127.0.0.1:8080/ask?message=What%20are%20my%20tasks%3F"
```

The servers are interchangeable: this client also works with the Python or Go
server, and the Python/Go clients work with this server.
