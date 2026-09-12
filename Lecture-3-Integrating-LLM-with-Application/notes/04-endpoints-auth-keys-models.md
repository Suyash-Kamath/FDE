# 04 — Endpoints, Authentication, API Keys, Models, Input

Everything the video configures in Postman, explained properly.

---

## 1. The endpoint

```
POST https://api.openai.com/v1/responses
```

Dissected:

| Part | Meaning |
|---|---|
| `https` | TLS is mandatory. Your key travels in a header; plaintext HTTP would leak it. |
| `api.openai.com` | The **API** host. `platform.openai.com` is the dashboard; `chatgpt.com` is the product. Three different things. |
| `/v1` | API version. Providers add fields freely within a version and break only across versions. |
| `/responses` | The resource. Note: **plural**. |
| `POST` | You are sending a body and the call is not idempotent (it costs money and samples randomly). |

### The 404 in the video

Aditya typed `/v1/response` and got **404 Not Found**. Correct path is `/v1/responses`.

Worth internalising the diagnostic reflex: **404 means the URL is wrong, never that
your key is wrong.** Auth problems are 401/403. A key mistake cannot produce a 404.

### Two endpoints, two body shapes

OpenAI currently offers both:

| | `/v1/chat/completions` | `/v1/responses` |
|---|---|---|
| Age | The long-standing standard | Newer, unified |
| Input field | `messages: [{role, content}]` | `input: string` or `input: [items]` |
| Output field | `choices[0].message.content` | `output[].content[].text` (or `output_text` helper) |
| Built-in tools | function calling | function calling + hosted tools (web search, code interpreter, file search) |
| Server-side state | none | optional (`store`, `previous_response_id`) |
| Ecosystem | **Almost every other provider clones this shape** | OpenAI-specific |

That last row is the practical one: Groq, Together, DeepSeek, Mistral, OpenRouter,
vLLM, Ollama all expose an OpenAI-*Chat-Completions*-compatible endpoint. If you want
provider portability by just swapping `base_url`, target Chat Completions. If you want
OpenAI's hosted tools and server-side conversation state, target Responses.

---

## 2. HTTP anatomy of the request

```http
POST /v1/responses HTTP/1.1
Host: api.openai.com
Authorization: Bearer sk-proj-xxxxxxxxxxxxxxxxxxxxxxxx
Content-Type: application/json
Accept: application/json

{"model":"gpt-5.6-luna","input":"Explain me Docker in two lines"}
```

Postman's "Authorization → Bearer Token" field is just a UI that writes that
`Authorization` header — which is exactly what the video observes under "hidden
headers". There is nothing special about it.

`Bearer` is from OAuth 2.0 / RFC 6750 and the name is literal: **whoever bears this
token is treated as you.** No signature, no device binding, no expiry by default.

---

## 3. What an API key actually is

The video says it does authentication. True, and incomplete. A provider key carries:

1. **Identity** — which account/organization/project.
2. **Authorization** — which models and endpoints this key may touch (modern keys are
   scoped per project with restricted permissions).
3. **Billing attribution** — every token is metered against it.
4. **Rate limiting** — RPM/TPM buckets are keyed to it.
5. **Audit** — usage dashboards break down spend per key.

It is *not*:

- Encryption (TLS does that).
- A user identity. Your end users must never have one. Your key identifies **your
  application** to the provider; you authenticate your users separately.
- Revocable after the fact for damage already done. A leaked key can be burned within
  minutes.

### The architectural rule this implies

```
   ✗ WRONG                                ✓ RIGHT

   Browser / mobile app                   Browser / mobile app
        │  key baked in!                       │  your own session/JWT
        ▼                                      ▼
   api.openai.com                         YOUR BACKEND  ── key in env/secret manager
                                               │         ── authn, authz, rate limit,
                                               ▼            audit, cost cap, prompt
                                          api.openai.com     assembly, PII redaction
```

**Never put a provider key in a browser, a mobile binary, or a desktop app.** Anything
shipped to a user's device is public. Your backend is the only correct place — which
also happens to be where you add per-user quotas and abuse controls.

### Key hygiene checklist

- [ ] Key lives in an environment variable or secret manager (Vault, AWS Secrets
      Manager, SSM Parameter Store), never in code, never in `application.properties`
      committed to git.
- [ ] `.env` in `.gitignore`; a secret scanner (gitleaks / trufflehog) in CI.
- [ ] One key per environment (dev/staging/prod) and per service, so you can rotate
      one without downtime elsewhere.
- [ ] Project-scoped and permission-restricted where the provider supports it.
- [ ] A **hard monthly spend limit** and budget alerts configured in the dashboard.
      Do this before your first request, not after the first bad bill.
- [ ] Rotation runbook written and tested.
- [ ] Never logged. Watch out for frameworks that log full request headers on error.

If a key leaks: revoke first, rotate second, then audit usage for the window.

---

## 4. Choosing a model

### The ladder

Providers ship a tiered family: a flagship, a mid-tier at a fraction of the price, and
a small/cheap model for volume. As of **September 2026** OpenAI's ladder looked like
this (USD per 1M tokens, standard short-context tier):

| Model | Input | Output | Use for |
|---|---|---|---|
| GPT-6 Astra (flagship, released 3 Sep 2026) | $10 | $50 | Hardest reasoning, when quality dominates cost |
| GPT-5.6 Sol | ~$4–5 | ~$20–30 | Strong general production default |
| GPT-5.6 Terra | $2 | $12 | Balanced |
| GPT-5.6 Luna | $0.20 | $1.20 | High-volume routine work — summarising, classifying, extracting |

