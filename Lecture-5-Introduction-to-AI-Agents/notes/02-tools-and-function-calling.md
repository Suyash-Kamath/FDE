# 02 · What Tools Actually Are, and What Goes Over the Wire

## 2.1 The one-sentence definition

> A **tool** is a JSON Schema describing a function, sent to the model as text,
> plus a Python callable in your process that you agree to run when the model
> names that schema.

There are two halves and they live in different places:

| Half | Lives where | The model sees it? |
|---|---|---|
| The **schema** (name, description, parameter types) | In your API request, serialised into the prompt | **Yes** — it's literally prompt text |
| The **implementation** (your Python function body) | In your process | **Never** |

This split explains almost every confusing thing about tool calling:

- *Why does the description matter so much?* Because the description **is the
  prompt**. It's not documentation; it's instruction.
- *Why do tools cost tokens?* Because the schema is serialised into the input
  context on every single request.
- *Why can't the model "see" my bug?* It only ever sees the schema and the
  string you return. Your function body is invisible to it.
- *Why can I lie in a description?* You can, and the model will believe you, and
  it will call the tool wrongly. Descriptions are a contract you must keep.

## 2.2 Tools are injected into the system prompt

This is the mechanical fact most tutorials skip. When you pass `tools=[...]`,
the API doesn't do anything magic at the neural-network level. It **builds a
system prompt** containing your tool definitions in JSON Schema form, along with
formatting instructions telling the model how to emit a call, and prepends it to
your own system prompt.

Anthropic documents this openly: the API constructs a special system prompt from
the tool definitions, tool configuration, and your system prompt, roughly in the
order *tool-use preamble → formatting instructions → tool definitions as JSON
Schema → your system prompt → tool configuration*. OpenAI documents the same
thing more tersely: functions are injected into the system message in a syntax
the model was trained on, and count against the context limit as input tokens.

Practical consequences, all of which will bite you eventually:

1. **Tool definitions are billed input tokens, every request.** 25 tools with
   chatty descriptions can be 4–6k tokens of overhead *per call*. In a 20-step
   agent loop that's 100k+ tokens spent re-sending schemas.
2. **They compete for attention with your system prompt.** More tools = worse
   selection accuracy. OpenAI's guidance is to keep fewer than ~20 tools
   callable at the start of a turn. Anthropic's guidance is to consolidate
   related operations into one tool with an `action` parameter rather than
   shipping `create_pr` / `review_pr` / `merge_pr` separately.
3. **Tool schemas sit at the front of the prompt, so they are cacheable.** Put
   stable things (tools, system prompt) first and volatile things (the growing
   conversation) last, and prompt caching will pay for most of the overhead.
   Changing `tool_choice` invalidates cached message blocks, so don't toggle it
   per-request casually.
4. **The model was *trained* on this format.** That is the only reason it works.
   Tool calling is not an architectural feature of transformers; it's a learned
   behaviour reinforced during post-training. Which is why tool-calling quality
   varies dramatically between models and why a fine-tuned 7B model will happily
   emit malformed calls.

## 2.3 Natural language → structured tool call

This is the step that is genuinely hard and that only the LLM can do. Look at
what has to happen for this input:

```
"I'm earning in euros. I have 5000 euros. What if I convert my euros
 to pounds? How much exactly will I get back?"
```

The model must perform, in a single forward pass sequence:

1. **Intent classification** — this is a currency conversion, not a weather query.
2. **Entity extraction** — amount = 5000.
3. **Normalisation to a coded vocabulary** — "euros" → `EUR`, "pounds" → `GBP`.
   Note there's nothing in the user's text that says "EUR". The model supplied
   ISO-4217 codes from world knowledge because your schema's description told it
   codes are required.
4. **Directionality** — from EUR, to GBP, not the reverse. Getting this backwards
   is one of the most common tool-calling bugs and is purely a description-quality
   problem.
5. **Decomposition** — "how much exactly will I get back" needs *rate × amount*,
   which is a second tool call that depends on the first one's output. The model
   has to notice the dependency and sequence it.
6. **Format compliance** — emit exactly the JSON the schema demands.

Steps 1–5 are why you use an LLM at all. Step 6 is where things break, and it's
where `strict` mode helps.

### The thing you must prevent: free-form arguments

Your Python function is a `if op == "add"` chain. The model, left alone, will
cheerfully send `"plus"`, `"+"`, `"sum"`, `"addition"`, or `"Add"` — all
reasonable English, all a `KeyError` for you. Two defences, use both:

