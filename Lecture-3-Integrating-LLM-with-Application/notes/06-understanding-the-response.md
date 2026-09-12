# 06 — Understanding the LLM API Response

The video's reaction to the response — *"so much stuff… id, object, created, status,
background, billing… but we only care about output → content → text"* — is the right
instinct for a demo and the wrong instinct for production. Everything else in that
envelope is your observability data.

---

## 1. The full envelope, annotated

```jsonc
{
  "id": "resp_68c2f1a9b4d8",        // unique ID. LOG THIS. It's how you ask support
                                    // "what happened on this call?"
  "object": "response",
  "created_at": 1757577600,         // unix seconds
  "status": "completed",            // completed | incomplete | failed | in_progress
  "error": null,
  "incomplete_details": null,       // if status=incomplete, WHY (e.g. max_output_tokens)

  "model": "gpt-5.6-luna-2026-07-09",
  //  ↑ the RESOLVED version. You asked for an alias; this is what actually ran.
  //    Log it. When quality shifts overnight, this field is the evidence.

  "output": [                       // ← an ARRAY of items, not a single message
    {
      "id": "msg_01",
      "type": "message",            // could also be: reasoning | function_call |
                                    // web_search_call | code_interpreter_call …
      "role": "assistant",
      "status": "completed",
      "content": [                  // ← also an ARRAY, of parts
        {
          "type": "output_text",    // could be: refusal | output_audio | …
          "text": "Docker packages applications and their dependencies into lightweight, portable containers…",
          "annotations": []         // citations, file refs, URLs
        }
      ]
    }
  ],

  "output_text": "Docker packages …",   // SDK convenience: all text parts concatenated

  "usage": {
    "input_tokens": 13,
    "input_tokens_details":  { "cached_tokens": 0 },
    "output_tokens": 39,
    "output_tokens_details": { "reasoning_tokens": 0 },
    "total_tokens": 52
  },

  "temperature": 1.0,
  "top_p": 1.0,
  "max_output_tokens": null,
  "tools": [],
  "tool_choice": "auto",
  "parallel_tool_calls": true,
  "service_tier": "default",        // default | flex | priority — affects price & latency
  "store": true,
  "metadata": {}
}
```

Those `13 + 39 = 52` tokens are exactly the numbers from the video's first successful
call. Costing them out is in [`07`](07-tokens-cost-latency-context.md).

---

## 2. Why `output` is an array of items containing an array of parts

This is the design decision that trips everyone up. It exists because a single
assistant turn can legitimately contain several things:

```jsonc
"output": [
  { "type": "reasoning",     "summary": [...] },          // thinking (reasoning models)
  { "type": "web_search_call", "status": "completed" },   // a hosted tool ran
  { "type": "function_call", "name": "get_weather",
    "arguments": "{\"city\":\"Delhi\"}" },                // it wants YOUR tool
  { "type": "message", "role": "assistant",
    "content": [
      { "type": "output_text", "text": "Here's what I found…" },
      { "type": "output_text", "text": " Also…" }
    ] }
]
```

And a content part can be a **refusal** rather than text:

```jsonc
"content": [ { "type": "refusal", "refusal": "I can't help with that." } ]
```

So `output[0].content[0].text` — the path the video reads by hand — is correct for a
plain text answer and **wrong the moment a tool, a reasoning trace, or a refusal
appears**. That's a `NullPointerException` waiting in production.

### Parse by type, never by index

```java
// ✗ fragile
String text = resp.output().get(0).content().get(0).text();

// ✓ robust
String text = resp.output().stream()
    .filter(o -> "message".equals(o.type()))
    .flatMap(m -> m.content().stream())
    .filter(c -> "output_text".equals(c.type()))
    .map(Content::text)
    .collect(Collectors.joining());

List<FunctionCall> toolCalls = resp.output().stream()
    .filter(o -> "function_call".equals(o.type()))
    .map(FunctionCall::from)
    .toList();

Optional<String> refusal = resp.output().stream()
    .filter(o -> "message".equals(o.type()))
    .flatMap(m -> m.content().stream())
    .filter(c -> "refusal".equals(c.type()))
    .map(Content::refusal)
    .findFirst();
```

Spring AI's `.content()` does the text extraction for you — that is one of the things
you're paying the framework for.

---

## 3. The Chat Completions envelope (for comparison)