> Sources differ slightly on Sol (promotional vs list rate), and Luna's price was cut
> ~80% on 30 July 2026. **Verify on the provider's live pricing page before quoting
> these anywhere.** Prices in this market move monthly.

Also relevant from the same pricing pages: cached input bills at ~10% of standard
input; the Batch API halves everything; "Fast" tiers double it; and requests past the
long-context threshold (~272K tokens) reprice the *whole* request at roughly 2× input
and 1.5× output. More on all of that in [`07`](07-tokens-cost-latency-context.md).

### How to actually choose

Do not start from the price table. Start from the task:

1. **Write an eval set first** — 30–50 real inputs with expected outputs, or a rubric.
2. **Start at the cheapest plausible tier** (Luna-class) and measure.
3. **Escalate only where it fails.** Most production LLM traffic is summarise /
   classify / extract / rewrite, and small models are excellent at those.
4. **Consider routing**: cheap model by default, escalate to the big one when a
   confidence signal or a classifier says the input is hard.
5. **Pin the version.** Use an explicit dated snapshot if the provider offers one, so
   behaviour doesn't drift under you. Re-run your eval before upgrading.

Aditya's advice in the video is sound: for learning, take the cheap older model, put in
the $5 minimum, and you will not run out for a long time. At $0.20/1M input, $5 buys
an enormous amount of practice.

### Free options and their catch

**OpenRouter** is a gateway: one key, one OpenAI-compatible endpoint, many providers
(OpenAI, Anthropic, Google, Mistral, DeepSeek, open-weights hosts), some with free
tiers. Genuinely useful for learning and for A/B-ing models.

Caveats the video hints at, made explicit:
- Free tiers are heavily rate-limited and get deprecated without notice.
- Smaller context windows.
- Your prompts may be used for training on some free routes — **never send customer
  data through a free tier**. Check the data-retention policy per route.
- An extra network hop adds latency.

Self-hosting (Ollama, vLLM, llama.cpp) is the other free path: zero marginal cost, full
data control, and you pay in GPU and ops instead. Good for dev loops and privacy-bound
workloads.

---

## 5. The request body, field by field

Minimal (what the video sends):

```json
{
  "model": "gpt-5.6-luna",
  "input": "Explain me Docker in two lines"
}
```

Realistic production body:

```jsonc
{
  "model": "gpt-5.6-luna",

  // WHAT to think about
  "input": [
    { "role": "system", "content": "You are a support-ticket summariser. …" },
    { "role": "user",   "content": "<<<ticket text>>>" }
  ],

  // HOW to sample
  "temperature": 0.2,        // 0 = near-deterministic. Low for extraction/summary,
                             // higher (0.7–1.0) for creative drafting.
  "top_p": 1,                // nucleus sampling. Tune temperature OR top_p, not both.

  // COST AND SAFETY RAILS
  "max_output_tokens": 150,  // hard cap. ALWAYS set this. It bounds your worst-case bill
                             // and your worst-case latency.

  // SHAPE OF THE OUTPUT
  "text": {
    "format": {
      "type": "json_schema",
      "name": "ticket_summary",
      "strict": true,
      "schema": { /* … */ }
    }
  },

  // CAPABILITIES
  "tools": [ /* … */ ],
  "tool_choice": "auto",     // "auto" | "none" | "required" | {specific tool}

  // DELIVERY
  "stream": true,            // Server-Sent Events, token by token

  // OPS
  "metadata": { "tenant": "acme", "feature": "ticket-summary" },
  "store": false             // don't retain the response server-side
}
```

The three fields juniors forget, in order of how much pain they cause:

1. **`max_output_tokens`** — without it a runaway generation costs real money and
   blows your latency SLO.
2. **`temperature`** — leaving it at the default for a data-extraction task gives you
   inconsistent JSON and flaky tests.
3. **Structured output** — parsing prose with regex is the number one source of
   production LLM bugs. Ask for a schema, get a schema.

---

## 6. Rate limits and resilience

You will be limited on at least two axes:

- **RPM** — requests per minute
- **TPM** — tokens per minute (input + output)

Exceeding either returns **429** with `Retry-After` and rate-limit headers. Your
client must handle it:

```
retry on: 429, 500, 502, 503, 504, connection reset, timeout
do NOT retry on: 400, 401, 403, 404, 422  (they'll fail identically)

backoff: exponential with full jitter
   sleep = random(0, min(cap, base * 2^attempt))
attempts: 3–5, then fail to a fallback
timeout: set one! LLM calls can hang. 30–60 s for non-streaming.
```

Also plan the **fallback path**: what does your feature do when the provider is down?
Options: queue and process later, degrade to a non-AI path (show the raw ticket),
fail over to a second provider, or serve a cached result. Decide before launch —
"the provider had an incident" is not an acceptable answer to your own users.

For write-effecting flows, send an idempotency key so a retry doesn't duplicate work.

---

## 7. Checkpoint questions

1. You get a 404. Name three things it is *not*.
2. Why should a React Native app never hold an OpenAI key, and what goes in its place?
3. You need to summarise 50,000 tickets overnight. Which three pricing levers apply?
4. What breaks if you omit `max_output_tokens` on a user-facing endpoint?
5. Your CTO asks "can we switch to Anthropic next quarter?" — which endpoint choice
   makes that easier and why?

→ Next: [`05-first-request-postman.md`](05-first-request-postman.md)
