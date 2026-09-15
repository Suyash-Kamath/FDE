# Agentic AI Concepts in Python — Deep Explanation

Since you're going to build this in **Python**, I would not spend time reproducing the Spring AI syntax. Learn the architecture underneath it, then implement the architecture directly with the **official OpenAI Python SDK**.

The most important sentence in this entire lecture is:

> **The LLM decides what should happen. Your Python application decides what is actually allowed to happen and executes it.**

The transcript begins from exactly this distinction: a raw LLM can generate text/code, but an external capability is required to turn that generated intent into an action. The lecture’s “smart person without hands” analogy is useful: the model is the reasoning/language component, while tools provide capabilities for interacting with external systems.

---

# The entire lecture in one architecture

Forget Spring AI for a moment.

Think of an agentic application as **three layers**:

```text
                  ┌─────────────────────────┐
                  │          USER           │
                  │ "Build me a website"    │
                  └────────────┬────────────┘
                               │
                               ▼
                  ┌─────────────────────────┐
                  │   YOUR PYTHON PROGRAM   │
                  │                         │
                  │  Agent / Orchestrator   │
                  └────────────┬────────────┘
                               │
                               ▼
                  ┌─────────────────────────┐
                  │           LLM           │
                  │                         │
                  │ Understands             │
                  │ Reasons                 │
                  │ Chooses tools           │
                  │ Creates arguments       │
                  └────────────┬────────────┘
                               │
                   function call request
                               │
                               ▼
              ┌─────────────────────────────────┐
              │             TOOLS               │
              │                                 │
              │ calculate()                     │
              │ get_weather()                   │
              │ get_exchange_rate()             │
              │ create_directory()              │
              │ write_file()                    │
              │ read_file()                     │
              │ list_files()                    │
              └────────────────┬────────────────┘
                               │
                         Tool result
                               │
                               ▼
                             LLM
                               │
                         decide again
                               │
                               ▼
                             User
```

There are therefore **three different responsibilities**:

| Component | Responsibility |
|---|---|
| LLM | Understand and decide |
| Python orchestrator | Control execution |
| Tools | Actually do things |

That separation is fundamental.

---

# Why can't an LLM directly perform actions?

A raw language model fundamentally maps input to output.

Conceptually:

```text
input tokens
    ↓
Transformer
    ↓
probability distribution
    ↓
next token
    ↓
next token
    ↓
next token
    ↓
output
```

Suppose you ask:

```text
Create a folder called portfolio.
```

A plain model can produce:

```bash
mkdir portfolio
```

But generating the characters:

```text
m k d i r ...
```

doesn't create anything on your computer.

Those are just output tokens.

Likewise:

```text
Send an email to Rahul.
```

The model might generate:

```text
Sure, I've sent the email.
```

But unless something outside the model actually calls an email API, **nothing happened**.

Similarly:

```text
Refund order #93842
```

requires:

```text
database
payment provider
authorization
business rules
audit logs
```

A language model doesn't magically possess those capabilities.

---

# LLM = brain, tools = hands

This analogy is useful, but I would extend it.

```text
LLM             = Brain
Tools           = Hands
Your Python app = Nervous system + security guard
```

That third component is critical.

Because you **do not want**:

```text
LLM → unrestricted operating system
```

You want:

```text
LLM
 ↓
structured request
 ↓
your Python program
 ↓
validation
 ↓
permission check
 ↓
tool execution
```

For example, imagine the model says:

```json
{
  "name": "delete_database",
  "arguments": {
    "database": "production"
  }
}
```

That doesn't mean it happens.

Your application can say:

```python
if tool_name not in allowed_tools:
    reject()
```

or:

```python
if requires_human_approval(tool_name):
    ask_for_approval()
```

This is one reason understanding the orchestration layer matters more than memorizing an agent framework.

---

# So what exactly is a Tool?

Tools are basically normal functions.

For example:

```python
def calculate(a: float, b: float, operation: str) -> float:
    if operation == "add":
        return a + b

    if operation == "subtract":
        return a - b

    if operation == "multiply":
        return a * b

    if operation == "divide":
        return a / b

    raise ValueError("Unknown operation")
```

There's nothing "AI" about this function.

It is ordinary deterministic Python.

The AI part is:

> **Who decides when to call it and what arguments to provide?**

The LLM.

---

# Function calling does NOT mean the LLM calls Python directly

This distinction is frequently misunderstood.

Suppose the user asks:

```text
What is 8593 × 927?
```

The model has been told that this tool exists:

```text
calculate(
    a: number,
    b: number,
    operation: "add" | "subtract" | "multiply" | "divide"
)
```

Instead of answering normally, the model might produce a tool-call object conceptually like:

```json
{
  "name": "calculate",
  "arguments": {
    "a": 8593,
    "b": 927,
    "operation": "multiply"
  }
}
```

The model has **not executed anything yet**.

Your Python program receives that object.

Then:

```python
result = calculate(
    a=8593,
    b=927,
    operation="multiply",
)
```

Your code gets:

```text
7965711
```

Then your program sends that result back to the model:

```json
{
  "tool_result": 7965711
}
```

The model can finally respond:

```text
8593 × 927 = 7,965,711.
```

That is function calling.

---

# Natural language → structured tool calls

This is where LLMs become extremely useful.

Normal Python functions don't understand arbitrary human language very well.

Suppose you have:

```python
def calculate(
    a: float,
    b: float,
    operation: str,
):
    ...
```

Your deterministic program expects something such as:

