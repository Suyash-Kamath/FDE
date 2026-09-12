# Forward Deployed Engineering — Deep Notes on "Talking to an LLM via API"

These notes expand the Coder Army (Aditya Tandon) transcript into a full reference.
The video is ~47 minutes and moves fast; each of its ideas is unpacked here down to
the mechanism, the JSON on the wire, and the production consequences.

**Read them in order.** Each file assumes the previous ones.

| # | File | What it answers |
|---|------|-----------------|
| 01 | [`01-chatgpt-vs-raw-llm.md`](01-chatgpt-vs-raw-llm.md) | ChatGPT is not an LLM. What is the actual difference, layer by layer? |
| 02 | [`02-why-llms-need-tools.md`](02-why-llms-need-tools.md) | Why can't a model do `87345 × 5623`? Why can't it count `*`? Why doesn't it know today's weather? |
| 03 | [`03-tool-calling-architecture.md`](03-tool-calling-architecture.md) | The actual tool-calling loop, with real JSON. Who calls the tool — the model or your code? |
| 04 | [`04-endpoints-auth-keys-models.md`](04-endpoints-auth-keys-models.md) | Endpoints, `Authorization: Bearer`, what an API key really is, how to pick a model. |
| 05 | [`05-first-request-postman.md`](05-first-request-postman.md) | The first call, the 404, curl equivalents, every error code you will hit. |
| 06 | [`06-understanding-the-response.md`](06-understanding-the-response.md) | Every field in the response envelope, and how to parse it without writing fragile code. |
| 07 | [`07-tokens-cost-latency-context.md`](07-tokens-cost-latency-context.md) | Tokens, why output costs 6× input, why Hindi costs more, why chat gets quadratically expensive. |
| 08 | [`08-spring-ai-and-chatclient.md`](08-spring-ai-and-chatclient.md) | What Spring AI actually does, `ChatClient`, advisors, correct property names. |
| 09 | [`09-support-ticket-summarizer.md`](09-support-ticket-summarizer.md) | The real app — and the four serious bugs in the version built on screen. |
| 10 | [`10-system-user-assistant-roles.md`](10-system-user-assistant-roles.md) | Roles, statelessness, and the real reason it forgot "my name is Aditya". |
| 11 | [`11-corrections-and-glossary.md`](11-corrections-and-glossary.md) | Where the video is imprecise, corrected. Plus a glossary and what to build next. |

---

## The one-paragraph summary of the whole video

A large language model is a frozen function: given a sequence of tokens, it outputs a
probability distribution over the next token. That is *all* it does. It cannot browse,
compute, remember, or act. Every capability you experience in ChatGPT — live weather,
correct arithmetic, memory of your name, code execution — is **software built around
that function**, not the function itself. When you call the provider's HTTP API directly
you get the bare function plus almost nothing else, which is why the same model that
answers perfectly in the chat app gives you a wrong multiplication and forgets your name
two messages later. Building a "GenAI application" means re-adding the layers the chat
product had: system prompts, conversation state, tool wiring, guardrails, retries,
and cost control.

## Mental model to carry through all 11 files

```
                        ┌──────────────────────────────────────────┐
                        │        GenAI Application (software)      │
                        │                                          │
   you ──HTTP──►  ┌─────┴─────┐   ┌──────────┐   ┌─────────────┐   │
                  │  your API │──►│ orchestr-│──►│  RAW LLM    │   │
                  │  (Spring) │   │ ation    │◄──│ (capability)│   │
                  └─────┬─────┘   └────┬─────┘   └─────────────┘   │
                        │              │                           │
                        │         ┌────▼─────┐                     │
                        │         │  tools:  │                     │
                        │         │ search   │                     │
                        │         │ calc     │                     │
                        │         │ code run │                     │
                        │         │ your DB  │                     │
                        │         └──────────┘                     │
                        └──────────────────────────────────────────┘
```

> Aditya's analogy: the engine is not the car. The LLM is the engine. Steering,
> chassis, wheels, brakes — that is the application. These notes are mostly about
> building the rest of the car.

---

### A note on dates and model names

The video demonstrates against OpenAI's `gpt-5.6-luna` and mentions `astra`, `sol`,
and `terra`. Those are real models as of September 2026, but **prices and model names
in this space change monthly** — Luna's price was cut by 80% on 30 July 2026, and
GPT-6 Astra only launched on 3 September 2026. Every price in these notes is a
snapshot. Always re-check the provider's live pricing page before you put a number
in a business case.
