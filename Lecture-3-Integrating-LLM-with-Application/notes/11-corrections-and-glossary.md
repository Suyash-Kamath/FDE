# 11 — Corrections, Glossary, and What to Build Next

## Part A — Where the video is imprecise

None of these make the video wrong in spirit; it's a good introduction. But if you
repeat these in an interview or a design review, someone will push back.

| # | What the video says | More accurate |
|---|---|---|
| 1 | "Going to platform.openai.com lets you talk to the **raw LLM**" | It's the same post-trained, safety-aligned model behind the same API. What you lose is **tools, memory, system-prompt scaffolding, and retrieval** — not "rawness". A truly raw base model would autocomplete your prompt, not answer it. |
| 2 | "The LLM uses the calculator / the LLM calls the tool" | The model **emits a structured request**. Your application executes it and feeds back the result. This distinction is where authorization, validation and idempotency live. |
| 3 | "The raw LLM can't calculate" | Directionally right. More precisely: it *approximates* arithmetic from patterns and is unreliable at scale. It usually gets `2+2`. It fails on 5-digit multiplication, and it fails **silently and confidently**. Reasoning models with a scratchpad do better but still lose to a calculator. |
| 4 | Star counting fails because "it can't calculate" | It fails because of **tokenization**. A run of `****` is one opaque token ID; the character count is destroyed before the model sees anything. This is perception, not arithmetic. Same cause as "how many r's in strawberry". |
| 5 | "The compiler adds the natural language" (~21:10) | Slip of the tongue — the **LLM** wraps the tool output in natural language. The compiler/sandbox returns bare stdout. He says it correctly elsewhere. |
| 6 | "The API key tells the server who you are and how much top-up you have" | Also: which models/endpoints are permitted, which rate-limit buckets apply, and how usage is attributed for billing and audit. And critically — it identifies your **application**, never your end user. |
| 7 | `spring.ai.openai.chat.model=…` | For the OpenAI starter the property is `spring.ai.openai.chat.options.model`. |
| 8 | API key pasted into `application.properties` | Use `${OPENAI_API_KEY}` from the environment, and a secret manager in production. Committed keys are the most common LLM security incident. |
| 9 | "The cutoff date is June 2024" (asked the model directly) | A model's self-reported cutoff is **unreliable** — it's a post-trained belief, not a stored register. Check the model card. |
| 10 | "You're wasting 52 tokens" | "Spending", not "wasting" — and the input/output split matters because output costs ~6× more per token. |
| 11 | `@Autowired` on the field, "to simplify" | Constructor injection: final fields, explicit dependencies, testable without Spring, no NPE window. With one constructor the annotation isn't even needed. |
| 12 | `@RequestBody String ticket` | Use a validated DTO with a size cap. Raw `String` gives you no validation, no evolvability, and no protection against a 2 MB payload. |
| 13 | The summariser returns a `String` | Return structured output. Prose parsed by regex downstream is the #1 source of production LLM bugs. |
| 14 | Synchronous HTTP endpoint for summarisation | Nobody is blocked on a summary. Queue it: free retries, DLQ, backpressure, and eligibility for 50%-off batch pricing. |
| 15 | "Prices: Astra $10/$50, Sol $4/$20, Terra $2/$12, Luna $0.20/$1.20" | Close to correct for September 2026 (Sol is listed at $5/$30 on some pages, $4/$20 as a promotional rate; Luna was cut ~80% on 30 July 2026). **Always re-verify — these move monthly.** |
| 16 | "Free models have small context windows" | True historically; less true now. The real reasons to avoid free tiers for anything serious are **rate limits, sudden deprecation, and data-retention policies** — never send customer data through one. |

### One thing the video gets *very* right

It demonstrates prompt injection live — `What is 2+2?` into a food-delivery
summariser — without naming it, and flags it as a problem to solve. Most
introductory material doesn't surface this until much later. It is the most important
security property of the whole stack.

---

## Part B — Glossary

**Autoregressive decoding** — Generating one token at a time, each conditioned on all
previous tokens. Why output is slow and expensive.

**BPE (Byte-Pair Encoding)** — The tokenizer algorithm. Merges frequent byte pairs
into single vocabulary entries. Cause of the character-counting failure and the
Devanagari cost penalty.

**Context window** — Maximum total tokens (input + output + tools + history) per
request. ~1.05M on current frontier models.

**Decode** — The sequential output phase. Memory-bandwidth-bound.

**Embedding** — A vector representation of text used for semantic similarity search.
The basis of RAG.

**Few-shot prompting** — Including example input/output pairs as fabricated
user/assistant turns.

**Fine-tuning** — Further training on your data to change *style and format*. Not a
way to add facts — use RAG for that.

