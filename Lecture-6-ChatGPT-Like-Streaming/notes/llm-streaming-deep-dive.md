# LLM Streaming: A Deep Dive

**From token generation to a live ChatGPT-style UI — language independent, with Python, Go, Java and JavaScript code.**

---

Let me build this from the bottom up, because the whole topic only clicks once you separate three layers that people usually mash together: **how the model generates**, **how the provider delivers**, and **how your app re-delivers**.

---

## 1. How an LLM actually produces text

An LLM is an autoregressive next-token predictor. Given a sequence of tokens, it outputs a probability distribution over its entire vocabulary for the *next* token, samples one, appends it, and repeats.

```
input:  ["Capital", " of", " India", " is"]
step 1: → distribution over ~100k tokens → picks " New"
step 2: input becomes [... " is", " New"] → picks " Delhi"
step 3: → picks "."
step 4: → picks <|endoftext|>  (stop)
```

There is no mode where the model "writes the whole paragraph at once." Generation is inherently incremental. That's the single fact that makes streaming possible at all.

### Prefill vs decode (this explains TTFT)

Inference has two distinct phases with very different cost profiles:

**Prefill** — the model reads your entire prompt, tokenizes it, embeds it, and runs all of it through the transformer in one parallel forward pass, building a KV cache (the cached key/value attention tensors for every layer, for every input token). Cost scales with input length. This is compute-bound.