```python
calculate(
    a=20,
    b=50,
    operation="add",
)
```

But humans might say:

```text
Add 20 and 50.
```

or:

```text
I had 20 apples and purchased another 50.
How many apples do I have?
```

or:

```text
If I already owned twenty apples and my mother
gave me fifty more, what's my total?
```

A traditional parser would require rules.

An LLM understands that all three imply:

```json
{
  "a": 20,
  "b": 50,
  "operation": "add"
}
```

Think of the model as doing:

```text
UNSTRUCTURED INTENT
        ↓
       LLM
        ↓
STRUCTURED INTENT
```

---

# The tool schema is a contract

You shouldn't merely tell the model:

```text
I have a calculator.
```

You need to explain its interface.

Conceptually:

```text
Tool name:
calculate

Description:
Perform exact arithmetic calculations.

Parameters:
a         → number
b         → number
operation → add | subtract | multiply | divide
```

With the official OpenAI Python SDK, a function tool looks like this:

```python
calculator_tool = {
    "type": "function",
    "name": "calculate",
    "description": "Perform an arithmetic operation on two numbers.",
    "parameters": {
        "type": "object",
        "properties": {
            "a": {
                "type": "number",
                "description": "The first number",
            },
            "b": {
                "type": "number",
                "description": "The second number",
            },
            "operation": {
                "type": "string",
                "enum": [
                    "add",
                    "subtract",
                    "multiply",
                    "divide",
                ],
                "description": "The arithmetic operation to perform",
            },
        },
        "required": [
            "a",
            "b",
            "operation",
        ],
        "additionalProperties": False,
    },
    "strict": True,
}
```

This is much stronger than prompting:

```text
Please return JSON.
```

Because this:

```json
"operation": {
    "enum": ["add", "subtract", "multiply", "divide"]
}
```

tells the model:

```text
DO NOT produce:

"operation": "+"
"operation": "sum"
"operation": "plus"
"operation": "addition"

Produce exactly one of:
add
subtract
multiply
divide
```

That's essentially a typed interface between probabilistic AI and deterministic software.

---

# Spring AI → Python/OpenAI mapping

The lecture uses Spring AI, but conceptually very little changes.

| Spring AI concept | Python/OpenAI equivalent |
|---|---|
| Java method | Python function |
| `@Tool` | OpenAI function-tool definition |
| `@ToolParam` | JSON Schema property description |
| Tool registration | `tools=[...]` |
| Tool execution | Your Python dispatcher |
| Tool result | `function_call_output` |
| Chat client | `OpenAI()` |
| Agent loop | Python `while` loop |

So don't think:

> "I need to learn Spring AI before I understand agents."

No.

Spring AI is an abstraction over these concepts.

The architecture is much more important.

---

# Your first real Python tool-calling program

Install the official SDK:

```bash
uv add openai
```

or:

```bash
pip install openai
```

Then:

```python
from openai import OpenAI

client = OpenAI()
```

Assuming:

```bash
OPENAI_API_KEY=...
```

is available in the environment.

Now define your deterministic tool:

```python
def calculate(
    a: float,
    b: float,
    operation: str,
) -> float:
    match operation:
        case "add":
            return a + b

        case "subtract":
            return a - b

        case "multiply":
            return a * b

        case "divide":
            if b == 0:
                raise ValueError("Cannot divide by zero")
            return a / b

        case _:
            raise ValueError(
                f"Unsupported operation: {operation}"
            )
```

Then describe it to the model:

```python
tools = [
    {
        "type": "function",
        "name": "calculate",
        "description": (
            "Perform exact arithmetic calculations."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "a": {
                    "type": "number",
                },
                "b": {
                    "type": "number",
                },
                "operation": {
                    "type": "string",
                    "enum": [
                        "add",
                        "subtract",
                        "multiply",
                        "divide",
                    ],
                },
            },
            "required": [
                "a",
                "b",
                "operation",
            ],
            "additionalProperties": False,
        },
        "strict": True,
    }
]
```

Notice something subtle:

```python
def calculate(...)
```

and:

```python
{
    "name": "calculate",
    "parameters": ...
}
```

are **two different things**.

The first is executable Python.

The second is a description of that capability presented to the model.

Your application connects them.

---

# The most important piece: the tool loop

Here is the architectural core:

```python
import json

from openai import OpenAI

client = OpenAI()
```

We'll have a registry:

```python
TOOL_FUNCTIONS = {
    "calculate": calculate,
}
```

Then:

```python
input_items = [
    {
        "role": "user",
        "content": "What is 8593 multiplied by 927?",
    }
]
```

Call the model:

```python
response = client.responses.create(
    model="gpt-5.6",
    input=input_items,
    tools=tools,
)
```

Now preserve the model's output:

```python
input_items += response.output
```

Then inspect it:

```python
for item in response.output:
    if item.type != "function_call":
        continue

    arguments = json.loads(item.arguments)

    function = TOOL_FUNCTIONS[item.name]

    result = function(**arguments)

    input_items.append(
        {
            "type": "function_call_output",
            "call_id": item.call_id,
            "output": json.dumps(
                {
                    "result": result
                }
            ),
        }
    )
```

Then send everything back:

```python
response = client.responses.create(
    model="gpt-5.6",
    input=input_items,
    tools=tools,
)
```

Finally:

```python
print(response.output_text)
```

---

# Why do we need the second LLM call?

Consider this sequence.

## First model call

User:

```text
What is 8593 × 927?
```

LLM:

```text
I need the calculator.
```

