# 07 — Tokens: Cost, Latency, and Context

The video's claim — *"LLMs charge you per token, input plus output, added together"* —
is correct and is the single most load-bearing fact in LLM engineering. This file
takes it as far as it goes.

---

## 1. What a token is

A token is a unit in the model's vocabulary (typically 50k–200k entries), produced by
a **Byte-Pair Encoding** tokenizer. BPE starts from bytes and repeatedly merges the
most frequent adjacent pair until the vocabulary is full. The result: common words
are one token, rare words split into pieces.

```
"Hello world"            → ["Hello", " world"]                        2 tokens
"unbelievable"           → ["un", "bel", "iev", "able"]               4 tokens
"Suyash"                 → ["Su", "y", "ash"]                         3 tokens
"87345"                  → ["873", "45"]                              2 tokens
"{\"status\": \"ok\"}"   → ["{\"", "status", "\":", " \"", "ok", "\"}"] ~6 tokens
```

Note the leading space belongs to the token. `" world"` and `"world"` are different
IDs. This is why prompt formatting subtly changes token counts.

### Rough conversion rates

| Language / content | Chars per token | Note |
|---|---|---|
| English prose | ~4 | ~0.75 words per token |
| Code | ~3 | punctuation is expensive |
| JSON | ~2.5–3 | braces, quotes, colons all cost |
| **Hindi / Devanagari** | **~1–1.5** | see below |
| Other Indic scripts | ~1–1.5 | same problem |
| Emoji | 1–4 tokens each | often multi-byte |

### The Devanagari tax — matters for Indian products

Tokenizers are trained predominantly on English text, so Devanagari characters often
don't merge into efficient tokens; many become multiple tokens *per character* via
UTF-8 byte fallback.

```
"How can I cancel my order?"            ≈  7 tokens
"मैं अपना ऑर्डर कैसे रद्द करूं?"                  ≈ 25–40 tokens
```

Same meaning, **3–5× the tokens**, therefore 3–5× the cost, a faster-filling context
window, and higher latency. If you're building for Indian users, budget for this
explicitly and measure it with a real tokenizer rather than assuming.