```python
# 1. Constrain in the schema — enum is a hard constraint the decoder respects
"operation": {
    "type": "string",
    "enum": ["add", "subtract", "multiply", "divide", "modulo", "power"],
    "description": "The arithmetic operation to perform. Must be exactly one "
                   "of the listed values, lowercase.",
}

# 2. Be liberal in your implementation anyway — defence in depth
_ALIASES = {"plus": "add", "+": "add", "sum": "add", "addition": "add",
            "minus": "subtract", "-": "subtract", "times": "multiply",
            "*": "multiply", "x": "multiply", "/": "divide", "%": "modulo"}
op = _ALIASES.get(raw.strip().lower(), raw.strip().lower())
```

An `enum` in JSON Schema is not a suggestion. With strict/constrained decoding
the API masks out any token that would make the output violate the schema, so
the model is *physically unable* to emit `"plus"`. Without strict mode it's a
strong hint that is usually but not always obeyed.

## 2.4 Anatomy of a tool definition

The two SDKs use the same JSON Schema, wrapped differently. Learn the shape once.

### Anthropic

```python
{
    "name": "calculate",                       # ^[a-zA-Z0-9_-]{1,128}$
    "description": "...",                      # ← the prompt. 3-4 sentences min.
    "input_schema": {                          # ← plain JSON Schema
        "type": "object",
        "properties": {
            "operation": {"type": "string", "enum": [...], "description": "..."},
            "a": {"type": "number", "description": "..."},
            "b": {"type": "number", "description": "..."},
        },
        "required": ["operation", "a", "b"],
    },
    # optional
    "input_examples": [{"operation": "add", "a": 5, "b": 10}],
    "strict": True,
}
```

### OpenAI — Responses API (current default)

```python
{
    "type": "function",
    "name": "calculate",
    "description": "...",
    "parameters": {                            # ← same JSON Schema, renamed
        "type": "object",
        "properties": {...},
        "required": ["operation", "a", "b"],
        "additionalProperties": False,         # required by strict mode
    },
    "strict": True,
}
```

### OpenAI — Chat Completions (older, nested one level deeper)

```python
{
    "type": "function",
    "function": {                              # ← the extra nesting level
        "name": "calculate",
        "description": "...",
        "parameters": {...},
        "strict": True,
    },
}
```

> **Gotcha that costs everyone an hour:** Responses API puts `name`/`parameters`
> at the top level; Chat Completions nests them under `"function"`. Copy-pasting
> between them silently produces a 400.

### Field-by-field, and what each one is really for

| Field | Purpose | How to get it wrong |
|---|---|---|
| `name` | The token the model emits to route the call | Vague names (`get_data`, `process`) → wrong tool chosen |
| `description` | **The instruction.** When to use it, when *not* to, what it returns, caveats | One line ("gets the weather"). Aim for 3–4 sentences |
| `properties[x].description` | Per-argument instruction — format, units, vocabulary | Omitting it and wondering why `from`/`to` get swapped |
| `enum` | Hard-constrains a string to a fixed vocabulary | Not using it for anything categorical |
| `required` | Which args must be present | Making everything optional so the model omits the important one |
| `strict` | Turns the schema into a decoding constraint, guaranteeing valid JSON | Leaving it off and then writing defensive parsing code forever |
| `input_examples` (Anthropic) | Schema-validated example inputs for complex/nested shapes | Using it as a substitute for a good description |

### The "intern test"

OpenAI's guidance, and it's the best heuristic in the space: hand your schema to
a competent intern with **no other context**. Can they call the function
correctly? If they'd ask you a question, the answer to that question belongs in
the description. `from` and `to` on a currency tool fail this test instantly
unless you write *"source currency code, e.g. USD"* and *"target currency code,
e.g. INR"*.

### Good vs bad description

```python
# ❌ the model will guess, and guess wrong
"description": "Gets the exchange rate."

# ✅
"description": (
    "Fetches the latest foreign-exchange rate between two currencies from a "
    "live market data provider. Returns how many units of the target currency "
    "one unit of the source currency is worth, as a JSON object. Use this "
    "whenever the user asks about converting money between currencies, or "
    "about today's or the current exchange rate. Both currencies must be "
    "given as ISO-4217 three-letter uppercase codes (USD, INR, EUR, GBP, JPY). "
    "This tool returns only the rate — it does NOT multiply by an amount; use "
    "the calculate tool for that. It does not support historical rates or "
    "cryptocurrencies."
)
```

Note what that description does beyond describing: it states the **output
format**, the **trigger condition**, the **argument vocabulary**, an explicit
**negative boundary** ("does not multiply"), and the **limitations**. The
negative boundary is what makes the model chain to the calculator instead of
doing the arithmetic in its head.

