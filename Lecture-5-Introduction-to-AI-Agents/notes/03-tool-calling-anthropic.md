# 03 · The Calculator Tool — Anthropic SDK, End to End

This is the video's `CalculatorTool.java` + `ChatService.java`, rebuilt in
Python. Read it line by line; every later chapter is a variation on this file.

```bash
pip install anthropic python-dotenv
export ANTHROPIC_API_KEY=sk-ant-...
```

## 3.1 Step 1 — the plain Python function

Nothing AI about this. It's the deterministic half.

```python
# tools/calculator.py
from __future__ import annotations


class ToolError(Exception):
    """Raised by a tool for a condition the MODEL should see and react to.

    Distinguish this from a bug in your code. A ToolError is a legitimate
    outcome ("divide by zero", "city not found") that we want to hand back to
    the model so it can correct itself. A TypeError is our bug and should
    crash loudly in development.
    """


_ALIASES = {
    "+": "add", "plus": "add", "sum": "add", "addition": "add",
    "-": "subtract", "minus": "subtract", "subtraction": "subtract", "sub": "subtract",
    "*": "multiply", "x": "multiply", "times": "multiply", "product": "multiply", "mul": "multiply",
    "/": "divide", "division": "divide", "div": "divide",
    "%": "modulo", "mod": "modulo", "remainder": "modulo",
    "^": "power", "**": "power", "exponent": "power", "pow": "power",
}


def calculate(operation: str, a: float, b: float) -> float:
    """Deterministic arithmetic. The model never sees this body."""
    op = _ALIASES.get(str(operation).strip().lower(), str(operation).strip().lower())

    if op == "add":
        return a + b
    if op == "subtract":
        return a - b
    if op == "multiply":
        return a * b
    if op == "divide":
        if b == 0:
            raise ToolError("Cannot divide by zero. Ask the user to clarify.")
        return a / b
    if op == "modulo":
        if b == 0:
            raise ToolError("Cannot take modulo by zero.")
        return a % b
    if op == "power":
        if abs(b) > 1000:
            raise ToolError("Exponent too large; refusing to compute.")
        return a ** b

    raise ToolError(
        f"Unsupported operation {operation!r}. "
        f"Supported: add, subtract, multiply, divide, modulo, power."
    )
```

Three deliberate choices:

- **`ToolError` vs a real exception.** A `ToolError` is a *result* — it goes back
  into the conversation and the model reacts. A `TypeError` is a *bug* — it
  should crash in dev. Conflating them means real bugs get swallowed and quietly
  described to your user as "I encountered an error."
- **The alias map**, even though the schema has an `enum`. Belt and braces: models
  occasionally ignore enums when strict mode is off, and you don't want a
  `KeyError` in prod because of a casing difference.
- **Guard rails inside the tool** (`abs(b) > 1000`). The model chose the
  arguments; they are, in the strictest sense, **untrusted input**. `2 ** 10**9`
  will hang your process. Treat every tool argument as if a user typed it into a
  URL — because in effect one did, via the model.

## 3.2 Step 2 — the schema (= Spring AI's `@Tool` + `@ToolParam`)

```python
# tools/calculator.py (continued)

CALCULATOR_SCHEMA = {
    "name": "calculate",
    "description": (
        "Performs a single exact arithmetic operation on two numbers and "
        "returns the numeric result. Use this for EVERY calculation without "
        "exception, including ones that look trivial — you must never compute "
        "arithmetic yourself, because your own arithmetic is unreliable for "
        "large or unusual numbers. Supported operations are add, subtract, "
        "multiply, divide, modulo and power. It handles exactly two operands "
        "per call; for a longer expression, call it repeatedly and chain the "
        "results. It returns only a number — it has no knowledge of currencies, "
        "units or context."
    ),
    "input_schema": {
        "type": "object",
        "properties": {
            "operation": {
                "type": "string",
                "enum": ["add", "subtract", "multiply", "divide", "modulo", "power"],
                "description": (
                    "The operation to perform. Must be exactly one of the listed "
                    "lowercase values. Do not send symbols like '+' or synonyms "
                    "like 'sum'."
                ),
            },
            "a": {
                "type": "number",
                "description": (
                    "The left-hand operand. For subtract this is the minuend, "
                    "for divide the dividend (numerator), for power the base."
                ),
            },
            "b": {
                "type": "number",
                "description": (
                    "The right-hand operand. For subtract this is the subtrahend, "
                    "for divide the divisor (denominator), for power the exponent."
                ),
            },
        },
        "required": ["operation", "a", "b"],
    },
}
```

Compare with the video's Java:

```java
@Tool(description = "Performs arithmetic calculation. Supported operations are ...")
public double calculate(
    @ToolParam(description = "operation: add, subtract, ...") String operation,
    @ToolParam(description = "first number") double a,
    @ToolParam(description = "second number") double b) { ... }
```

