# Tomato Support Chatbot (JavaScript)

JavaScript (Node 20+, ESM) port of the Lecture 4 terminal chatbot: a customer
support assistant for the "Tomato" food delivery app. It keeps the whole
conversation in memory and resends it (with the system prompt) on every call
to the OpenAI Chat Completions API.

## Files

| JavaScript file | Ported from                                              |
| --------------- | -------------------------------------------------------- |
| `main.js`       | `pythonDemo/src/pythondemo/main.py` (same as `goDemo/main.go`) |

## Setup

```bash
npm install
cp .env.example .env   # then put your real OPENAI_API_KEY in .env
```

`OPENAI_MODEL` is optional; it defaults to `gpt-5.6-luna`, as in the Python and Go
versions.

## Run

```bash
npm start
```

Commands inside the chat:

```text
/history  -> Show conversation history
/reset    -> Clear conversation memory
/exit     -> Exit chatbot
```

Ctrl+C or Ctrl+D also stops the chat.
