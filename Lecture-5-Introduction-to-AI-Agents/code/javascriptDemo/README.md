# Lecture 5 - Introduction to AI Agents (JavaScript)

JavaScript (Node 20+, ESM) port of `../pythonDemo` and `../goDemo`.

| File | Port of | What it does |
| --- | --- | --- |
| `main.js` | `pythonDemo/main.py` | Two agents on the `openai` SDK (Chat Completions): `chat()` with calculator / weather / currency tools, and `generateWebsite()` with filesystem tools that write a static site into `./generated-sites`. |
| `mainUsingSdk.js` | `goDemo/main_using_sdk.go` | Chat agent on the `openai` SDK using the **Responses API** (`previous_response_id` + `function_call_output`). |
| `mainWithoutUsingSdk.js` | `goDemo/main_without_using_sdk.go` | Both agents again, but calling `https://api.openai.com/v1/chat/completions` by hand with `fetch` (no SDK). |

`main.py` and `main_using_sdk.go` only define functions (no entry point). When you run
`main.js` / `mainUsingSdk.js` directly they run the same example prompts as the Go
`main_without_using_sdk.go` `main()`. You can also import `chat` / `generateWebsite` from them.

## Setup

```bash
npm install
cp .env.example .env   # fill in OPENAI_API_KEY and WEATHER_API_KEY
```

## Run

```bash
npm start              # node main.js
npm run sdk            # node mainUsingSdk.js
npm run without-sdk    # node mainWithoutUsingSdk.js
```

Websites are generated into `generated-sites/` in the directory you run the command from.