Structured output:

```json
calculate(
  a=8593,
  b=927,
  operation="multiply"
)
```

The model doesn't yet know what your Python function returned.

## Tool execution

Python:

```python
7965711
```

## Second model call

Now the model receives:

```text
User wanted:
8593 × 927

I requested:
calculate(...)

Tool returned:
7965711
```

It can produce:

```text
8593 × 927 = 7,965,711.
```

So function calling is fundamentally a **conversation between the model and the runtime**.

---

# The complete loop

A real system shouldn't assume there will only be one tool call.

Use:

```text
while model keeps asking for tools:
    execute tools
    return observations

when model stops asking:
    return final answer
```

Conceptually:

```python
while True:

    response = call_llm()

    tool_calls = find_tool_calls(response)

    if not tool_calls:
        return response.output_text

    for tool_call in tool_calls:
        result = execute(tool_call)
        send_result_back(result)
```

A more realistic implementation:

```python
import json

from openai import OpenAI

client = OpenAI()


def run_agent(user_message: str) -> str:
    input_items = [
        {
            "role": "user",
            "content": user_message,
        }
    ]

    max_turns = 10

    for _ in range(max_turns):

        response = client.responses.create(
            model="gpt-5.6",
            input=input_items,
            tools=tools,
        )

        input_items += response.output

        tool_calls = [
            item
            for item in response.output
            if item.type == "function_call"
        ]

        if not tool_calls:
            return response.output_text

        for call in tool_calls:
            try:
                arguments = json.loads(
                    call.arguments
                )

                function = TOOL_FUNCTIONS[
                    call.name
                ]

                result = function(**arguments)

                output = {
                    "ok": True,
                    "result": result,
                }

            except Exception as exc:
                output = {
                    "ok": False,
                    "error": str(exc),
                }

            input_items.append(
                {
                    "type": "function_call_output",
                    "call_id": call.call_id,
                    "output": json.dumps(output),
                }
            )

    raise RuntimeError(
        "Agent exceeded maximum number of turns."
    )
```

That little loop contains the essence of a huge amount of what is marketed as "agentic AI."

---

# What is actually happening inside that loop?

We can represent the state transitions as:

```text
STATE 1
User request
     ↓

STATE 2
LLM decision
     ↓

STATE 3A
Final answer
     ↓
    END

OR

STATE 3B
Tool request
     ↓

STATE 4
Application validates request
     ↓

STATE 5
Application executes tool
     ↓

STATE 6
Observation returned to LLM
     ↓

back to STATE 2
```

The magic is not really:

```text
LLM + function
```

It is:

```text
LLM
 ↓
decision
 ↓
action
 ↓
observation
 ↓
LLM
 ↓
new decision
```

That's the foundation of agents.

---

# Calculator tool: one subtle correction

The statement “LLMs cannot calculate; they merely predict” is useful pedagogically, but slightly oversimplified.

Modern reasoning models can often perform arithmetic and algorithmic reasoning internally.

The engineering principle is still correct:

> When the answer needs to be exact and deterministic, use a deterministic tool.

Why?

The model is probabilistic.

The calculator:

```python
8593 * 927
```

is deterministic.

Same input:

```text
8593, 927
```

always gives:

```text
7965711
```

So think:

```text
LLM:
"What needs to be calculated?"

Calculator:
"What is the exact result?"
```

That division of labor is excellent software design.

---

# Live weather tool

Suppose the user says:

```text
What's the weather in Mumbai right now?
```

The model itself shouldn't invent current weather.

Instead, give it something like:

```python
def get_weather(city: str) -> dict:
    ...
```

That function could internally call:

```text
Weather provider API
```

and return:

```json
{
  "city": "Mumbai",
  "temperature_c": 29,
  "humidity": 81,
  "condition": "Cloudy"
}
```

The model sees:

```text
temperature = 29
humidity = 81
condition = cloudy
```

and turns that into natural language.

So we now have another very useful separation:

```text
External API
      ↓
 structured facts
      ↓
     LLM
      ↓
human-friendly explanation
```

The LLM should **not** be your database.

The LLM should **not** be your weather provider.

The LLM should **not** be your exchange-rate provider.

It is the reasoning/orchestration layer.

---

# Currency conversion tool

Imagine:

```python
def get_exchange_rate(
    from_currency: str,
    to_currency: str,
) -> float:
    ...
```

The model might call:

```json
{
  "from_currency": "INR",
  "to_currency": "USD"
}
```

The API might return:

```text
0.0114
```

If the user asked:

```text
How many USD will I receive for ₹10,000?
```

the agent now knows:

```text
INR → USD rate = 0.0114
```

But it still needs:

```text
10000 × 0.0114
```

What happens?

It can call another tool.

---

# Multiple tool calls

User:

```text
I have ₹10,000.
How many US dollars will I get based on today's rate?
```

The model can't answer reliably without today's exchange rate.

So:

```text
User
 ↓

LLM
 ↓
"I need today's INR → USD rate."
 ↓

get_exchange_rate(
    from="INR",
    to="USD"
)
 ↓

0.0114
 ↓

LLM
 ↓
"I now need 10000 × 0.0114."
 ↓

calculate(
    a=10000,
    b=0.0114,
    operation="multiply"
)
 ↓

114
 ↓

LLM
 ↓

"You would receive approximately $114."
```

Notice something profound.

The original user never said:

```text
Step 1: Call currency API.
Step 2: Extract rate.
Step 3: Call calculator.
Step 4: Multiply.
Step 5: explain.
```