Spring AI reflects over the annotations and builds exactly this JSON. In Python
you write the JSON yourself, which is more typing but removes all the magic —
you can see precisely what the model is being told.

Notice how much the descriptions do beyond naming:

- *"without exception, including ones that look trivial"* — this is the fix for
  the video's problem where the model skipped the calculator for `10000 / 95`.
  You can put it in the system prompt too, but putting it **in the tool
  description** is more reliable, because the description sits right next to the
  schema in the constructed prompt.
- *"For subtract this is the minuend, for divide the dividend"* — kills argument
  ordering bugs. `a` and `b` are meaningless names; without this the model has a
  real chance of computing `95 / 10000`.
- *"It returns only a number — it has no knowledge of currencies"* — a negative
  boundary that stops the model from expecting the tool to format money.

## 3.3 Step 3 — the registry

One dict that maps a model-visible name to a Python callable. This is the *only*
surface the model can reach. If a name isn't in here, it cannot be executed,
no matter what the model emits.

```python
# registry.py
from tools.calculator import calculate, CALCULATOR_SCHEMA

TOOL_SCHEMAS = [CALCULATOR_SCHEMA]

TOOL_IMPLS = {
    "calculate": calculate,
}
```

> This dict **is** your security boundary, and it's worth pausing on. In the
> video's website builder there's a long discussion about why you shouldn't give
> the model a generic `run_command` tool. The reason, stated precisely: a
> registry with `{"create_directory", "write_file", "read_file", "list_files"}`
> has no key that can delete anything, so `rm -rf` is not merely disallowed —
> it is **unrepresentable**. Chapter 10 builds on this.

## 3.4 Step 4 — the dispatcher

```python
# dispatch.py
import json
from registry import TOOL_IMPLS
from tools.calculator import ToolError


def run_tool(name: str, args: dict) -> tuple[str, bool]:
    """Execute one tool call.

    Returns (content_for_model, is_error). NEVER raises for model-caused
    problems — every failure becomes text the model can read and recover from.
    """
    fn = TOOL_IMPLS.get(name)
    if fn is None:
        # The model hallucinated a tool. Tell it so; it will usually recover.
        return (
            f"Error: no tool named {name!r} exists. "
            f"Available tools: {', '.join(TOOL_IMPLS)}.",
            True,
        )

    try:
        result = fn(**args)
    except ToolError as e:
        return f"Error: {e}", True
    except TypeError as e:
        # Wrong/missing arguments — the model's fault, recoverable.
        return f"Error: invalid arguments for {name}: {e}", True
    except Exception as e:                      # noqa: BLE001
        # Our bug or an infra failure. Log the real thing, tell the model little.
        import logging
        logging.exception("tool %s blew up", name)
        return f"Error: {name} failed unexpectedly ({type(e).__name__}).", True

    return json.dumps(result) if not isinstance(result, str) else result, False
```

**Why errors are returned, not raised:** an exception kills the loop and the user
gets nothing. A returned error string goes into the conversation, the model reads
*"Error: Cannot divide by zero"* and either corrects its arguments or explains
the problem to the user in plain language. **Errors are context, not crashes.**
This one idea is what makes agents feel robust.

**Why `fn(**args)` needs care:** `args` comes from the model. If the model sends
an extra key, `**args` raises `TypeError` — which is handled above. Never write
`eval`, `exec` or `getattr(module, name)` here; the lookup must be a dict on a
fixed set of keys.

## 3.5 Step 5 — the loop (this is the agent)

```python
# agent.py
import json
from anthropic import Anthropic
from registry import TOOL_SCHEMAS
from dispatch import run_tool

client = Anthropic()
MODEL = "claude-sonnet-4-5"          # check docs.claude.com for current IDs

SYSTEM_PROMPT = """You are a helpful assistant with access to external tools.

Rules:
1. For ANY arithmetic, always use the calculate tool — even for trivial sums.
   Never compute arithmetic yourself.
2. You may call multiple tools in sequence when a request needs several steps.
3. After receiving tool results, explain the answer in natural language.
4. Never invent numbers. If a tool fails, say so plainly."""


def chat(user_message: str, history: list | None = None, max_turns: int = 10):
    messages = list(history or [])
    messages.append({"role": "user", "content": user_message})

    for turn in range(max_turns):
        response = client.messages.create(
            model=MODEL,
            max_tokens=2048,
            system=SYSTEM_PROMPT,
            tools=TOOL_SCHEMAS,
            messages=messages,
        )

        # ① echo the assistant turn back verbatim — required by the API
        messages.append({"role": "assistant", "content": response.content})

        # ② not a tool request → we're done
        if response.stop_reason != "tool_use":
            text = "".join(b.text for b in response.content if b.type == "text")
            return text, messages

        # ③ run every tool_use block in this turn (there may be several)
        results = []
        for block in response.content:
            if block.type != "tool_use":
                continue
            print(f"  ↳ {block.name}({json.dumps(block.input)})")
            content, is_error = run_tool(block.name, block.input)
            results.append({
                "type": "tool_result",
                "tool_use_id": block.id,        # ← must match exactly
                "content": content,
                "is_error": is_error,
            })

        # ④ ALL results go back in ONE user message
        messages.append({"role": "user", "content": results})

    return "Stopped: exceeded maximum tool-calling turns.", messages


if __name__ == "__main__":
    answer, _ = chat("What is 789654 multiplied by 34576?")
    print(answer)
    print("truth:", 789654 * 34576)
```

