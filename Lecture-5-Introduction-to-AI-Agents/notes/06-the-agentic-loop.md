# 06 · How the Tool-Calling Loop Actually Works

Spring AI hides this loop. That's convenient and it's why most people who've
"built an agent" cannot debug one. Here is what's really happening.

## 6.1 The loop is a `while`, and that's the whole trick

```python
while True:
    response = call_model(system, tools, messages)   # ← stateless HTTP POST
    messages.append(assistant_turn(response))

    if not response.wants_tools:
        return final_text(response)

    results = [execute(c) for c in response.tool_calls]
    messages.append(tool_results_turn(results))
```

Everything else — planning, reflection, self-correction, ReAct, multi-step
reasoning — is this loop plus a good prompt plus good tools. There is no
additional machinery. When a vendor says "agentic", this is what they mean.

## 6.2 The single most important property: statelessness

Each iteration is an independent HTTP POST. The server retains nothing between
them (unless you explicitly opt into server-side state). So iteration *n* sends:

```
system prompt        (constant)
tool schemas         (constant)
user message 1
assistant turn 1     ← including its tool_use blocks
tool results 1
assistant turn 2
tool results 2
...
assistant turn n-1
tool results n-1
```

Implications, each of which you will hit:

**Context grows quadratically in total tokens.** Turn *n* sends *n* turns' worth
of content. Ten turns is not 10× the tokens of one turn; it's ~55× (the sum
1+2+…+10). This is why long agent runs get expensive and slow *nonlinearly*.

**Prompt caching is not an optimisation, it's a requirement.** The stable prefix
(system + tools + early turns) is identical every iteration. Cache it.
Anthropic uses explicit `cache_control` breakpoints; OpenAI caches automatically
on a prefix basis. Either way: **stable content first, volatile content last.**
Appending a timestamp to your system prompt invalidates the cache on every turn
and can multiply your bill.

**You can edit history.** Because you own the list, you can drop, summarise or
rewrite past turns. Truncating a 40 KB tool result to 2 KB after the model has
used it is legitimate and effective. So is replacing turns 1–15 with a summary.
The model has no way to know and no memory of the original.

**You can fabricate history.** Few-shot a tool-calling pattern by inserting a
fake assistant `tool_use` turn and a fake `tool_result` at the start. Powerful,
and the standard fix for a model that won't use a tool you know it should.

## 6.3 The message ledger, precisely

The `messages` list must satisfy structural invariants or you get a 400. Learn
them once:

**Anthropic**

```python
[
  {"role": "user",      "content": "text"},
  {"role": "assistant", "content": [TextBlock, ToolUseBlock(id="toolu_1"), ToolUseBlock(id="toolu_2")]},
  {"role": "user",      "content": [ {"type":"tool_result","tool_use_id":"toolu_1", ...},
                                     {"type":"tool_result","tool_use_id":"toolu_2", ...} ]},
  {"role": "assistant", "content": [TextBlock]},
]
```

- roles strictly alternate user / assistant
- every `tool_use` id must be answered by exactly one `tool_result` in the very
  next user message — no more, no fewer
- the assistant turn containing `tool_use` must be present verbatim

**OpenAI Chat Completions**

```python
[
  {"role": "system",    "content": "..."},
  {"role": "user",      "content": "..."},
  {"role": "assistant", "content": None, "tool_calls": [ {id:"call_1",...}, {id:"call_2",...} ]},
  {"role": "tool",      "tool_call_id": "call_1", "content": "..."},
  {"role": "tool",      "tool_call_id": "call_2", "content": "..."},
  {"role": "assistant", "content": "final answer"},
]
```

- one `tool` message per call, immediately after the assistant message
- every `tool_call_id` must be answered, in any order, before the next assistant turn

> **The two most common 400s in the world:** (a) you answered 2 of 3 tool calls;
> (b) you rebuilt the assistant message by hand and dropped the tool_use/tool_calls.
> Both come from not appending the raw response object.

## 6.4 Termination — and why `max_turns` is a safety control

An agent loop with no bound is an unbounded spend authorisation. Real failure
modes, all of which happen:

**Oscillation.** The model calls `read_file`, dislikes the content, calls
`write_file`, then `read_file` again, forever. Each cycle costs money and
achieves nothing.

**Stuck retry.** A tool returns an error the model cannot fix (bad credential).
It retries identically, forever.