They gave a **goal**.

The system derived the intermediate steps.

That's where agent-like behavior begins.

---

# Sequential vs parallel tool calls

These are different.

Suppose:

```text
What's the weather in Mumbai, Delhi and Bengaluru?
```

The calls are independent:

```text
get_weather("Mumbai")
get_weather("Delhi")
get_weather("Bengaluru")
```

Potentially:

```text
              ┌─ Mumbai
LLM ──────────┼─ Delhi
              └─ Bengaluru
```

They can potentially happen in parallel.

But:

```text
Convert ₹10,000 to USD at today's rate.
```

has a dependency:

```text
get exchange rate
       ↓
need the result
       ↓
calculate converted amount
```

That's sequential.

The model can't calculate the second tool arguments correctly until it observes the first result.

---

# Tool calling is not the same as an agent

Suppose you build:

```text
User asks arithmetic question
       ↓
model chooses calculator
       ↓
calculator runs
       ↓
model gives answer
```

That's tool calling.

You could call it an agent if you want, but it's closer to a **tool-enabled workflow**.

---

# Workflow vs Agent

Consider this system:

```text
1. Read document
2. Summarize document
3. Save summary
```

Every step was predetermined by a programmer.

```text
A → B → C
```

That's a workflow.

Now consider:

```text
Goal:
"Build me a portfolio website."
```

Available capabilities:

```text
create_directory()
write_file()
read_file()
list_files()
```

Nobody explicitly tells the system:

```text
Create portfolio/
Create index.html
Create styles.css
Create script.js
Read index.html
Inspect styles.css
Fix missing stylesheet link
...
```

Instead, after each observation it decides:

```text
What should I do next?
```

That's much more agentic.

A useful spectrum is:

```text
Fixed code
    ↓
LLM call
    ↓
LLM + one tool
    ↓
LLM + multiple predetermined tools
    ↓
LLM chooses tools
    ↓
LLM chooses repeated actions
    ↓
LLM observes results and replans
    ↓
Autonomous agent
```

Agentic systems are not a magical separate species of AI.

They're largely **LLMs embedded in control loops**.

---

# Goal → Decide → Act → Observe

Imagine:

```text
GOAL
Build a portfolio website.

        ↓

DECIDE
I need a directory.

        ↓

ACT
create_directory("portfolio")

        ↓

OBSERVE
Directory successfully created.

        ↓

DECIDE
I need index.html.

        ↓

ACT
write_file(
    "portfolio/index.html",
    "..."
)

        ↓

OBSERVE
File written successfully.

        ↓

DECIDE
I need styling.

        ↓

ACT
write_file(
    "portfolio/styles.css",
    "..."
)

        ↓

OBSERVE
File created.

        ↓

DECIDE
I should inspect index.html.

        ↓

ACT
read_file("portfolio/index.html")

        ↓

OBSERVE
HTML doesn't link styles.css.

        ↓

DECIDE
Fix it.

        ↓

ACT
write_file(...)

        ↓

OBSERVE
Fixed.

        ↓

DECIDE
Goal complete.

        ↓

FINAL ANSWER
Website created.
```

That's an agent loop.

---

# Why observation matters

Without observation, this:

```text
LLM → action → action → action → action
```

is dangerous.

Consider:

```python
create_directory("portfolio")
```

What if it returns:

```json
{
  "ok": false,
  "error": "Directory already exists"
}
```

The agent needs to see the failure.

It might then decide:

```text
I'll list the existing directory.
```

and call:

```python
list_files("portfolio")
```

Observation lets the model react to the actual world rather than hallucinating what happened.

This is one of the differences between merely **generating a plan** and **executing a plan**.

---

# Your Python agent architecture

```text
┌──────────────────────────────────────────────┐
│                FastAPI / CLI                 │
│                User request                  │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│              Agent Orchestrator              │
│                                              │
│ • conversation state                         │
│ • tool registry                              │
│ • max iterations                             │
│ • permissions                                │
│ • approvals                                  │
│ • error handling                             │
│ • tracing/logging                            │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│                OpenAI Model                  │
│                                              │
│ Understand goal                              │
│ Choose next action                           │
│ Produce structured arguments                 │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│                  Tool Layer                  │
│                                              │
│ Calculator                                   │
│ Weather                                      │
│ Currency                                     │
│ Filesystem                                   │
│ Database                                     │
│ Email                                        │
│ Search                                       │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│                  Environment                 │
│                                              │
│ Files / APIs / DB / services / network       │
└──────────────────────────────────────────────┘
```

This architecture is far more important for you as an AI-backend engineer than memorizing a LangChain class name.

---

# Information tools vs action tools

An information tool might be:

```python
get_weather()
get_exchange_rate()
search_products()
read_file()
lookup_order()
calculate()
```

Generally:

```text
World → tool → information
```

An action tool might be:

```python
send_email()
cancel_order()
refund_payment()
delete_user()
write_file()
deploy_application()
transfer_money()
```

Generally:

```text
Agent → tool → world changes
```

That's a major security difference.

---

# A strong permission model

A production agent might assign tool risk like this:

| Tool | Risk | Automatic? |
|---|---:|---|
| `calculate` | Very low | Yes |
| `get_weather` | Very low | Yes |
| `read_file` | Low | Usually |
| `write_file` inside sandbox | Medium | Possibly |
| `send_email` | Medium/high | Approval |
| `delete_file` | High | Approval |
| `deploy_production` | Very high | Approval |
| `refund_payment` | Very high | Approval |
| `transfer_money` | Extremely high | Strong approval |

