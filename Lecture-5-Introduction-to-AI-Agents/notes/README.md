# AI Tools & Agents — Deep Notes (Python, official SDKs)

These notes cover everything in the lecture transcript, but rebuilt from first
principles in Python instead of Spring AI. Every Java/Spring idea from the video
has a direct Python counterpart, and the notes point them out explicitly.

## Mental model in one line

> An LLM is a **pure function** `tokens -> tokens`. Tool calling is the protocol
> by which that pure function asks *your* impure code to do side effects on its
> behalf. An agent is that protocol run in a **loop** until a goal is reached.

## Reading order

| # | File | Covers (from your list) |
|---|------|--------------------------|
| 01 | [`01-why-llm-cannot-act.md`](01-why-llm-cannot-act.md) | Why an LLM cannot directly perform actions · LLM = Brain, Tools = Hands |
| 02 | [`02-tools-and-function-calling.md`](02-tools-and-function-calling.md) | What are Tools / Function Calling · Natural Language → Structured Tool Calls · what actually goes over the wire |
| 03 | [`03-tool-calling-anthropic.md`](03-tool-calling-anthropic.md) | Calculator tool, end to end, with the `anthropic` SDK (the Spring AI `@Tool` equivalent) |
| 04 | [`04-tool-calling-openai.md`](04-tool-calling-openai.md) | Same calculator with the `openai` SDK (Chat Completions **and** Responses API) |
| 05 | [`05-weather-and-currency-tools.md`](05-weather-and-currency-tools.md) | Live Weather Tool · Currency Conversion Tool · Multiple Tool Calls for a Single Request |
| 06 | [`06-the-agentic-loop.md`](06-the-agentic-loop.md) | How the Tool Calling Loop Actually Works (statelessness, message ledger, parallel calls, errors, budgets) |
| 07 | [`07-information-vs-action-tools.md`](07-information-vs-action-tools.md) | Information Tools vs Action Tools |
| 08 | [`08-agents-vs-workflows.md`](08-agents-vs-workflows.md) | When does an LLM become an AI Agent? · AI Agents vs Workflows · Goal → Decide → Act → Observe Loop |
| 09 | [`09-website-builder-agent.md`](09-website-builder-agent.md) | Building an AI Website Builder Agent · File System Tools · `create_directory`, `write_file`, `read_file`, `list_files` |
| 10 | [`10-sandboxing-and-security.md`](10-sandboxing-and-security.md) | Agent Sandboxing & Path Traversal Protection · Tool Permissions and Security |
| 11 | [`11-human-in-the-loop.md`](11-human-in-the-loop.md) | Human-in-the-Loop |
| 12 | [`12-production-checklist.md`](12-production-checklist.md) | Building practical agentic systems (the Spring AI section, translated to real production concerns) |

## Setup once, for all files

```bash
python -m venv .venv && source .venv/bin/activate
pip install anthropic openai httpx python-dotenv
```

```bash
# .env
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
WEATHER_API_KEY=...          # free key from weatherapi.com
```

```python
from dotenv import load_dotenv
load_dotenv()   # both SDKs read their key from env automatically
```

## Spring AI → Python translation table

The video is Spring AI. Nothing in it is Java-specific; here is the mapping so
you can watch the video and write Python at the same time.

| Spring AI (video) | Python equivalent |
|---|---|
| `ChatClient` | `anthropic.Anthropic()` / `openai.OpenAI()` client |
| `.prompt().system(...)` | `system=` (Anthropic) / `{"role":"system"}` or `instructions=` (OpenAI) |
| `.messages(history)` | you own the `messages` list yourself |
| `@Tool(description = "...")` | the `description` field of the tool dict |
| `@ToolParam(description = "...")` | `description` inside `input_schema.properties` |
| `.tools(calculatorTool)` | `tools=[...]` param |
| Spring auto-runs the tool & loops | **you write the loop** (or use a helper) |
| `RestClient` | `httpx.Client` |
| `IllegalArgumentException` on bad path | raise, then return as a tool error result |

The single most important difference: **Spring AI hides the loop from you.**
In Python with the raw SDKs you write it, which is exactly why you'll actually
understand agents instead of just using them.
