# 01 — ChatGPT is not an LLM

## 1. The misconception, stated precisely

> "ChatGPT is an LLM." — wrong.

**ChatGPT** is a *product*: a web/mobile frontend, an authentication system, a
backend, a conversation store, a tool layer, safety filters, billing, and
somewhere deep inside it, a model.

**GPT-5.6 Luna** is a *model*: a file of weights plus the code to run a forward pass.

Confusing the two is like confusing "Gmail" with "SMTP". One is a product; the other
is the primitive it's built on.

The sentence to memorise, from the video:

> **A raw LLM is a capability. A GenAI application is the software built around that capability.**

---

## 2. What a "raw LLM" actually is

Strip away everything and an LLM is a pure function:

```
f(sequence_of_token_ids) -> probability_distribution_over_the_vocabulary
```

That's it. One call gives you *one* probability distribution for the *next* token.

To produce a sentence, the runtime runs this function in a loop — this is called
**autoregressive decoding**:

```
prompt:  ["Hi", " Chat", "GPT", ",", " my", " name", " is", " Aditya"]

step 1:  f(prompt)                      -> most likely next token: " Hi"
step 2:  f(prompt + [" Hi"])            -> " Aditya"
step 3:  f(prompt + [" Hi", " Aditya"]) -> "!"
step 4:  f(...)                         -> " Nice"
step 5:  f(...)                         -> " to"
...
step n:  f(...)                         -> <end_of_turn>   ← loop stops
```

Aditya walks through exactly this in the video. Three things follow from it that
explain almost every strange LLM behaviour:

1. **The model has no memory between calls.** Everything it "knows about the
   conversation" is re-fed as input tokens every single step. Nothing persists.
2. **The model has no ability to act.** The only thing it emits is a token. It cannot
   open a socket, read a file, or run a program.
3. **The model is stochastic.** It samples from a distribution. Same input can give
   different output (unless temperature is 0 and even then, not guaranteed bit-exact).

### What "raw" does and doesn't mean

Careful here — the video is slightly loose. When you hit
`platform.openai.com` (the Playground) or the HTTP API, you are **not** talking to a
raw transformer either. You are talking to:

- a **post-trained** model (supervised fine-tuning + RLHF/RLAIF), which is why it
  behaves like a helpful assistant instead of autocompleting your prompt into spam;
- behind a **serving stack** that does tokenization, batching, KV caching, sampling;
- usually behind **moderation/safety classifiers** on input and output.

What you *lose* versus the ChatGPT product is not "safety" — it's **tools, memory,
system-prompt scaffolding, and retrieval**. That is the honest version of the
distinction. Say "the model without the application", not "the raw transformer".

---

## 3. The full request path, layer by layer

When you type into ChatGPT:

```
┌────────────┐
│  BROWSER   │  you type "What's the weather in Delhi right now?"
└─────┬──────┘
      │ HTTPS POST (session cookie, not an API key)
      ▼
┌────────────────────────────────────────────────────────────┐
│  CHATGPT BACKEND                                           │
│                                                            │
│  1. authn/authz    — who are you, which plan, rate limits  │
│  2. conversation   — load prior turns from the DB          │
│  3. system prompt  — inject product instructions, date,    │
│                      user custom instructions, memories    │
│  4. tool catalogue — attach web_search, python, image gen, │
│                      connectors, file_search …             │
│  5. input moderation — safety classifiers                  │
│  6. tokenize       — text → token IDs                      │
│                                                            │
│        ┌──────────────────────────────────────┐            │
│        │   MODEL (the actual weights)         │            │
│        │   autoregressive decode loop         │            │
│        └──────────────────────────────────────┘            │
│                          │                                 │
│  7. if the model emitted a tool call → run the tool,       │
│     append the result, and GO BACK to the model            │
│  8. output moderation                                      │
│  9. detokenize, stream tokens back as SSE                  │
│ 10. persist the turn to the conversation store             │
│ 11. meter usage for billing                                │
└─────┬──────────────────────────────────────────────────────┘
      ▼
┌────────────┐
│  BROWSER   │  renders the streamed answer
└────────────┘
```

Steps 2, 3, 4, 7 and 10 are the ones you **do not get** when you call the API
yourself. Building them is the job.

---

## 4. The four demos in the video, and what each one proves

| # | Demo | ChatGPT app | Playground / API | What it proves |
|---|------|-------------|------------------|----------------|
| 1 | "Weather of Delhi right now?" | "Searching the web…" → 31 °C | "I can't access live weather data" | The model has **no live data**; ChatGPT has a **search tool** |
| 2 | "What is your cutoff date?" | — | "June 2024" | Knowledge is **frozen at training time** |
| 3 | `87345 × 5623` | "Calculating the product…" → correct | wrong number, confidently | The model **cannot compute**; ChatGPT has a **code/calculator tool** |
| 4 | "Count the stars: `****…`" (32 of them) | "Counting stars in a string" → 32 | 16 — wrong | The model **cannot count characters**; ChatGPT **writes and runs code** |

Notice the tell in the ChatGPT UI both times: *"Searching the web"*, *"Calculating
the product of two integers"*, *"Counting stars in a string"*. Those status lines are
the orchestrator telling you a tool ran. When you see one, a raw model was not the
thing that produced the fact.

Demo 4 has a deeper cause than the video gives — see [`02`](02-why-llms-need-tools.md) §3.

---

## 5. Two ways to reach "the model without the application"

The video names both:

**(a) The Playground** — `platform.openai.com/chat`.
A developer UI over the same API. You choose the model, set temperature, top-p,
max tokens, system prompt. No web search, no code interpreter, no memory across
sessions unless you enable them. This is the cheapest way to *see* raw behaviour.

**(b) The API, with a key** — from Postman, curl, or your own application.
`POST https://api.openai.com/v1/responses` with `Authorization: Bearer sk-…`.
This is what you'll actually ship. Covered in [`04`](04-endpoints-auth-keys-models.md)
and [`05`](05-first-request-postman.md).

Once you're on (b), *you* are the backend in the diagram above. Everything between
step 2 and step 10 is now your code.

---

## 6. Why this framing matters for an FDE / backend engineer

Nearly every "the AI is stupid" bug report in production turns out to be a missing
layer, not a bad model:

| Symptom | Actual cause | Layer to add |
|---|---|---|
| "It forgot what I told it" | Stateless API, you didn't resend history | Conversation state (file 10) |
| "It made up a customer's order ID" | No grounding data in the prompt | RAG / tool call (file 03) |
| "It gave the wrong total" | Arithmetic in the model | Calculator / code tool (file 02) |
| "It answered an off-topic question" | No system prompt scoping it | System role (file 10) |
| "Someone made it leak the prompt" | User text concatenated into instructions | Prompt injection defence (file 09) |
| "Our bill exploded" | Full history resent every turn | Token budget (file 07) |
| "It's slow" | Big model, long prompt, no streaming | Model + latency tuning (file 07) |

The model is rarely the variable you should reach for first.

---

## 7. Checkpoint questions

1. Why can two identical API requests return different text?
2. Your colleague says "we'll just fine-tune so it knows today's stock price." Why is
   that the wrong tool?
3. ChatGPT answers a question about an event from last week. Name the three layers
   that had to cooperate for that to happen.
4. What exactly is lost when you move from the ChatGPT UI to `POST /v1/responses`?

→ Next: [`02-why-llms-need-tools.md`](02-why-llms-need-tools.md)