The important lesson:

> **Permissions belong in application code, not merely in the system prompt.**

This is weak:

```text
SYSTEM:
Never delete important files.
```

The model can still make a mistake.

This is strong:

```python
ALLOWED_TOOLS = {
    "read_file",
    "write_file",
    "list_files",
    "create_directory",
}
```

No:

```text
delete_file
```

exists.

Therefore the model literally cannot call it.

That is **capability-based security**.

---

# Why the website-builder example matters

A naïve implementation might give the model:

```python
run_terminal(command: str)
```

Then the model can theoretically generate:

```bash
mkdir ...
rm ...
curl ...
ssh ...
chmod ...
cat ~/.ssh/id_rsa
```

That's horrifying from a security standpoint.

A much better design is:

```text
create_directory()
write_file()
read_file()
list_files()
```

And nothing else.

This is **least privilege**.

---

# Building the filesystem tools in Python

Let's create a workspace:

```text
project/
│
├── main.py
│
├── agent/
│   ├── orchestrator.py
│   └── tools.py
│
└── generated-sites/
```

Every agent-generated website must stay inside:

```text
generated-sites/
```

Not:

```text
~/Desktop
/
~/.ssh
/etc
```

---

# Why path traversal is dangerous

Suppose you expose:

```python
read_file(path: str)
```

The model generates:

```text
../../../../etc/passwd
```

Naïve implementation:

```python
with open(path) as file:
    ...
```

You just allowed the tool to escape its workspace.

Or:

```text
../../.env
```

could potentially expose secrets.

This attack/bug is commonly called **path traversal**.

---

# A Python sandbox path helper

Using `pathlib`:

```python
from pathlib import Path

WORKSPACE = Path(
    "./generated-sites"
).resolve()

WORKSPACE.mkdir(
    parents=True,
    exist_ok=True,
)


def safe_path(relative_path: str) -> Path:
    requested = Path(relative_path)

    if requested.is_absolute():
        raise PermissionError(
            "Absolute paths are not allowed."
        )

    resolved = (
        WORKSPACE / requested
    ).resolve()

    try:
        resolved.relative_to(WORKSPACE)
    except ValueError:
        raise PermissionError(
            "Path escapes the workspace."
        )

    return resolved
```

So:

```python
safe_path("portfolio/index.html")
```

becomes something such as:

```text
/.../generated-sites/portfolio/index.html
```

Fine.

But:

```python
safe_path("../../.env")
```

fails.

---

# createDirectory

```python
def create_directory(path: str) -> dict:
    directory = safe_path(path)

    directory.mkdir(
        parents=True,
        exist_ok=True,
    )

    return {
        "ok": True,
        "path": str(
            directory.relative_to(WORKSPACE)
        ),
    }
```

---

# writeFile

```python
def write_file(
    path: str,
    content: str,
) -> dict:

    file_path = safe_path(path)

    file_path.parent.mkdir(
        parents=True,
        exist_ok=True,
    )

    file_path.write_text(
        content,
        encoding="utf-8",
    )

    return {
        "ok": True,
        "path": str(
            file_path.relative_to(WORKSPACE)
        ),
        "bytes_written": len(
            content.encode("utf-8")
        ),
    }
```

---

# readFile

```python
def read_file(path: str) -> dict:
    file_path = safe_path(path)

    if not file_path.is_file():
        raise FileNotFoundError(path)

    content = file_path.read_text(
        encoding="utf-8"
    )

    return {
        "ok": True,
        "path": str(
            file_path.relative_to(WORKSPACE)
        ),
        "content": content,
    }
```

---

# listFiles

```python
def list_files(path: str = ".") -> dict:
    directory = safe_path(path)

    if not directory.is_dir():
        raise NotADirectoryError(path)

    entries = []

    for item in directory.iterdir():
        entries.append(
            {
                "name": item.name,
                "type": (
                    "directory"
                    if item.is_dir()
                    else "file"
                ),
            }
        )

    return {
        "ok": True,
        "entries": entries,
    }
```

---

# Notice what I intentionally did NOT create

There is no:

```python
delete_file()
```

No:

```python
run_shell()
```

No:

```python
execute_python()
```

No:

```python
install_package()
```

No:

```python
read_any_path()
```

That's not a limitation.

That's good design.

The safe question when designing an agent is not:

> "How many tools can I give it?"

It is:

> **"What is the minimum capability necessary to achieve the goal?"**

---

# Sandboxing is deeper than checking paths

The `safe_path()` function is useful, but do not confuse it with a complete production sandbox.

Production execution may additionally need:

```text
Filesystem isolation
Network isolation
CPU limits
Memory limits
Execution timeouts
Process isolation
Credential isolation
Package restrictions
OS permissions
Rate limits
Audit logs
```

For a local learning project:

```text
safe_path()
+
tool allowlist
```

is excellent.

For production:

```text
container / VM sandbox
+
restricted filesystem
+
restricted network
+
restricted credentials
+
tool allowlist
+
permissions
+
approvals
+
logging
```

is the right mindset.

---

# Agent tools should be narrow

Bad:

```python
def execute(command: str):
    os.system(command)
```

Good:

```python
def create_directory(path: str):
    ...

def write_file(path: str, content: str):
    ...

def read_file(path: str):
    ...

def list_files(path: str):
    ...
```

This principle is called **least privilege**.

You should expose capabilities, not universal backdoors.

---