## 2.5 Three design principles for tools

**1. Offload work to code, don't make the model fill in what you already know.**
If your app already knows `order_id` from session state, do *not* put it in the
schema. `submit_refund()` with zero parameters and the ID injected in your code
is strictly better than `submit_refund(order_id)` — one fewer thing to
hallucinate, and the model physically cannot refund the wrong order.

**2. Make invalid states unrepresentable.** `toggle_light(on: bool, off: bool)`
permits `on=True, off=True`. `set_light(state: "on"|"off")` doesn't. Use enums
and required fields the way you'd use a type system.

**3. Shape the *return value* for the model, not for a human.** The tool result
is context the model must reason over. Returning a 40-field JSON blob when the
model needs two fields wastes tokens and buries the signal. Return semantic,
stable identifiers (slugs, codes) rather than opaque internal numbers, and only
the fields needed for the next decision.

```python
# ❌ dumps the provider's entire response — 800 tokens of noise
return json.dumps(weather_api_response)

# ✅ 40 tokens, all signal
return json.dumps({
    "city": data["location"]["name"],
    "country": data["location"]["country"],
    "temp_c": data["current"]["temp_c"],
    "condition": data["current"]["condition"]["text"],
    "humidity": data["current"]["humidity"],
    "observed_at_local": data["location"]["localtime"],
})
```

## 2.6 The full round trip, as data

Before any code, hold this picture. Four HTTP-level events, two round trips:

```
──────────────────────────────────────────────────────────────────────
 1. YOU → API
    system: "You are a helpful assistant..."
    tools : [calculate schema, get_weather schema, get_rate schema]
    messages:
      user → "I have 5000 EUR. How many GBP do I get today?"

 2. API → YOU          stop_reason = "tool_use"   (finish_reason = "tool_calls")
    assistant:
      [text]     "Let me look up today's rate."
      [tool_use] id=toolu_01A  name=get_exchange_rate  input={from:EUR,to:GBP}

    ⚠️ Nothing has been executed. This is a *request*.

 3. YOU execute get_exchange_rate("EUR","GBP") → {"rate": 0.858}
    YOU → API   (resend EVERYTHING: system + tools + all 3 messages)
    messages:
      user      → "I have 5000 EUR..."
      assistant → [text + tool_use block, verbatim]
      user      → [tool_result tool_use_id=toolu_01A content='{"rate":0.858}']

 4. API → YOU          stop_reason = "tool_use"   ← it wants ANOTHER tool
    assistant:
      [tool_use] id=toolu_02B  name=calculate  input={op:multiply,a:5000,b:0.858}

 5. YOU execute → 4290.0 ;  resend everything again

 6. API → YOU          stop_reason = "end_turn"   ← done
    assistant: [text] "At today's rate of 0.858, 5000 EUR gives you £4,290."
──────────────────────────────────────────────────────────────────────
```

Five facts to burn in:

- **The model never executes anything.** Step 2 is a request, not an action.
  You are free to ignore it, modify it, or ask the user first.
- **The API is stateless.** Step 3 resends the *entire* history including the
  tool definitions. There is no server-side session (unless you opt into one —
  `previous_response_id` in the Responses API, or Anthropic's Managed Agents).
- **`stop_reason` is the loop condition.** `tool_use` → keep looping.
  `end_turn` → return to the user. Never parse the prose to decide.
- **IDs must match exactly.** `tool_use_id` / `call_id` links a result to its
  request. With parallel calls, mismatched IDs is the #1 bug.
- **The assistant's tool_use block must be echoed back verbatim.** You can't
  summarise it or drop it. The API rejects a `tool_result` with no matching
  `tool_use` in the preceding assistant turn.

## 2.7 Three ways this gets called in practice

| Approach | What it is | When |
|---|---|---|
| **Raw SDK + your own loop** | What these notes teach | Learning; and production, because you need control over approval, budgets, logging |
| **SDK loop helper** | Anthropic ships a Tool Runner that runs the agentic loop for you; OpenAI has the Agents SDK | Fast prototypes; simple tool sets |
| **Framework** (LangChain, LlamaIndex, CrewAI) | Abstraction over both | When the team already uses it. Learn the raw loop first or you'll be debugging someone's abstraction over a thing you don't understand |

Write it raw at least three times before reaching for a framework. The loop is
about 40 lines.

➡️ Next: [03 · Calculator tool with the Anthropic SDK](03-tool-calling-anthropic.md)