Mitigations: pick models with better multilingual tokenizers (test, don't assume);
consider translating to English for internal reasoning steps where fidelity allows;
keep system prompts in English even when output is in Hindi.

### Counting them properly

Never estimate in production. Use the real tokenizer:

```python
# pip install tiktoken
import tiktoken
enc = tiktoken.get_encoding("o200k_base")
print(len(enc.encode("Explain me Docker in two lines")))
```

```xml
<!-- Java: jtokkit -->
<dependency>
  <groupId>com.knuddels</groupId><artifactId>jtokkit</artifactId>
</dependency>
```

```go
// Go: github.com/pkoukk/tiktoken-go
enc, _ := tiktoken.GetEncoding("o200k_base")
n := len(enc.Encode(text, nil, nil))
```

The authoritative count is the `usage` block in the response. Local tokenizers are for
*pre-flight* checks ("will this fit?", "should I truncate?").

---

## 2. Input vs output tokens, and why output costs more

Across every provider, output is priced **5–10× input**. Not arbitrary — it reflects
the physics of inference.

```
PREFILL (your input)                    DECODE (the output)
────────────────────                    ───────────────────
All input tokens processed              One token at a time, sequentially.
in PARALLEL in one forward pass.        Each token = a full forward pass
Compute-bound, GPU runs hot,            over all weights, reading the whole
excellent hardware utilisation.         model from HBM every step.
                                        MEMORY-BANDWIDTH-bound. Terrible
1000 input tokens ≈ 1 pass              utilisation. Hard to batch well.

                                        1000 output tokens ≈ 1000 passes
```

Consequences beyond price:

- **Output tokens dominate latency**, not input. A 100-token prompt with a 2000-token
  answer is far slower than a 2000-token prompt with a 100-token answer.
- **"Be concise" is a performance optimisation**, not just a style preference.
- **Reasoning models** generate hidden reasoning tokens that you pay for as output and
  wait for, even though you never see them. Watch
  `output_tokens_details.reasoning_tokens`.

---

## 3. Costing the video's actual call

From the response: `input_tokens: 13`, `output_tokens: 39`, `total: 52`.
At GPT-5.6 Luna rates ($0.20 / $1.20 per 1M):

```
input :  13 / 1,000,000 × $0.20  = $0.0000026
output:  39 / 1,000,000 × $1.20  = $0.0000468
─────────────────────────────────────────────
total                            = $0.0000494   ≈ ₹0.0044  (less than half a paisa)
```

So $5 of credit buys roughly **100,000 calls** of this size. Aditya's "your $5 will
last a long time" is well justified.

### But now scale it — the ticket summariser from file 09

| | value |
|---|---|
| Tickets per day | 10,000 |
| System prompt | 250 tokens |
| Ticket text | ~400 tokens |
| Summary out | ~60 tokens |

```
per call:  input 650 × $0.20/1M  = $0.000130
           output 60 × $1.20/1M  = $0.000072
                                   ─────────
                                   $0.000202

per day:   10,000 × $0.000202     = $2.02
per month:                        ≈ $61        (~₹5,400)
```

Comfortable. Now change **one** variable — someone swaps the model to the flagship
because "quality":

```
Astra @ $10/$50:  650×$10/1M + 60×$50/1M = $0.0065 + $0.0030 = $0.0095/call
per month: 10,000 × 30 × $0.0095         = $2,850   (~₹2.5 lakh)
```

**47× the bill for the same feature.** This is why model choice is an architectural
decision with a finance owner, and why you tag requests by feature so you can see
which one moved.

---

## 4. The context window

The context window is the **maximum total tokens** — input + output + tools +
history — for a single request. Current frontier models are around 1M tokens; the
GPT-5.6 family ships ~1.05M across its tiers.

```
┌──────────────────── context window (e.g. 1,050,000) ───────────────────┐
│ system │ tool schemas │ conversation history │ RAG chunks │ user │ OUT │
└────────────────────────────────────────────────────────────────────────┘
                                                        ↑
                                         max_output_tokens is reserved from here
```

Exceed it and you get a 400. But long before that, three things bite:

1. **Cost.** Input is cheap per token but a 200k-token prompt is 200k tokens *every
   turn*.
2. **Latency.** Prefill is fast but not free; time-to-first-token grows with prompt
   length.
3. **Long-context repricing.** Past a threshold (~272K tokens for current OpenAI
   tiers) the *entire* request reprices at roughly 2× input and 1.5× output — not just
   the overflow. A prompt that creeps from 270k to 275k tokens more than doubles in
   cost.
4. **"Lost in the middle."** Models attend best to the beginning and end of a long
   context. Material buried in the middle of a 500k-token prompt is measurably less
   likely to be used. A big window is not a reason to stop being selective — put the
   important instructions at the start and the critical data at the end.

> **A large context window is a capacity, not a strategy.** RAG (retrieve the 5
> relevant chunks) usually beats "paste all 400 pages" on cost, latency *and*
> accuracy.

---

## 5. The quadratic cost of conversation

This is the consequence of statelessness (file 10) that surprises everyone.

Every turn resends the whole transcript. If each turn adds ~200 tokens:

| Turn | Input tokens sent | Cumulative input |
|---|---|---|
| 1 | 200 | 200 |
| 2 | 400 | 600 |
| 3 | 600 | 1,200 |
| 5 | 1,000 | 3,000 |
| 10 | 2,000 | 11,000 |
| 20 | 4,000 | 42,000 |
| 50 | 10,000 | 255,000 |

Total input tokens over an *n*-turn chat grows as **O(n²)**. A 50-turn conversation
costs ~25× a 10-turn one, not 5×.

### Mitigations

| Strategy | How | Trade-off |
|---|---|---|
| **Sliding window** | Keep last *k* turns | Drops early context abruptly |
| **Summarisation buffer** | Summarise old turns into a running précis | Costs an extra call; lossy |
| **Token budget** | Trim oldest until under N tokens | Simple and effective; do this first |
| **Vector memory** | Embed old turns, retrieve relevant ones | Complex; good for long-lived assistants |
| **Prompt caching** | Keep the stable prefix byte-identical | Huge win, see below |
| **Structured state** | Extract facts to a DB, inject only what's needed | Best for task-oriented bots |

---

## 6. The four pricing levers

### (a) Prompt caching — ~90% off repeated prefixes
Providers cache the KV state of a prompt prefix. Cached input bills at roughly **10%**
of the standard rate (with a cache-*write* surcharge around 1.25× on first use for
current OpenAI models).

To benefit, the prefix must be **byte-identical** across calls. Therefore:

```
┌─────────────────────────────┐
│ system prompt      (stable) │  ← cacheable
│ tool schemas       (stable) │  ← cacheable
│ few-shot examples  (stable) │  ← cacheable
├─────────────────────────────┤
│ conversation history        │  ← partially cacheable if append-only
│ THIS request's user text    │  ← never cacheable
└─────────────────────────────┘
```

**Put everything variable at the end.** Injecting a timestamp or a request ID at the
top of your system prompt silently destroys your cache hit rate. This is a real and
common own-goal.

### (b) Batch API — 50% off
Submit a file of requests, get results asynchronously (typically within 24 h). Perfect
for: nightly summarisation, backfills, evals, bulk classification. Useless for
interactive features.

### (c) Service tiers
"Flex"/batch halve the rate with worse latency; "Fast"/priority double it for better
latency. The *same model* can have four prices depending on urgency. Match the tier
to the use case rather than paying interactive rates for a cron job.

### (d) Model tier
Covered in file 04. The biggest lever by far — see the 47× example above.

---

## 7. Latency engineering

Two numbers, not one:

- **TTFT** (time to first token) — dominated by queueing, prompt length, prefill.
  This is what the user feels as "responsiveness."
- **TPOT** (time per output token) — dominated by model size. Determines how fast the
  answer appears once it starts.

```
total ≈ TTFT + (output_tokens × TPOT)
```

Levers, in rough order of effect:

1. **Smaller model.** Often 3–5× faster.
2. **Fewer output tokens.** Cap it, and instruct for brevity.
3. **Stream.** Doesn't reduce total time; cuts perceived time dramatically.
4. **Shorten the prompt.** Reduces prefill and TTFT.
5. **Prompt caching.** Cache hits skip prefill for the cached prefix.
6. **Parallelise.** Independent sub-tasks concurrently rather than in one long chain.
7. **Cache results.** Identical input → identical output. A plain Redis cache keyed on
   a hash of (model, system prompt, user input) is free money for repeated queries.

Set a **latency budget** per feature and enforce a timeout. And always have a
degraded path for when you blow it.

---

## 8. Checkpoint questions

1. Why is output priced ~6× input? Give the hardware reason, not the business one.
2. Your Hindi-language chatbot costs 4× your English one at the same traffic. Why?
3. A 50-turn conversation costs 25× a 10-turn one. Show why.
4. Someone adds `"Current time: {{now}}"` to the top of the system prompt. What did
   they just break?
5. Your p99 latency is 12 s. List four things you'd try, in order.
6. Your prompt is 271,000 tokens and a PM asks to add "just one more document". What
   do you tell them?

→ Next: [`08-spring-ai-and-chatclient.md`](08-spring-ai-and-chatclient.md)