# Human-in-the-loop

Suppose your agent has:

```python
send_email()
```

User says:

```text
Write an angry email to my manager explaining
everything and send it.
```

Should the model immediately send it?

Probably not.

Instead:

```text
LLM decides:
send_email(...)

        ↓

Policy layer:
This action requires approval.

        ↓

User sees:

To: ...
Subject: ...
Body: ...

Send?
        ↓

User approves
        ↓

Python executes send_email()
```

That's human-in-the-loop.

---

# Why HITL belongs between decision and execution

Remember:

```text
Goal
 ↓
Decide
 ↓
Act
 ↓
Observe
```

With approval:

```text
Goal
 ↓
Decide
 ↓
PROPOSE ACTION
 ↓
Permission / approval
 ↓
Act
 ↓
Observe
```

This is far safer.

---

# Example permission layer

```python
TOOL_POLICIES = {
    "calculate": "auto",
    "get_weather": "auto",
    "read_file": "auto",
    "list_files": "auto",
    "write_file": "auto",
    "send_email": "approval",
    "refund_order": "approval",
    "delete_file": "approval",
}
```

Then:

```python
policy = TOOL_POLICIES[call.name]

if policy == "approval":
    return {
        "status": "approval_required",
        "tool": call.name,
        "arguments": arguments,
    }
```

Only after the user approves:

```python
result = function(**arguments)
```

This means:

```text
LLM decides ≠ LLM is authorized
```

Remember that sentence.

---

# Information tools and action tools should have different safeguards

Imagine customer support.

Information:

```python
lookup_order("ORD-123")
```

Result:

```json
{
  "status": "delayed",
  "amount": 899
}
```

Usually okay.

But:

```python
refund_order(
    order_id="ORD-123",
    amount=899
)
```

changes money.

You should probably require:

```text
Authentication
Authorization
Business-rule validation
Approval
Idempotency
Audit logging
```

particularly for consequential actions.

---

# Idempotency becomes extremely important with agents

Imagine:

```text
LLM:
refund order 123

API:
timeout
```

The model doesn't know whether the refund happened.

So it retries:

```text
refund order 123
```

Potentially two refunds.

Therefore an action tool might accept:

```python
refund_order(
    order_id="123",
    idempotency_key="agent-run-8291-refund-123",
)
```

Then the backend ensures:

```text
same request + same idempotency key
=
one effect
```

This is where backend engineering becomes very relevant to AI engineering.

---

# What exactly makes something an AI Agent?

A practical definition:

> **An AI agent is a software system in which an AI model is given a goal, state/context, a set of permitted actions, and observations from the environment, and can iteratively choose subsequent actions until it reaches a stopping condition.**

So:

```text
Goal
+
Model
+
Tools
+
State
+
Environment
+
Observations
+
Loop
+
Stopping condition
=
Practical agent architecture
```

A single:

```python
response = client.responses.create(...)
```

isn't very agentic.

Even:

```text
Question → calculator → answer
```

is barely agentic.

But:

```text
"Build me a website"

→ inspect workspace
→ create directory
→ create HTML
→ create CSS
→ inspect HTML
→ notice problem
→ rewrite HTML
→ inspect files
→ determine completion
```

is substantially more agentic.

---

# The website builder agent

User:

```text
Build me a beautiful portfolio website for
a backend engineer specializing in Python,
FastAPI and AI systems.
```

Available tools:

```text
create_directory
write_file
read_file
list_files
```

System instruction might conceptually say:

```text
You are a frontend website-building agent.

Build complete websites using the provided
filesystem tools.

All work must be performed inside the workspace.

For each project:
- create a dedicated directory
- create index.html
- create styles.css
- create script.js when appropriate
- inspect your output
- fix problems you discover
- only finish when the website is complete
```

---

# What the model might internally choose to do

You do **not** hardcode:

```python
create_directory(...)
write_file(...)
write_file(...)
write_file(...)
read_file(...)
```

because then you built a workflow.

Instead, you expose capabilities:

```python
tools = [
    CREATE_DIRECTORY_SCHEMA,
    WRITE_FILE_SCHEMA,
    READ_FILE_SCHEMA,
    LIST_FILES_SCHEMA,
]
```

and give a goal.

The model chooses the sequence.

That's the key architectural difference.

---

# Tool registry

Your Python application might contain:

```python
TOOL_FUNCTIONS = {
    "create_directory": create_directory,
    "write_file": write_file,
    "read_file": read_file,
    "list_files": list_files,
}
```

The model does not receive Python function objects.

It receives **schemas describing them**.

Your runtime owns:

```python
TOOL_FUNCTIONS
```

The model sees:

```python
tools
```

Keep those two concepts separate in your head.

---

# The generic website agent loop

Architecturally:

```python
def run_agent(goal: str) -> str:

    conversation = [
        {
            "role": "user",
            "content": goal,
        }
    ]

    for iteration in range(20):

        response = client.responses.create(
            model="gpt-5.6",
            instructions=SYSTEM_PROMPT,
            input=conversation,
            tools=TOOLS,
        )

        conversation += response.output

        calls = [
            item
            for item in response.output
            if item.type == "function_call"
        ]

        if not calls:
            return response.output_text

        for call in calls:

            args = json.loads(
                call.arguments
            )

            result = execute_tool(
                call.name,
                args,
            )

            conversation.append(
                {
                    "type":
                        "function_call_output",

                    "call_id":
                        call.call_id,

                    "output":
                        json.dumps(result),
                }
            )

    raise RuntimeError(
        "Maximum agent iterations exceeded"
    )
```

