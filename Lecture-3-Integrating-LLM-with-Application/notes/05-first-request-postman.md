# 05 — Sending Your First LLM Request (Postman, curl, and the errors)

The video's Postman walkthrough, reproduced exactly and then extended into the
workflow you'd actually use.

---

## 1. Why Postman *before* code

This is a genuinely good habit and worth naming:

- Separates "is my request shape right?" from "is my code right?" — two bugs at once
  is four times the debugging.
- Shows you the **raw** response envelope. Once an SDK wraps it, you stop seeing the
  `usage` block, the `finish_reason`, the `service_tier` — the fields that matter
  operationally.
- Lets you iterate on prompts in seconds without a recompile.
- A saved collection becomes shareable documentation for your team.

---

## 2. Step by step

**1. Method and URL**
```
POST   https://api.openai.com/v1/responses
```

**2. Authorization**
`Authorization` tab → Type: **Bearer Token** → paste your key.
Postman writes `Authorization: Bearer sk-…` into the (hidden) headers, which is
exactly what the video shows when Aditya opens the hidden-headers panel.

**Better:** don't paste the key literally. Create a Postman **Environment** with a
variable `OPENAI_KEY` marked *secret*, and use `{{OPENAI_KEY}}`. Now the collection is
safe to export and share.

**3. Headers**
`Content-Type: application/json` — Postman adds it when you pick a JSON body.

**4. Body** → `raw` → `JSON`

```json
{
  "model": "gpt-5.6-luna",
  "input": "Explain me Docker in two lines"
}
```

**5. Send.** Expect `200 OK` and a large JSON envelope. If you get `404`, check the
path is `/v1/responses` (plural) — this is the exact mistake made in the video.

---

## 3. The same call in other forms

### curl
```bash
curl https://api.openai.com/v1/responses \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.6-luna",
    "input": "Explain me Docker in two lines"
  }'
```

Keep the key in an env var, never inline — shell history is a file on disk.

### Chat Completions shape (portable across providers)
```bash
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.6-luna",
    "messages": [
      {"role": "system", "content": "You answer in exactly two lines."},
      {"role": "user",   "content": "Explain Docker."}
    ],
    "temperature": 0.2,
    "max_tokens": 100
  }'
```

Point the same request at another provider by changing only the host and key:

```bash
# Groq
curl https://api.groq.com/openai/v1/chat/completions -H "Authorization: Bearer $GROQ_KEY" …
# OpenRouter
curl https://openrouter.ai/api/v1/chat/completions  -H "Authorization: Bearer $OR_KEY"  …
# local Ollama
curl http://localhost:11434/v1/chat/completions     -H "Authorization: Bearer ollama"   …
```

That portability is the practical argument for the Chat Completions shape.

---

## 4. The second experiment: proving statelessness

The video does this and it's the single most important demo in the whole session.

**Request A**
```json
{ "model": "gpt-5.6-luna", "input": "Hi, my name is Aditya" }
```
→ *"Hi Aditya, nice to meet you. How can I help you today?"*

**Request B** (new request, same key, seconds later)
```json
{ "model": "gpt-5.6-luna", "input": "What is my name?" }
```
→ *"I don't know your name yet."*

Meanwhile the Playground and ChatGPT both answer *"Your name is Aditya."*

**Why:** the Playground is a chat client — it keeps the transcript and resends it.
Your Postman request sent one message with no history. The API has no session, no
cookie, no server-side conversation (unless you opt in to `store` +
`previous_response_id`). Two requests with the same key are completely unrelated.

The fix is entirely on your side and it is mechanical: **resend the transcript.**

```json
{
  "model": "gpt-5.6-luna",
  "input": [
    { "role": "user",      "content": "Hi, my name is Aditya" },
    { "role": "assistant", "content": "Hi Aditya, nice to meet you." },
    { "role": "user",      "content": "What is my name?" }
  ]
}
```
→ *"Your name is Aditya."*

Full treatment of the cost and design consequences in [`10`](10-system-user-assistant-roles.md).

---

## 5. Error reference

| Code | Meaning | Usual cause | Fix |
|---|---|---|---|
| **400** | Bad request | Malformed JSON, unknown field, bad param combination | Read `error.message` — it names the field |
| **401** | Unauthorized | Missing/typo'd/revoked key, `Bearer` prefix missing | Recheck the header |
| **403** | Forbidden | Key lacks permission for this model/endpoint, or region blocked | Check project scopes |
| **404** | Not found | **Wrong URL path**, or a model name that doesn't exist | Check the path spelling and the model string |
| **413** | Payload too large | Enormous prompt | Chunk it |
| **422** | Unprocessable | Schema/validation failure | Fix the body |
| **429** | Rate limited **or** out of credit | Two very different things — read the body | `rate_limit_exceeded` → back off; `insufficient_quota` → add credit |
| **500 / 502 / 503 / 504** | Provider-side | Transient | Retry with exponential backoff + jitter |

The 429 ambiguity catches people constantly. Branch on `error.code`, not the status:

```java
if (status == 429 && "insufficient_quota".equals(err.code())) {
    // retrying will NEVER help. Alert billing.
} else if (status == 429) {
    // back off and retry
}
```

### Error body shape
```json
{
  "error": {
    "message": "Incorrect API key provided: sk-pro***xyz.",
    "type": "invalid_request_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

---

## 6. Turning this into a workflow

**Build a Postman collection with:**

- An **environment** per provider/stage (`OPENAI_KEY`, `BASE_URL`, `MODEL`), keys
  marked secret.
- A folder per pattern: bare completion, with system prompt, with JSON schema, with
  tools, streaming.
- **Tests** on each request, so it becomes a smoke suite:

```javascript
pm.test("200 OK", () => pm.response.to.have.status(200));

const body = pm.response.json();
pm.test("has output text", () => {
  const txt = body.output
     .filter(o => o.type === "message")
     .flatMap(m => m.content)
     .filter(c => c.type === "output_text")
     .map(c => c.text).join("");
  pm.expect(txt.length).to.be.above(0);
});

pm.test("within token budget", () =>
  pm.expect(body.usage.total_tokens).to.be.below(500));

console.log("tokens:", body.usage.input_tokens, "+", body.usage.output_tokens);
```

That last assertion is unusual and valuable: **a regression test on cost.** If someone
bloats the system prompt, the test fails before the invoice does.

---

## 7. Checkpoint questions

1. You changed nothing but the model name and now get 404. What happened?
2. Two requests, same key, one minute apart. Does the second know about the first? Why?
3. Which 429 should you retry and which should you page someone about?
4. Why is a `total_tokens` assertion in a smoke test a good idea?

→ Next: [`06-understanding-the-response.md`](06-understanding-the-response.md)