**Function / tool calling** — The model emits a structured request for a named
function; your code executes it and returns the result.

**Grounding** — Supplying real data in the prompt so the model doesn't invent facts.

**Hallucination** — Fluent, confident, false output. Not a bug to be patched; a
property of next-token prediction. Mitigated by grounding and verification.

**Idempotency key** — A client-supplied ID so a retried write happens once.

**KV cache** — Cached attention keys/values so each new token doesn't reprocess the
whole prompt. The mechanism behind prompt caching.

**Knowledge cutoff** — Date of the training corpus snapshot. Fuzzy, and
self-reported values are unreliable.

**LLM-as-judge** — Using a model to score another model's output against a rubric.
Standard practice in eval pipelines.

**MCP (Model Context Protocol)** — A standard protocol for exposing tools to models,
so tool servers are written once and reused across hosts.

**Prefill** — The parallel phase where your input prompt is processed. Compute-bound,
fast.

**Prompt caching** — Providers cache the KV state of a repeated prompt *prefix*,
billing it at ~10%. Requires a byte-identical prefix.

**Prompt injection** — Attacker-controlled text in the prompt that overrides your
instructions. *Direct* = the user types it. *Indirect* = it arrives via a retrieved
document, web page, or email.

**RAG (Retrieval-Augmented Generation)** — Retrieve relevant chunks from your own
data and inject them into the prompt. The standard fix for stale and private
knowledge.

**Reasoning tokens** — Hidden intermediate tokens generated by reasoning models. You
pay for them and wait for them; you don't see them.

**RLHF** — Reinforcement Learning from Human Feedback. Post-training that turns a
text-completer into an assistant.

**Statelessness** — The API keeps no memory between calls. All continuity is the
client resending the transcript.

**System prompt** — Developer-authored instructions with higher trust than user
messages. A strong tendency, not an enforced boundary.

**Temperature** — Sampling randomness. ~0 for extraction/classification, higher for
creative work.

**Token** — The unit of input, output, billing, and context. ~4 English characters;
~1 Devanagari character.

**TPM / RPM** — Tokens per minute / requests per minute rate limits.

**TTFT / TPOT** — Time to first token / time per output token. Latency is two numbers.

---

## Part C — What to build next

A progression that actually teaches the material, in order:

### 1. Reproduce the failures yourself
Playground vs ChatGPT: the weather question, a 5-digit multiplication, character
counting, "what's my name". Confirm each failure with your own eyes. Add one of your
own: ask both to reverse a 20-character string.

### 2. Bare HTTP, no SDK
Postman then curl then one file of code in your own language. Print the `usage` block
every time. Build the habit of seeing the token count.

### 3. Make it stateful
Add the transcript array by hand. Watch `input_tokens` climb turn over turn. Plot it.
Then add a token-budget trimmer and watch it flatten.

### 4. Add a system prompt and break it
Write the scoped summariser. Then attack your own system prompt — try five injections
and see which get through. Add each defence layer from file 09 §3 and re-test.

### 5. Structured output
Convert the summariser to return a typed object. Notice how much downstream code
disappears.

### 6. One tool
Give it a single tool — `get_order_status(orderId)` against a fake in-memory map.
Implement the loop *by hand* once, without the framework, so you've seen every
message in the exchange. Then let the framework do it.

### 7. A tiny RAG
20 documents, an embedding model, cosine similarity in a list (no vector DB yet).
Then swap in pgvector. You'll understand what the database is buying you.

### 8. Instrument it
Log tokens, latency, finish reason, and cost per call. Build a Grafana panel of
spend by feature. Then find and remove your most expensive prompt.

### 9. Evaluate it
50 labelled examples, a scoring script, and a CI job. Change one word of your prompt
and watch the score move. This is the step that turns prompt-fiddling into
engineering.

### 10. Ship it async
Queue the summariser. Add retries, a DLQ, idempotency, and a cache. Measure cost per
1,000 tickets before and after.

---

## Part D — Further reading

- The provider's own API reference — the single highest-value document. Read the
  Responses/Chat Completions page end to end once.
- Spring AI reference docs (`docs.spring.io/spring-ai/reference`) — `ChatClient`,
  Advisors, Tool Calling, Structured Output, Chat Memory.
- OWASP Top 10 for LLM Applications — prompt injection, insecure output handling,
  excessive agency. Short, practical, directly applicable to file 09.
- The provider's prompt-engineering guide and their pricing page (bookmark the
  latter; check it before every cost estimate).
- Your own production logs. Nothing teaches token economics like your first
  unexpected invoice.

---

*End of series. Back to [`00-README.md`](00-README.md).*