```jsonc
{
  "id": "chatcmpl-xyz",
  "object": "chat.completion",
  "created": 1757577600,
  "model": "gpt-5.6-luna-2026-07-09",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Docker packages applications…",
        "refusal": null,
        "tool_calls": null
      },
      "logprobs": null,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 13,
    "completion_tokens": 39,
    "total_tokens": 52,
    "prompt_tokens_details":     { "cached_tokens": 0 },
    "completion_tokens_details": { "reasoning_tokens": 0 }
  },
  "system_fingerprint": "fp_a1b2c3"
}
```

Note the vocabulary shift: `prompt_tokens`/`completion_tokens` here vs
`input_tokens`/`output_tokens` in Responses. Same concept.

`choices` is an array because of the `n` parameter (ask for k samples). You almost
always use `n=1`, but the array is why the shape is `choices[0]`.

---

## 4. `finish_reason` — the field you must branch on

| Value | Meaning | What to do |
|---|---|---|
| `stop` | Model finished naturally | Normal path |
| `length` | **Hit `max_tokens` and was cut off mid-sentence** | Your output is truncated. Retry with a bigger cap, or design around it. Do NOT parse it as JSON — it will be invalid. |
| `tool_calls` | Model wants a tool | Run the loop (file 03) |
| `content_filter` | Blocked by safety | Show a graceful message, log for review |
| `function_call` | Legacy alias | Treat as `tool_calls` |

In Responses this appears as `status: "incomplete"` plus
`incomplete_details.reason: "max_output_tokens"`.

**The truncation bug is the classic one.** You set `max_output_tokens: 150`, ask for
JSON, the model needs 170, you get:

```json
{"summary":"Customer received wrong items after a 50 minute wait and is req
```

Your `objectMapper.readValue()` throws, and the stack trace blames Jackson rather
than your token cap. Always check `finish_reason` before parsing.

---

## 5. Streaming

Non-streaming: you wait for the *entire* generation, then get one JSON blob. For a
400-token answer that's several seconds of a blank screen.

Streaming (`"stream": true`) returns **Server-Sent Events**:

```
event: response.output_text.delta
data: {"delta":"Docker"}

event: response.output_text.delta
data: {"delta":" packages"}

event: response.output_text.delta
data: {"delta":" applications"}

…

event: response.completed
data: {"response":{"usage":{"input_tokens":13,"output_tokens":39}, …}}
```

Key points:

- **Total time is the same. Perceived time is far better.** The user sees the first
  word in ~300 ms instead of the whole answer in 4 s.
- `usage` typically arrives only in the **final** event. If you need token accounting,
  capture it there — don't forget and then wonder why your cost dashboard is empty.
- Streaming complicates your architecture: you must stream through *your* backend to
  the client too (SSE or WebSocket all the way through), handle mid-stream errors
  (you've already sent a 200 and half an answer), and you cannot validate the full
  output before the user sees the start of it.

**When not to stream:** batch jobs, machine-consumed output (structured JSON you're
going to parse anyway), and anywhere you need to validate/moderate before display.
The support-ticket summariser in file 09 is a good example of a *don't stream* case —
nobody is reading it live.

---

## 6. What to log on every call

Build this into your client wrapper from day one:

```java
log.info("llm.call id={} model={} status={} finish={} in={} cached={} out={} reasoning={} ms={} cost_usd={} feature={} tenant={}",
    resp.id(), resp.model(), resp.status(), finishReason,
    u.inputTokens(), u.cachedTokens(), u.outputTokens(), u.reasoningTokens(),
    elapsedMs, estimatedCost, feature, tenantId);
```

Metrics worth emitting:

| Metric | Why |
|---|---|
| `llm_tokens_total{direction,model,feature}` | Cost attribution per feature |
| `llm_latency_seconds` (p50/p95/p99) | SLOs; p99 is where LLMs hurt |
| `llm_time_to_first_token_seconds` | The number users actually feel |
| `llm_errors_total{code}` | 429 vs 500 vs quota |
| `llm_finish_reason_total{reason}` | A rising `length` rate = your cap is too low |
| `llm_cache_hit_ratio` | Prompt caching effectiveness |
| `llm_tool_calls_total{tool}` | Which tools the model actually reaches for |

Spend attribution by feature is the one people add late and regret not having. Tag
requests with `metadata` at the call site.

---

## 7. Checkpoint questions

1. Why is `output[0].content[0].text` unsafe?
2. Your JSON parser throws intermittently on ~3% of calls. Which response field do you
   check first?
3. You enabled streaming and your cost dashboard went to zero. What did you miss?
4. What's the difference between `status: "incomplete"` and `finish_reason: "length"`?
5. Why log the resolved `model` field when you already know what you requested?

→ Next: [`07-tokens-cost-latency-context.md`](07-tokens-cost-latency-context.md)
