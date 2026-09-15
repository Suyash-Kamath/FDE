# 04 · The Same Calculator with the OpenAI SDK

The concepts are identical. Only the wire format differs. Learning both makes the
*shape* of tool calling visible independently of any vendor, which is the point.

```bash
pip install openai
export OPENAI_API_KEY=sk-...
```

OpenAI now has two APIs that both support function calling:

| | Chat Completions | Responses |
|---|---|---|
| Status | Older, still fully supported | Current default for new work |
| Message container | `messages=[...]` | `input=[...]` |
| System prompt | `{"role": "system", ...}` message | `instructions="..."` param |
| Tool schema nesting | `{"type":"function","function":{...}}` | `{"type":"function", ...}` flat |
| Model's call appears as | `message.tool_calls[i]` | `response.output[i].type == "function_call"` |
| Result sent back as | `{"role":"tool","tool_call_id":...}` | `{"type":"function_call_output","call_id":...}` |
| Server-side state | none | optional via `previous_response_id` |

Learn Chat Completions first — it maps most cleanly onto the Anthropic shape and
onto the mental model in chapter 2. Then read the Responses section, because
that's where new models land.

## 4.1 Chat Completions version

```python
# openai_agent.py
import json
from openai import OpenAI
from tools.calculator import calculate, ToolError   # reuse chapter 3's function

client = OpenAI()
MODEL = "gpt-5.6"          # check developers.openai.com for current model IDs

TOOLS = [{
    "type": "function",
    "function": {
        "name": "calculate",
        "description": (
            "Performs a single exact arithmetic operation on two numbers and "
            "returns the numeric result. Use this for EVERY calculation without "
            "exception, including trivial ones — never compute arithmetic "
            "yourself. Handles exactly two operands per call; chain calls for "
            "longer expressions. Returns only a number."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "operation": {
                    "type": "string",
                    "enum": ["add", "subtract", "multiply", "divide", "modulo", "power"],
                    "description": "Exactly one of the listed lowercase values.",
                },
                "a": {"type": "number", "description": "Left operand: minuend / dividend / base."},
                "b": {"type": "number", "description": "Right operand: subtrahend / divisor / exponent."},
            },
            "required": ["operation", "a", "b"],
            "additionalProperties": False,        # ← required by strict mode
        },
        "strict": True,
    },
}]

IMPLS = {"calculate": calculate}

SYSTEM = ("You are a helpful assistant with tool access. Always use the "
          "calculate tool for arithmetic, even trivial arithmetic. After tool "
          "results arrive, explain the answer naturally.")


def run_tool(name, args) -> str:
    fn = IMPLS.get(name)
    if fn is None:
        return f"Error: unknown tool {name!r}."
    try:
        return json.dumps(fn(**args))
    except ToolError as e:
        return f"Error: {e}"
    except Exception as e:                          # noqa: BLE001
        return f"Error: {name} failed ({type(e).__name__})."


def chat(user_message: str, history=None, max_turns=10):
    messages = [{"role": "system", "content": SYSTEM}] + list(history or [])
    messages.append({"role": "user", "content": user_message})

    for _ in range(max_turns):
        completion = client.chat.completions.create(
            model=MODEL, messages=messages, tools=TOOLS,
        )
        msg = completion.choices[0].message

        # echo the assistant message back verbatim
        messages.append(msg)

        if not msg.tool_calls:
            return msg.content, messages

        for call in msg.tool_calls:
            if call.type != "function":
                continue
            args = json.loads(call.function.arguments)   # ← a JSON *string*
            print(f"  ↳ {call.function.name}({args})")
            messages.append({
                "role": "tool",                          # ← dedicated role
                "tool_call_id": call.id,                 # ← must match
                "content": run_tool(call.function.name, args),
            })

    return "Stopped: exceeded maximum turns.", messages


if __name__ == "__main__":
    print(chat("What is 789654 * 34576?")[0])
```

### Differences from Anthropic, itemised

1. **`finish_reason` / `msg.tool_calls` instead of `stop_reason`.** Idiomatic
   OpenAI code checks `if not msg.tool_calls`. `finish_reason == "tool_calls"`
   carries the same information.