**Decode** — now it generates one token at a time. Each step is a forward pass over *one* token that attends to the cached KV of everything before it. Cost per token is roughly constant and is memory-bandwidth-bound (you're streaming the model weights from HBM each step).

So:

```
|<---- prefill (long prompt = slow) ---->|<- t1 -><- t2 -><- t3 -> ...
                                          ^
                                          first token emitted here
```

**Time To First Token (TTFT)** ≈ queue wait + prefill + one decode step.
**Inter-Token Latency (ITL)** ≈ one decode step, roughly steady.
**Total generation time** ≈ TTFT + (output_tokens − 1) × ITL.

This is why a 50-page prompt has terrible TTFT but normal tokens-per-second afterwards, and why reasoning models have huge TTFT (they burn thinking tokens before the first visible one).

---

## 2. Generation vs delivery

Two independent things:

| | |
|---|---|
| **Generation** | Always token by token. Property of the model. You cannot change it. |
| **Delivery** | How the bytes reach you. Buffered (one blob at the end) or streamed (incrementally). **This is a choice.** |

Non-streaming request:

```
t=0s   you send prompt
       provider forwards to model
       model decodes... decodes... decodes...   (you see nothing)
t=8s   provider serializes the full completion and sends one JSON response
```

Streaming request:

```
t=0.4s  "The"
t=0.45s " capital"
t=0.50s " of"
...
t=8.0s  done
```

**Total time is identical.** 8 seconds is 8 seconds. What changes is *perceived* latency: the user starts reading at 0.4s instead of staring at a spinner for 8s. Human reading speed is roughly 250 wpm; models often generate faster than that, so once the first token lands the user never catches up to the generator and it feels instant.

Two honest caveats the video doesn't mention:

1. Streaming can be genuinely faster end to end if you act on partial output (start rendering markdown, start a downstream call, let the user hit stop).
2. Non-streaming long generations often die on gateway/proxy idle timeouts (60s on many load balancers). Streaming keeps bytes flowing so the connection never looks idle. This is a real production reason, not just UX.

---

## 3. When does the model stop?

Four independent stop conditions:

1. **EOS / stop token.** During pretraining, documents are separated by a special end-of-text token. The model learns that after a natural conclusion, the highest-probability next token is that one. When it samples it, decoding halts. This is the "natural" stop.
2. **`max_tokens`** — a hard cap you set. Output gets truncated mid-sentence; `finish_reason` comes back as `"length"` instead of `"stop"`.
3. **Stop sequences** — you pass `stop=["\nUser:"]` and the runtime cuts generation when that string appears.
4. **Context window exhaustion** — input + output must fit in the window.

Note: "write 1000 words" is *not* a stop condition. The model has no reliable counter; it approximates from training-data intuitions about document length. That's why you get 780 or 1300 words.

Always check `finish_reason`. A truncated answer that you saved to history as if complete is a classic silent bug.

---

## 4. Tokens ≠ chunks ≠ packets

Three different granularities, and conflating them causes bugs:

**Token** — the model's unit (`" Delhi"`, `"ization"`).

**SSE chunk / delta** — what the provider emits as one event. Usually one token, but providers batch under load, and some (Anthropic) emit typed events where a single text delta may cover several tokens. Never assume `1 chunk == 1 token`.

**TCP segment / HTTP chunk** — what actually arrives at your socket. One `read()` may give you half an SSE event, or two and a half events. This is the bug that bites everyone:

```
read() → 'data: {"delta":"New Del'
read() → 'hi"}\n\ndata: {"delta":"."}\n\n'
```

You **must** buffer and split on the SSE frame delimiter (`\n\n`) rather than parsing each network read as a complete message. More on this in the frontend section.

---

## 5. The provider API: `call` vs `stream`

Spring AI's `.call()` vs `.stream()` is just a `stream: true` flag on the HTTP request body. Every SDK is a wrapper over the same thing.

**Raw HTTP, non-streaming:**

```json
POST /v1/chat/completions
{"model": "gpt-4o-mini", "messages": [...], "stream": false}
```

Response: `Content-Type: application/json`, one body, arrives at the end.

**Raw HTTP, streaming:**

```json
{"model": "gpt-4o-mini", "messages": [...], "stream": true}
```

Response: `Content-Type: text/event-stream`, `Transfer-Encoding: chunked`, and the body dribbles out as:

```
data: {"choices":[{"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":"New"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":" Delhi"},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]

```

Note the structure: each event is `data: ` + JSON + **two** newlines. Each delta carries only the *new* text, not the accumulated text. You concatenate.

### Python (OpenAI-style)

```python
from openai import OpenAI

client = OpenAI()

stream = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Capital of India is"}],
    stream=True,
)

for event in stream:
    delta = event.choices[0].delta.content or ""   # can be None on first/last
    print(delta, end="", flush=True)
```

The `or ""` matters: the first event has `delta.role` but no content, and the last has neither.

### Python (Anthropic-style, typed events)

```python
with client.messages.stream(
    model="claude-sonnet-4-6",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Capital of India is"}],
) as stream:
    for text in stream.text_stream:
        print(text, end="", flush=True)
    final = stream.get_final_message()   # accumulated, with usage + stop_reason
```

Anthropic's wire format is richer: `message_start`, `content_block_start`, `content_block_delta`, `content_block_stop`, `message_delta` (carries `stop_reason` and output token count), `message_stop`. More work to parse by hand, but you get structured events for tool calls and thinking blocks.

---

## 6. What `Flux<String>` really is

Java returns `Flux<String>` because a stream is not a value, it's a *sequence arriving over time*. You cannot type it as `String`, because at the moment the function returns, the string doesn't exist yet.

Every language has this concept:

| Language | Type | Nature |
|---|---|---|
| Java (Reactor) | `Flux<String>` | push-based, reactive, backpressure-aware |
| Python | `AsyncIterator[str]` / async generator | pull-based |
| JavaScript | `ReadableStream` / `AsyncIterable` | pull-based with backpressure |
| Go | `<-chan string` | pull-based, blocking, buffered by channel capacity |
| Rust | `Stream<Item = String>` | poll-based |

The shared idea: **a lazy, time-extended sequence with a terminal signal** (complete or error).

Python's version:

```python
from typing import AsyncIterator

async def chat_stream(prompt: str) -> AsyncIterator[str]:
    stream = await client.chat.completions.create(
        model="gpt-4o-mini",
        messages=[{"role": "user", "content": prompt}],
        stream=True,
    )
    async for event in stream:
        if delta := (event.choices[0].delta.content or ""):
            yield delta
```

The function returns immediately. Nothing runs until someone iterates it. `Flux` differs in being push-based with explicit backpressure requests, but for your purposes the mental model is the same.

---

## 7. The transport layer: SSE vs WebSockets vs chunked HTTP

### What's actually underneath

HTTP/1.1 has a framing mechanism called **chunked transfer encoding**. When the server doesn't know the body length upfront, it sends:

```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Transfer-Encoding: chunked

1f
data: {"delta":"New"}\n\n
22
data: {"delta":" Delhi"}\n\n
0

```

Each chunk is prefixed with its hex length; a zero-length chunk ends the body. **SSE is a text protocol layered on top of this.** HTTP/2 and HTTP/3 use DATA frames instead, same idea.

### SSE format spec

```
data: some text
data: continued on next line
event: custom_event_name
id: 42
retry: 3000

```

Rules that matter:

- Fields are `field: value`, terminated by `\n`.
- A **blank line** (`\n\n`) dispatches the event.
- Multiple `data:` lines in one event are joined with `\n`.
- **Your JSON payload must not contain raw newlines**, or you'll split one event into two. Use compact JSON (`json.dumps(obj, separators=(",", ":"))`).
- Lines starting with `:` are comments, commonly used as keepalive pings (`: ping\n\n`) to stop intermediaries from closing an idle connection.

### Why SSE and not WebSockets

| | SSE | WebSocket |
|---|---|---|
| Direction | Server → client only, after a client request | Full duplex |
| Protocol | Plain HTTP | HTTP Upgrade handshake, then `ws://` framing |
| Infrastructure | Works through every proxy, CDN, LB that speaks HTTP | Needs explicit upgrade support |
| Auth | Normal headers, cookies, bearer tokens | Handshake-only headers, awkward |
| Reconnect | Built-in with `Last-Event-ID` | You implement it |
| Compression, HTTP/2 mux | Free | Separate mechanisms |

The chat interaction shape is: **client asks once, server answers at length**. That's request/response with a long response. WebSockets buy you server-initiated messages, and ChatGPT never messages you first. Paying the operational cost of a stateful bidirectional protocol for a capability you don't use is the definition of overkill.

Where WebSockets *do* win: realtime voice (audio in and out simultaneously), collaborative editing, live interruption of generation mid-stream with low latency. OpenAI's Realtime API uses WebSockets/WebRTC precisely because audio is bidirectional.

There's also a third option worth knowing: **plain chunked HTTP with newline-delimited JSON (NDJSON)**, no `data:` prefix. Simpler, and fine if you control both ends. SSE's advantage is the standardized event/id/retry semantics.

---

## 8. Your backend: re-streaming to your client

Here's the trap. Two streams exist:

```
Provider ──stream 1──▶ Your backend ──stream 2──▶ Browser
```

If you get stream 1 right but collect it into a string before sending stream 2, your user's experience is identical to no streaming at all. You've streamed into a bucket.

### Python / FastAPI

```python
import json
from fastapi import FastAPI, Request
from fastapi.responses import StreamingResponse
from openai import AsyncOpenAI

app = FastAPI()
client = AsyncOpenAI()

SYSTEM = "You are a funny AI chatbot. You reply to everything sarcastically."

@app.post("/api/chat")
async def chat(req: Request):
    body = await req.json()
    user_msg = body["message"]
    history.append({"role": "user", "content": user_msg})

    async def event_stream():
        buffer = []                       # the accumulator
        try:
            stream = await client.chat.completions.create(
                model="gpt-4o-mini",
                messages=[{"role": "system", "content": SYSTEM}, *history],
                stream=True,
            )
            async for event in stream:
                if await req.is_disconnected():   # user closed the tab
                    break
                delta = event.choices[0].delta.content or ""
                if delta:
                    buffer.append(delta)
                    yield f"data: {json.dumps({'delta': delta}, separators=(',',':'))}\n\n"
        except Exception as e:
            # headers already sent, so you CANNOT change the status code.
            # you must report errors in-band:
            yield f"data: {json.dumps({'error': str(e)})}\n\n"
        finally:
            if buffer:
                history.append({"role": "assistant", "content": "".join(buffer)})
            yield "data: [DONE]\n\n"

    return StreamingResponse(
        event_stream(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",   # tells nginx not to buffer
        },
    )
```

### Go

```go
func chatHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("X-Accel-Buffering", "no")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming unsupported", http.StatusInternalServerError)
        return
    }

    ctx := r.Context()
    var full strings.Builder

    stream := provider.StreamCompletion(ctx, req)   // returns <-chan Delta
    defer stream.Close()

    for {
        select {
        case <-ctx.Done():                  // client disconnected
            savePartial(full.String())
            return
        case d, open := <-stream.C:
            if !open {
                fmt.Fprint(w, "data: [DONE]\n\n")
                flusher.Flush()
                history.Append("assistant", full.String())
                return
            }
            full.WriteString(d.Text)
            b, _ := json.Marshal(map[string]string{"delta": d.Text})
            fmt.Fprintf(w, "data: %s\n\n", b)
            flusher.Flush()               // ← without this, nothing moves
        }
    }
}
```

**`flusher.Flush()` is the whole game in Go.** `net/http` buffers writes; without an explicit flush your handler streams internally and delivers one blob. Same class of mistake as `await response.text()` on the frontend.

Things that silently break streaming in production, in order of how often they bite people:

1. **nginx `proxy_buffering on`** (the default). Set `proxy_buffering off;` for the route, or send `X-Accel-Buffering: no`.
2. **Compression middleware.** gzip buffers to get a decent compression window. Exclude `text/event-stream`.
3. **Missing flush** (Go, Node with some frameworks, PHP).
4. **CDN/ALB response buffering.** CloudFront needs the right cache policy; some API gateways buffer unconditionally.
5. **Error handling.** You sent `200 OK` on the first byte. A failure at token 400 cannot become a 500. Design an in-band error event and have the client render it.

---

## 9. The frontend: why `response.text()` kills it

```js
const res = await fetch(url, { method: "POST", body: ... });
const text = await res.text();     // ← the culprit
```

`res.text()` returns a Promise that resolves only when the body reaches EOF. The bytes *did* arrive incrementally; you just told the browser "wake me when it's all here." Same for `res.json()`.

### The correct version

```js
const res = await fetch(API_URL, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ message: userMessage }),
});
if (!res.ok) throw new Error(`Request failed: ${res.status}`);

const reader  = res.body.getReader();     // ReadableStreamDefaultReader
const decoder = new TextDecoder("utf-8");
let sseBuffer = "";
let bubble = null;

while (true) {
  const { value, done } = await reader.read();
  if (done) break;

  // stream:true keeps state for multi-byte chars split across chunks
  sseBuffer += decoder.decode(value, { stream: true });

  // an SSE event ends with a blank line; the tail may be a partial event
  const frames = sseBuffer.split("\n\n");
  sseBuffer = frames.pop();               // keep the incomplete remainder

  for (const frame of frames) {
    if (!frame.startsWith("data:")) continue;
    const payload = frame.slice(5).trim();
    if (payload === "[DONE]") { reader.cancel(); return; }

    const { delta, error } = JSON.parse(payload);
    if (error) { showError(error); return; }

    if (!bubble) {                        // create the bubble ONCE
      hideTypingIndicator();
      bubble = createAssistantBubble("");
    }
    bubble.textContent += delta;          // append into the same bubble
    scrollToBottom();
  }
}
```

Three details worth internalizing:

**`res.body` is a `ReadableStream<Uint8Array>`.** Raw bytes, not text. `reader.read()` returns `{ value, done }`. When `done` is `true`, `value` is `undefined`, so never use the value on the final read.

**`TextDecoder` with `{ stream: true }`.** UTF-8 is variable length: `€` is 3 bytes, an emoji is 4. A network chunk can split one character across two reads. Without `stream: true` you get `` on boundaries, and Devanagari or emoji output will visibly corrupt. The flag makes the decoder hold incomplete byte sequences until the next call.

**Frame buffering.** `reader.read()` gives you network chunks, not SSE events. The `split("\n\n")` + `pop()` pattern is what reconciles the two. Skipping this produces intermittent `JSON.parse` failures that only show up under load or on slow connections, which is the worst kind of bug.

**One bubble, not N bubbles.** Create the DOM node when the first delta arrives, then append `textContent`. If you create a bubble per chunk you get a cascade of one-word message bubbles. Also: use `textContent` (or sanitize), not `innerHTML`, or model output becomes an XSS vector.

### What about `EventSource`?

The browser has a native SSE client:

```js
const es = new EventSource("/api/chat?q=hello");
es.onmessage = (e) => { bubble.textContent += JSON.parse(e.data).delta; };
```

It handles frame parsing and auto-reconnect for free. But it's **GET-only and can't send custom headers**, so you can't POST a long message body or attach an `Authorization` header. That's why essentially every chat UI uses `fetch` + `ReadableStream` instead and reimplements the parsing.

---

## 10. Maintaining history while streaming

The problem in Java was a type mismatch: `history.add(new AssistantMessage(response))` where `response` is a `Flux<String>`, not a `String`. The conceptual problem is universal: **history needs the complete message, but streaming gives you fragments, and you must not delay the client while waiting for completeness.**

The pattern: accumulate as a side effect of forwarding.

| Language | Accumulator | Per-chunk hook | Completion hook |
|---|---|---|---|
| Java | `StringBuilder` | `.doOnNext()` | `.doOnComplete()` |
| Python | `list[str]` + `"".join()` | inside the `async for` | `finally:` block |
| Go | `strings.Builder` | inside the loop | after channel close |
| JS | array + `join("")` | inside the `while` | after `done` |

Java:

```java
StringBuilder full = new StringBuilder();
return chatClient.prompt()
        .system(SYSTEM)
        .messages(history)
        .stream()
        .content()
        .doOnNext(full::append)
        .doOnComplete(() -> history.add(new AssistantMessage(full.toString())));
```

Key point: `doOnNext` is a *transparent side effect*. The chunk still flows downstream to the client immediately; you're just also copying it into the buffer. You are not buffering *instead of* forwarding.

Python, same semantics, using `finally` so it runs on normal completion, exception, and client disconnect:

```python
buffer = []
try:
    async for delta in provider_stream:
        buffer.append(delta)
        yield sse(delta)
finally:
    if buffer:
        history.append({"role": "assistant", "content": "".join(buffer)})
```

Use `"".join(list)` rather than `s += delta` in a loop. Python strings are immutable, so repeated concatenation is O(n²) over the response. Same reason Java uses `StringBuilder` instead of `+`.

**Partial saves matter.** If the user hits stop at token 300 of 900, do you persist the partial assistant message? For a chat UI, yes, and mark it truncated, otherwise the next turn's context has a user message with no reply and the model gets confused. This is exactly what the `finally` block buys you.

One more thing the toy example hides: `history` as a module-level list is a single shared conversation across every user of your server. In anything real, history is keyed by conversation ID and lives in Postgres or Redis, and the write happens at stream completion inside a transaction.

---

## 11. The whole pipeline end to end

```
 ┌─────────┐  1. POST /api/chat {message}          ┌──────────────┐
 │ Browser │ ────────────────────────────────────▶ │ Your backend │
 │         │                                        │              │
 │         │                                        │  2. POST /v1/chat/completions
 │         │                                        │     {stream:true}   │
 │         │                                        │         ▼           │
 │         │                                        │  ┌──────────────┐   │
 │         │                                        │  │ AI provider  │   │
 │         │                                        │  │  ┌────────┐  │   │
 │         │                                        │  │  │  LLM   │  │   │
 │         │                                        │  │  │ prefill│  │   │
 │         │                                        │  │  │ decode │  │   │
 │         │                                        │  │  └───┬────┘  │   │
 │         │        STREAM 1: SSE, text/event-stream│  └──────┼───────┘   │
 │         │  ◀──────────────────────────────────── ◀─────────┘           │
 │         │        STREAM 2: SSE, text/event-stream                       │
 │  reader │                                                               │
 │  decoder│    ── accumulate into StringBuilder/list on the way past ──   │
 │  bubble │                                                               │
 └─────────┘                                        └──────────────┘
```

Timeline for a 900-token answer:

```
t=0.00  browser POSTs
t=0.01  backend POSTs to provider (stream:true)
t=0.45  provider finishes prefill, emits first delta
t=0.46  backend receives it, appends to buffer, writes SSE frame, FLUSHES
t=0.47  browser reader.read() resolves, decodes, creates bubble, renders "The"
t=0.48  second delta ...
...
t=8.00  provider sends finish_reason:"stop", then [DONE]
t=8.01  backend writes assistant message to history, sends [DONE], closes
t=8.02  browser sees [DONE], stops the loop
```

Every arrow needs to be non-buffering. One buffered hop anywhere in that chain, and the user sees a spinner for 8 seconds no matter how correct the other five hops are.

---

## 12. Production checklist

- Set `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no`.
- Flush explicitly after every event in Go, Node, PHP.
- Disable proxy buffering and gzip on streaming routes.
- Send a keepalive comment (`: ping\n\n`) every 15–30s if generation can stall, so LBs don't reap the connection.
- Handle client disconnect (`req.is_disconnected()`, `ctx.Done()`, `AbortController` on the client) and cancel the upstream provider call so you stop paying for tokens nobody will read.
- Report mid-stream errors in-band; you can't change the status code after byte one.
- Check `finish_reason` and surface truncation.
- Buffer SSE frames on the client before parsing.
- Use `TextDecoder(..., {stream: true})`.
- Escape/sanitize model output before rendering; never `innerHTML`.
- Track TTFT and tokens/sec as separate metrics. They fail for different reasons: TTFT spikes mean queueing or oversized prompts, ITL spikes mean GPU contention.

---

The mental summary: **the model streams because it has no choice; the provider streams because you asked; your backend streams because you flushed; your browser streams because you read instead of awaiting the whole body.** Break any one link and the user waits.