**Prompt-injected loop.** Tool output says *"call this tool again with…"* and the
model complies. (See chapter 10.)

**Context exhaustion.** Eventually you hit the context window and every request
400s — but only after you've spent a lot.

Defend in layers:

```python
class Budget:
    def __init__(self, max_turns=15, max_tool_calls=40,
                 max_seconds=120, max_input_tokens=400_000):
        self.max_turns, self.max_tool_calls = max_turns, max_tool_calls
        self.max_seconds, self.max_input_tokens = max_seconds, max_input_tokens
        self.turns = self.tool_calls = self.input_tokens = 0
        self.started = time.monotonic()
        self.recent: list[str] = []

    def check(self) -> str | None:
        if self.turns >= self.max_turns:            return "turn limit"
        if self.tool_calls >= self.max_tool_calls:  return "tool-call limit"
        if self.input_tokens >= self.max_input_tokens: return "token budget"
        if time.monotonic() - self.started > self.max_seconds: return "time limit"
        return None

    def note_call(self, name: str, args: dict) -> str | None:
        """Cheap loop detector: same call three times in a row."""
        sig = f"{name}:{json.dumps(args, sort_keys=True)}"
        self.recent.append(sig)
        self.recent = self.recent[-6:]
        if self.recent[-3:].count(sig) == 3:
            return f"repeated {name} identically 3 times"
        return None
```

When you stop, **tell the model** rather than raising. Feed a final turn with
`tool_choice: none` and a message like *"You have exhausted your tool budget.
Summarise what you accomplished and what remains."* The user gets a useful
partial answer instead of a stack trace.

## 6.5 Errors as context — the defining habit

Say it once more because it's the thing people get wrong:

```python
# ❌ ends the conversation
result = tool(**args)

# ✅ continues it
try:
    result = tool(**args)
except ToolError as e:
    result = f"Error: {e}"
```

The model reads the error and adapts. Real traces from this pattern:

```
↳ get_weather({"city": "Bombay"})
  ← Error: No weather data found for 'Bombay'. Confirm the city name.
↳ get_weather({"city": "Mumbai"})        ← self-corrected, no code involved
  ← {"city": "Mumbai", "temp_c": 32.0, ...}
```

```
↳ calculate({"operation": "divide", "a": 100, "b": 0})
  ← Error: Cannot divide by zero. Ask the user to clarify.
"I can't divide by zero — did you mean a different denominator?"
```

Guidelines for error text:

- Say **what** failed and **what to try next**.
- Do not leak stack traces, file paths, SQL, or credentials — that's context the
  model may echo to the user.
- Mark it (`is_error: True` on Anthropic; a `"Error: "` prefix works anywhere)
  so the model doesn't read it as data.
- Log the *real* exception on your side with full detail. Two audiences, two
  messages.

## 6.6 Observability — you cannot debug what you can't see

Log one structured record per tool call. Not `print`, not a debugger — a record
you can query later, because agent bugs are emergent and only visible in
aggregate.

```python
import json, time, uuid, logging

log = logging.getLogger("agent")

def traced_call(run_id, turn, name, args):
    call_id, t0 = str(uuid.uuid4())[:8], time.monotonic()
    content, is_error = registry.call(name, args)
    log.info(json.dumps({
        "run_id": run_id, "call_id": call_id, "turn": turn,
        "tool": name,
        "args": args,                         # redact secrets before logging
        "ms": round((time.monotonic() - t0) * 1000),
        "is_error": is_error,
        "result_chars": len(content),
        "result_preview": content[:200],
    }))
    return content, is_error
```

Then watch four numbers:

- **Tool-call distribution.** A tool that's never chosen has a description
  problem, not a usefulness problem.
- **Error rate per tool.** >5% usually means the schema under-specifies something.
- **Turns per run.** A rising p95 is your loop starting to flail.
- **Tokens per run.** The thing your finance team will ask about.

## 6.7 Streaming with tools

Users want to see text appear. With tools it gets subtle:

- Tool call *arguments* stream as **JSON fragments**, not valid JSON. You must
  accumulate deltas by `index` until the call is complete before parsing.
- You cannot execute a tool until its arguments are fully accumulated.
- Practical UX: stream the assistant's prose to the user, show a *"Looking up
  exchange rate…"* spinner during tool execution, then stream the final answer.