2. **Arguments arrive as a JSON *string*,** not a dict. `json.loads(...)` is
   mandatory. Anthropic gives you `block.input` already parsed. This is the
   single most common porting bug between the two.
3. **One `role: "tool"` message per call,** not one user message containing a
   list of results. If the model made three calls, you append three messages.
4. **Order matters.** Tool messages must directly follow the assistant message
   that requested them, and every `tool_call_id` must be answered. Miss one and
   you get a 400.
5. **`strict: True` requires** `additionalProperties: False` **and every key
   listed in `required`.** Mark a field optional by adding `"null"` to its type:
   `{"type": ["string", "null"]}`. If your schema violates these, the request is
   rejected with details.

## 4.2 Responses API version

```python
# openai_responses_agent.py
import json
from openai import OpenAI
from tools.calculator import calculate, ToolError

client = OpenAI()
MODEL = "gpt-6-astra"        # check current model IDs in the docs

TOOLS = [{
    "type": "function",                     # ← flat, no nested "function" key
    "name": "calculate",
    "description": "Performs one exact arithmetic operation on two numbers ...",
    "parameters": {
        "type": "object",
        "properties": {
            "operation": {"type": "string",
                          "enum": ["add", "subtract", "multiply", "divide", "modulo", "power"]},
            "a": {"type": "number"},
            "b": {"type": "number"},
        },
        "required": ["operation", "a", "b"],
        "additionalProperties": False,
    },
    "strict": True,
}]


def chat(user_message: str, max_turns: int = 10) -> str:
    input_list = [{"role": "user", "content": user_message}]

    for _ in range(max_turns):
        response = client.responses.create(
            model=MODEL,
            instructions="Always use the calculate tool for arithmetic.",
            tools=TOOLS,
            input=input_list,
        )

        # carry the model's own output items forward — including reasoning items
        input_list += response.output

        calls = [i for i in response.output if i.type == "function_call"]
        if not calls:
            return response.output_text

        for call in calls:
            args = json.loads(call.arguments)
            try:
                out = json.dumps(calculate(**args))
            except ToolError as e:
                out = f"Error: {e}"
            input_list.append({
                "type": "function_call_output",
                "call_id": call.call_id,        # ← note: call_id, NOT id
                "output": out,
            })

    return "Stopped: exceeded maximum turns."
```

Responses-specific traps:

- **`call_id` ≠ `id`.** Each `function_call` item has *both*. `id` (`fc_...`)
  identifies the output item; `call_id` (`call_...`) is what you echo back. Using
  `id` produces a confusing mismatch error.
- **For reasoning models, reasoning items in the output must be passed back too,**
  alongside the tool outputs. `input_list += response.output` does this for free;
  hand-rebuilding the list does not, and you'll silently lose the model's chain
  of thought between turns.
- **`response.output_text`** is a convenience that concatenates the text items.
  Use it only when you've already confirmed there are no pending tool calls.
- **`previous_response_id`** lets the server hold the history so you only send the
  delta. Convenient — but you give up the ability to inspect, edit, truncate or
  replay the context, which you will want for debugging and for compaction.

## 4.3 Killing the boilerplate: a decorator-based registry

Writing the JSON Schema by hand for every tool gets old immediately, and the
schema drifting out of sync with the function signature is a real bug class. Both
ecosystems offer help (`pydantic_function_tool` on the OpenAI side), but a
40-line decorator gives you one registry that emits *both* wire formats.