### The four numbered lines, in detail

**① Echo the assistant turn verbatim.** `response.content` is a list of typed
blocks (`TextBlock`, `ToolUseBlock`). The SDK's Pydantic objects serialise
correctly when passed straight back, so `{"role": "assistant", "content":
response.content}` just works. Do **not** try to rebuild it by hand, and do not
drop the text block that often precedes a tool call — the API validates that
every `tool_result` has a matching `tool_use` in the previous assistant turn.

**② `stop_reason` is the loop condition.** Values you'll see:

| `stop_reason` | Meaning | Action |
|---|---|---|
| `tool_use` | Model wants tools run | Execute, append results, loop |
| `end_turn` | Model finished its answer | Return text to user |
| `max_tokens` | Hit your `max_tokens` cap | Raise the cap or ask it to be brief |
| `stop_sequence` | Hit a custom stop string | Handle per your design |

Never decide by searching the prose for the word "done".

**③ One turn can contain several tool calls.** When a model asks for the weather
in three cities, it emits three `tool_use` blocks in one assistant message. They
have no dependency on each other, so you can run them concurrently
(`asyncio.gather` / a thread pool). Iterate the list — never assume
`content[0]` is the tool call, because a text block usually comes first.

**④ All results go back in ONE user message** whose content is a *list* of
`tool_result` blocks. A very common bug is appending one message per result; the
API will reject it because the block count won't match the `tool_use` count.

> **Anthropic quirk worth knowing:** tool results are sent with `role: "user"`,
> not a dedicated `tool` role. Conceptually the "user" here is your program,
> speaking on behalf of the environment. OpenAI uses a separate `role: "tool"`
> message per call. Same idea, different encoding.

## 3.6 What you'll see when you run it

```
  ↳ calculate({"operation": "multiply", "a": 789654, "b": 34576})
789654 multiplied by 34576 equals 27,304,364,304.
truth: 27304364304
```

Exact, to the last digit, because a CPU did the multiplication. Compare with
chapter 1's demo. Same model, same question — the difference is entirely the
tool.

Add a `print` inside `calculate()` (the video's `System.out.println("Calculator
Tool Called")`) to prove it actually ran rather than being answered from memory.
Do this every time you add a tool; "did it really call the tool?" is the first
question in every debugging session.

## 3.7 Forcing / preventing tool use

```python
tool_choice={"type": "auto"}                        # default: model decides
tool_choice={"type": "any"}                         # must call some tool
tool_choice={"type": "tool", "name": "calculate"}   # must call this one
tool_choice={"type": "none"}                        # no tools this turn
```

Notes with real consequences:

- With `any` or `tool`, the API **prefills the assistant turn** to force a tool
  call, so the model produces no natural-language preamble — even if you ask for
  one. If you want both explanation and a specific tool, keep `auto` and add
  *"Use the calculate tool"* in the user message instead.
- Forced tool use isn't supported everywhere (manual extended thinking, and some
  model tiers, reject `any`/`tool` with a 400). `auto` + `strict` is the portable
  way to get schema-valid calls.
- Changing `tool_choice` between requests invalidates cached message blocks in
  prompt caching. Don't flip it per turn without a reason.

## 3.8 Making it a chat service (the video's `ChatService`)

History is just a list you own. There is no server-side session.

```python
class Conversation:
    def __init__(self):
        self.history: list = []

    def send(self, text: str) -> str:
        reply, self.history = chat(text, self.history)
        return reply


conv = Conversation()
print(conv.send("What is 15% of 2,340?"))
print(conv.send("And what if I double that?"))   # 'that' resolves via history
```

The second message works only because the whole first exchange — user turn,
assistant tool_use, tool_result, assistant answer — is re-sent. **Conversation
memory is a client-side data structure**, and its unbounded growth is your
problem to solve (chapter 12).

## 3.9 Exercises

1. Add a `sqrt` operation. Notice you must touch three places: the function, the
   `enum`, and the description. That triple-edit burden is why the next chapter's
   decorator-based registry is worth building.
2. Delete the `enum` from the schema and run 20 varied prompts. Log every
   `operation` value the model sends. This is the single most convincing
   demonstration of why enums exist.
3. Set `max_turns=1` and ask something needing two tools. Watch it fail
   gracefully — then understand why chapter 6 spends so long on budgets.

➡️ Next: [04 · The same thing with the OpenAI SDK](04-tool-calling-openai.md)