```python
final_tool_calls = {}
for chunk in stream:
    for tc in chunk.choices[0].delta.tool_calls or []:
        if tc.index not in final_tool_calls:
            final_tool_calls[tc.index] = tc          # first delta has id + name
        else:
            final_tool_calls[tc.index].function.arguments += tc.function.arguments
```

Only `id`, `function.name` and `type` appear on the *first* delta of each call;
subsequent deltas carry argument fragments only. Anthropic's streaming has an
equivalent `input_json_delta` event type.

## 6.8 Context management as the loop runs long

Around turn 15–20 on a real task, context becomes the binding constraint. Four
techniques, in increasing order of aggression:

**1. Result truncation.** Cap tool results (e.g. 4 KB) with an explicit marker:
`"... [truncated, 38 KB total; call read_file with an offset for more]"`. Telling
the model *how* to get the rest keeps the capability.

**2. Result eviction.** Once a tool result has been consumed (the model moved on),
replace its content with a stub: `"[result of read_file('index.html'), 12 KB,
elided]"`. Keeps the structural record, frees the tokens.

**3. Summarisation / compaction.** When context crosses a threshold, ask the model
to summarise turns 1..n into a compact state note, then restart the message list
with `[original user goal, summary, recent 3 turns]`. This is what long-running
coding agents do.

**4. Externalise state to a tool.** Instead of holding everything in context, give
the agent a scratchpad tool (`write_note` / `read_note`) or let it write files.
The filesystem becomes the memory; context holds only the working set. For the
website builder in chapter 9 this falls out naturally — the agent doesn't need
the HTML in context because it can `read_file` when needed.

## 6.9 Putting it together — a production-shaped loop

```python
import json, time, uuid
from anthropic import Anthropic
from toolkit import registry

client = Anthropic()


def run_agent(goal: str, system: str, model="claude-sonnet-4-5",
              budget: Budget | None = None, on_event=lambda **k: None) -> dict:
    budget = budget or Budget()
    run_id = str(uuid.uuid4())[:8]
    messages = [{"role": "user", "content": goal}]
    transcript = []

    while True:
        stop = budget.check()
        if stop:
            messages.append({"role": "user", "content":
                f"[system] Budget exhausted ({stop}). Stop calling tools. "
                f"Summarise what you completed and what remains."})
            final = client.messages.create(
                model=model, max_tokens=1024, system=system,
                tools=registry.anthropic_schemas(),
                tool_choice={"type": "none"}, messages=messages)
            return {"ok": False, "reason": stop, "run_id": run_id,
                    "text": "".join(b.text for b in final.content if b.type == "text"),
                    "transcript": transcript}

        resp = client.messages.create(
            model=model, max_tokens=4096, system=system,
            tools=registry.anthropic_schemas(), messages=messages)

        budget.turns += 1
        budget.input_tokens += resp.usage.input_tokens
        messages.append({"role": "assistant", "content": resp.content})

        for b in resp.content:
            if b.type == "text" and b.text.strip():
                on_event(kind="thought", text=b.text)

        if resp.stop_reason != "tool_use":
            return {"ok": True, "run_id": run_id,
                    "text": "".join(b.text for b in resp.content if b.type == "text"),
                    "transcript": transcript,
                    "turns": budget.turns, "tokens": budget.input_tokens}

        results = []
        for b in resp.content:
            if b.type != "tool_use":
                continue
            budget.tool_calls += 1
            if loop := budget.note_call(b.name, b.input):
                results.append({"type": "tool_result", "tool_use_id": b.id,
                                "is_error": True,
                                "content": f"Error: loop detected ({loop}). "
                                           f"Change your approach or stop."})
                continue

            on_event(kind="tool_start", tool=b.name, args=b.input)
            t0 = time.monotonic()
            content, is_error = registry.call(b.name, b.input)
            ms = round((time.monotonic() - t0) * 1000)
            on_event(kind="tool_end", tool=b.name, ms=ms, is_error=is_error)

            transcript.append({"tool": b.name, "args": b.input,
                               "ms": ms, "error": is_error,
                               "result": content[:500]})
            results.append({"type": "tool_result", "tool_use_id": b.id,
                            "content": content[:8000], "is_error": is_error})

        messages.append({"role": "user", "content": results})
```

This is ~70 lines and it is a real agent runtime: bounded, observable, loop-safe,
and it degrades into a useful partial answer instead of an exception. Everything
after this chapter is about *what tools you hand it* and *what you let it do*.

➡️ Next: [07 · Information tools vs action tools](07-information-vs-action-tools.md)