```python
# toolkit.py
from __future__ import annotations
import inspect, json, typing
from dataclasses import dataclass, field

_PY_TO_JSON = {str: "string", int: "integer", float: "number",
               bool: "boolean", list: "array", dict: "object"}


@dataclass
class Tool:
    name: str
    description: str
    fn: typing.Callable
    params: dict = field(default_factory=dict)
    required: list = field(default_factory=list)

    def anthropic(self) -> dict:
        return {"name": self.name, "description": self.description,
                "input_schema": {"type": "object", "properties": self.params,
                                 "required": self.required}}

    def openai_chat(self) -> dict:
        return {"type": "function", "function": {
            "name": self.name, "description": self.description,
            "parameters": {"type": "object", "properties": self.params,
                           "required": self.required,
                           "additionalProperties": False},
            "strict": True}}

    def openai_responses(self) -> dict:
        return {"type": "function", "name": self.name,
                "description": self.description,
                "parameters": {"type": "object", "properties": self.params,
                               "required": self.required,
                               "additionalProperties": False},
                "strict": True}


class Registry:
    def __init__(self) -> None:
        self._tools: dict[str, Tool] = {}

    def tool(self, description: str, **param_docs):
        """Decorator. param_docs maps arg name -> description, or -> (desc, enum)."""
        def deco(fn):
            sig = inspect.signature(fn)
            props, required = {}, []
            for pname, p in sig.parameters.items():
                doc = param_docs.get(pname, "")
                enum = None
                if isinstance(doc, tuple):
                    doc, enum = doc
                jtype = _PY_TO_JSON.get(p.annotation, "string")
                schema = {"type": jtype, "description": doc}
                if enum:
                    schema["enum"] = list(enum)
                props[pname] = schema
                if p.default is inspect.Parameter.empty:
                    required.append(pname)
            self._tools[fn.__name__] = Tool(
                name=fn.__name__, description=description,
                fn=fn, params=props, required=required)
            return fn
        return deco

    # ---- schema exports -------------------------------------------------
    def anthropic_schemas(self):       return [t.anthropic() for t in self._tools.values()]
    def openai_chat_schemas(self):     return [t.openai_chat() for t in self._tools.values()]
    def openai_responses_schemas(self): return [t.openai_responses() for t in self._tools.values()]

    # ---- execution ------------------------------------------------------
    def call(self, name: str, args: dict) -> tuple[str, bool]:
        tool = self._tools.get(name)
        if tool is None:
            return f"Error: unknown tool {name!r}. Available: {', '.join(self._tools)}.", True
        try:
            result = tool.fn(**args)
        except Exception as e:                       # noqa: BLE001
            return f"Error: {e}", True
        return result if isinstance(result, str) else json.dumps(result), False


registry = Registry()
```

Usage — this is now as terse as Spring AI's annotations, with none of the magic:

```python
from toolkit import registry

@registry.tool(
    description=("Performs one exact arithmetic operation on two numbers. Use "
                 "for EVERY calculation including trivial ones."),
    operation=("The operation to perform.",
               ["add", "subtract", "multiply", "divide", "modulo", "power"]),
    a="Left operand: minuend / dividend / base.",
    b="Right operand: subtrahend / divisor / exponent.",
)
def calculate(operation: str, a: float, b: float) -> float:
    ...
```

```python
# Anthropic
client.messages.create(..., tools=registry.anthropic_schemas())
# OpenAI Chat Completions
client.chat.completions.create(..., tools=registry.openai_chat_schemas())
# OpenAI Responses
client.responses.create(..., tools=registry.openai_responses_schemas())
```

One function definition, three wire formats, zero drift between the signature
and the schema. Everything from chapter 5 onwards assumes this registry.

> **Caveat:** the toy type mapping above doesn't handle `Optional`, `Literal`,
> nested models or lists of objects. For anything real, generate the schema from
> a Pydantic model (`Model.model_json_schema()`) instead of from
> `inspect.signature`. The principle is the same; Pydantic just does the hard
> JSON-Schema cases correctly.

## 4.4 Provider-agnostic loop

Once tools are registry-backed, the loop is the only vendor-specific code left,
and it's ~30 lines per vendor. Keep two thin adapters behind one interface:

```python
class LLMBackend(typing.Protocol):
    def step(self, system: str, messages: list, schemas: list) -> "Step": ...
```

where `Step` is your normalised `(assistant_turn, tool_calls, is_done)`. Resist
the urge to abstract further. Every framework that tries to unify prompt caching,
reasoning items, parallel calls and streaming across vendors ends up leaking.
Two small adapters beat one large abstraction.

➡️ Next: [05 · Live weather and currency tools](05-weather-and-currency-tools.md)