That's your agent.

Not:

```python
AgentMagicFramework()
```

The loop is the agentic architecture.

---

# Why maximum iterations matter

Imagine the agent goes:

```text
read_file
 ↓
write_file
 ↓
read_file
 ↓
write_file
 ↓
read_file
 ↓
write_file
 ↓
...
```

forever.

Without:

```python
max_turns = 20
```

you can create:

```text
infinite loops
API cost explosion
repeated side effects
hanging requests
```

So every production agent needs a **termination budget**.

Potential budgets include:

```text
Maximum LLM turns
Maximum tool calls
Maximum execution time
Maximum token usage
Maximum API cost
Maximum repeated failures
```

---

# Tools can fail — and failures are observations

Suppose:

```python
write_file(...)
```

fails:

```json
{
  "ok": false,
  "error": "Permission denied"
}
```

Don't hide this from the model.

Return it.

Then:

```text
Agent:
"Oh. The write failed."

What next?
```

Maybe it chooses another valid path.

So your tool layer should return structured failures rather than crashing the entire orchestration process for every recoverable error.

Example:

```python
try:
    result = function(**arguments)

    output = {
        "ok": True,
        "result": result,
    }

except Exception as exc:
    output = {
        "ok": False,
        "error": {
            "type": type(exc).__name__,
            "message": str(exc),
        },
    }
```

Observation isn't only successful information.

Errors are observations too.

---

# This leads directly to ReAct-style thinking

You may encounter the term:

```text
ReAct
```

which broadly refers to:

```text
Reason
+
Act
+
Observe
+
Reason again
```

Don't obsess over the name.

The practical mechanism is what matters:

```text
Model chooses action
       ↓
Environment responds
       ↓
Model sees result
       ↓
Model chooses next action
```

You don't need access to the model's private reasoning for this.

Your application only needs its externally visible decisions, calls, outputs, statuses and observations.

---

# Tool selection

Imagine the agent knows:

```text
calculate
get_weather
get_exchange_rate
read_file
write_file
```

User asks:

```text
What's the weather in Bengaluru?
```

The model routes to:

```text
weather question
→ get_weather
```

User:

```text
Create styles.css containing this CSS.
```

```text
filesystem modification
→ write_file
```

User:

```text
Convert ₹50,000 to dollars using today's rate.
```

```text
needs current rate
→ get_exchange_rate
→ calculate
```

Descriptions strongly affect this routing.

That's why:

```python
"description": "Do stuff"
```

is terrible.

Whereas:

```python
"description": (
    "Retrieve the latest available exchange rate "
    "between two ISO 4217 currency codes. "
    "Use this whenever current exchange-rate "
    "information is required."
)
```

is much better.

---

# Don't make overlapping tools unnecessarily

Suppose you expose:

```text
get_weather
lookup_weather
find_weather
weather_search
fetch_weather
```

The model now has to distinguish five nearly identical tools.

Better:

```text
get_weather
```

with a precise schema.

Agent engineering includes **designing good APIs for the model**.

It's almost like you're designing an SDK for a junior engineer.

Names should be obvious.

Parameters should be constrained.

Errors should be informative.

Effects should be predictable.

---

# Tools are APIs for AI models

When humans use an API:

```python
stripe.refunds.create(...)
```

they read documentation.

When an LLM uses your tool:

```text
tool name
description
parameter schema
```

is its documentation.

Therefore good tool design follows familiar API principles:

```text
Clear naming
Small surface area
Typed inputs
Typed outputs
Validation
Idempotency
Explicit errors
Stable behavior
Authorization
Observability
```

---

# Tool schema validation is not enough

Suppose your schema says:

```json
{
  "amount": {
    "type": "number"
  }
}
```

The model passes:

```json
{
  "amount": 900000000
}
```

Valid JSON.

Valid schema.

But perhaps users may only refund ₹10,000.

So you still need domain validation:

```python
if amount > 10_000:
    raise PermissionError(
        "Refund exceeds automatic limit"
    )
```

Think in layers:

```text
JSON schema validation
        ↓
application validation
        ↓
authorization
        ↓
business rules
        ↓
human approval if required
        ↓
execution
```

Never assume:

```text
strict schema == safe action
```

It doesn't.

---

# Prompt instructions are not security boundaries

Suppose your system prompt says:

```text
Never access files outside generated-sites.
```

That's useful behavioral guidance.

But this is stronger:

```python
candidate = safe_path(model_path)
```

And stronger still:

```text
agent runs inside isolated container
with only /workspace mounted
```

The hierarchy is roughly:

```text
Prompt restriction
        ↓ weaker

Application validation
        ↓

OS permissions
        ↓

Sandbox/container isolation
        ↓ stronger
```

Prompts help behavior.

Code enforces policy.

---

# Agent memory/state

An agent must know what has already happened.

Example:

```text
Turn 1:
Created portfolio/

Turn 2:
Created index.html

Turn 3:
Created styles.css
```

If every model request forgot previous results, it could repeatedly recreate things.

State can include:

```text
User messages
Model outputs
Tool calls
Tool results
Current task status
Project metadata
Approval state
```

---

# A real agent has two kinds of state

## Conversation state

```text
User:
Build website.

Agent:
Created directory.

Tool:
Success.

Agent:
Created HTML.
```

This helps the LLM reason.

## Application state

```python
job_id
user_id
workspace_id
created_at
status
tool_call_count
approval_status
token_usage
```

This belongs in your backend/database.

Do not shove every piece of application state into the prompt.

---

# Long-running agents

Imagine:

```text
Research 100 companies and prepare a report.
```

That might involve:

```text
100 searches
50 page reads
dozens of model calls
file creation
database updates
```

You wouldn't want one FastAPI request hanging indefinitely.

Then backend architecture enters:

```text
POST /agents/jobs
        ↓
create job
        ↓
queue
        ↓
worker
        ↓
agent loop
        ↓
persist checkpoints
        ↓
GET /agents/jobs/{id}
```

Potential infrastructure:

```text
FastAPI
PostgreSQL
Redis
Celery / Dramatiq / custom workers
OpenAI
object storage
observability
```

This is where "AI backend engineer" becomes much more than:

```python
client.responses.create(...)
```

---

# An agent becomes a distributed-systems problem once it gets serious

You eventually need to handle:

```text
Retries
Timeouts
Partial failures
Idempotency
Concurrency
Rate limiting
Queueing
State persistence
Cancellation
Recovery
Authentication
Authorization
Audit logs
Cost tracking
Observability
```

The LLM is only one component.

For example:

```text
Agent
 ↓
weather API unavailable
 ↓
retry?
 ↓
backoff?
 ↓
fallback provider?
 ↓
tell model?
 ↓
fail job?
```

That's backend engineering.

---

# Where FastAPI fits

FastAPI itself is **not the agent**.

FastAPI is your application interface.

For example:

```text
Frontend
   ↓
POST /api/agent
   ↓
FastAPI
   ↓
Agent service
   ↓
OpenAI model
   ↓
Tools
```

Something like:

```python
@app.post("/api/agent")
async def run_agent(request: AgentRequest):
    result = await agent_service.run(
        request.message
    )

    return {
        "result": result
    }
```

Inside:

```text
agent_service
```

lives your orchestration loop.

That's a clean separation.

---

# Recommended project structure

```text
agent_demo/
│
├── pyproject.toml
├── .env
│
├── app/
│   ├── main.py
│   │
│   ├── agent/
│   │   ├── orchestrator.py
│   │   ├── prompts.py
│   │   ├── registry.py
│   │   └── schemas.py
│   │
│   ├── tools/
│   │   ├── calculator.py
│   │   ├── weather.py
│   │   ├── currency.py
│   │   └── filesystem.py
│   │
│   ├── security/
│   │   ├── permissions.py
│   │   ├── sandbox.py
│   │   └── approvals.py
│   │
│   └── api/
│       └── routes.py
│
└── generated-sites/
```

Now you're learning actual architecture rather than stuffing everything into:

```text
main.py
```

---

# Spring AI vs what you should learn

```text
LECTURE TERM                 YOUR PYTHON CONCEPT

Spring AI                    OpenAI Python SDK

ChatClient                   OpenAI client

@Tool                        Python function
                             + function schema

@ToolParam                   JSON Schema property

calculator method            calculate()

weather service              get_weather()

currency service             get_exchange_rate()

tool execution               Python dispatcher

Spring-managed tool loop     Your orchestration loop

website tools                pathlib/file functions

agent                        LLM + tools + loop + state

sandbox                      constrained filesystem/runtime

HITL                         approval gate
```

You do **not** need to learn Spring AI to understand any of those concepts.

---

# Recommended learning order

1. **Official OpenAI Python SDK + raw function calling.**  
   Implement calculator, weather, and currency tools yourself and understand the call/result loop completely.

2. **Build the filesystem website agent manually.**  
   Add `create_directory`, `write_file`, `read_file`, `list_files`, sandboxing, max turns, errors, permissions, and an approval mechanism.

3. **Only after that**, explore a higher-level agent framework or official agent tooling.  
   Then when a framework offers concepts such as agents, runners, handoffs, tools, guardrails, sessions, or tracing, you'll know what machinery it is abstracting.

That order is much better if your goal is to become strong at **Python backend + AI engineering**, because the raw implementation forces you to understand orchestration rather than merely learning framework syntax.

---

# The deepest mental model to retain

Everything in this lecture can be compressed into this:

```text
                    USER GOAL
                       │
                       ▼
                ┌──────────────┐
                │     LLM      │
                │              │
                │  DECISION    │
                └──────┬───────┘
                       │
             structured intention
                       │
                       ▼
               ┌───────────────┐
               │ ORCHESTRATOR  │
               │               │
               │ validate      │
               │ authorize     │
               │ approve       │
               │ dispatch      │
               └───────┬───────┘
                       │
                       ▼
                 ┌───────────┐
                 │   TOOL    │
                 │           │
                 │ performs  │
                 │ action    │
                 └─────┬─────┘
                       │
                    result
                       │
                       ▼
                  OBSERVATION
                       │
                       ▼
                     LLM
                       │
                  decide again
                       │
                 ┌─────┴─────┐
                 │           │
               ACT        COMPLETE
                 │           │
                 └── LOOP ───┘
```

And perhaps the most important distinction is:

```text
LLM output:
"I want tool X called with arguments Y."

IS NOT

Tool X has been executed.
```

Between those two statements lives your **backend application**.

That layer is responsible for security, correctness, permissions, retries, errors, state, observability, and actual side effects.

If you understand that architecture deeply, concepts such as **function calling, MCP, tool use, agents, agentic workflows, computer use, multi-agent systems, LangGraph and orchestration frameworks** become much easier—they're variations and abstractions built around the same basic **decide → act → observe** loop.
