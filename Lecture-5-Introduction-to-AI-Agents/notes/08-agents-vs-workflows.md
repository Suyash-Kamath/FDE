# 08 · When Does an LLM Become an Agent? Agents vs Workflows

## 8.1 There is no bright line — and that's the useful answer

The video is honest about this: you *can* call a calculator-plus-LLM an agent and
nobody can prove you wrong. But the distinction the industry has converged on is
real and worth holding.

> **Workflow:** the *control flow* is written by you. The LLM fills in steps.
>
> **Agent:** the *control flow* is decided by the LLM at runtime. You supply the
> goal, the tools, and the guard rails.

The question isn't "does it use tools" or "how many tools". The question is:
**who decides what happens next?**

```
WORKFLOW — you wrote the arrows
  ┌────────┐   ┌─────────┐   ┌───────────┐   ┌─────────┐
  │classify│──►│ extract │──►│  look up  │──►│ compose │──► done
  └────────┘   └─────────┘   └───────────┘   └─────────┘
  Steps fixed at design time. The LLM is a smart function inside each box.
  Number of LLM calls is known before you run it.

AGENT — the model wrote the arrows
       ┌──────────────► goal
       │                 │
       │            ┌────▼─────┐
       │            │  decide  │ ◄── LLM chooses the next action
       │            └────┬─────┘
       │                 │
       │            ┌────▼─────┐
       │            │   act    │ ◄── a tool runs
       │            └────┬─────┘
       │                 │
       │            ┌────▼─────┐
       └────────────┤ observe  │ ◄── did it work? what now?
                    └────┬─────┘
                         │ goal met?
                         ▼ yes → done
  Path discovered at runtime. Number of LLM calls unknown in advance.
```

By this definition:

- The **calculator** example is a workflow. One LLM call decides one tool, one
  tool runs, one LLM call formats. Fixed shape.
- The **currency conversion** example is *borderline* — the model discovered at
  runtime that it needed two tools in sequence. Nobody coded "rate then
  multiply". That's a real (small) act of planning.
- The **website builder** is an agent. The number of files, their names, the
  order, whether to re-read and fix — all decided at runtime, unbounded in
  length.

## 8.2 A practical test

Ask three questions. Two or three "yes" ⇒ agent.

1. **Can you predict the number of LLM calls before running it?** No ⇒ agent.
2. **Does the model's choice at step *n* change what's available at step *n+1*?**
   Yes ⇒ agent.
3. **Does the model decide when it's finished?** Yes ⇒ agent.

## 8.3 Workflows are usually the right answer

This part gets skipped and it's the most important engineering advice in the
whole topic.

If you can express the task as a fixed pipeline, **do that instead**. A workflow
is cheaper (known token cost), faster (no extra round trips), testable (fixed
call graph), debuggable (you know where it broke), and reliable (no unbounded
exploration). Agents are what you use when the task genuinely cannot be
enumerated in advance.

| Task | Build |
|---|---|
| Classify a support ticket, then route it | workflow |
| Extract fields from an invoice PDF | workflow |
| RAG: retrieve → rerank → answer | workflow |
| Translate, then summarise, then post | workflow |
| Summarise this document | **not even a workflow** — one call |
| "Find why the nightly ETL failed and fix it" | agent |
| "Build me a portfolio site" | agent |
| "Research these 5 vendors and write a comparison" | agent |
| "Reconcile these invoices against the ledger" | agent |

The honest heuristic: **agents trade predictability for generality, and you
should only pay that price when you need generality.** Most production LLM value
today is workflows, and a lot of "agent" projects fail because a workflow would
have shipped in a week.

Also note the middle ground, which is where a lot of good systems land:
a **workflow whose steps are agentic**. Fixed outer pipeline (retrieve → analyse →
report), with one step allowed to loop with tools. You bound the risk to one box.

## 8.4 The six components of an agent

The video's list, expanded into what each actually means in code.

### 1. Goal

What you want. Comes from the user, or from a scheduler, or from another agent.
The single highest-leverage improvement to any agent is a **sharper goal
specification** — including how it will be judged.

```python
goal = (
    "Create a portfolio website for Suyash Kamath, a backend engineer in Mumbai "
    "working in Go, PostgreSQL and RabbitMQ. Include a hero section, an about "
    "section, a projects section with 3 placeholder projects, and a contact "
    "section. Dark theme. Mobile responsive. "
    "Done when index.html, style.css and script.js all exist, index.html links "
    "both of them, and no section is empty."
)
```

That last sentence is the **termination criterion**. Without it the model decides
for itself when it's finished, and it will stop early. This is the cheapest fix
for "the agent gave up halfway".

### 2. Decision maker

The LLM. Its job each turn: *given the goal, the history, and the available
tools, what is the single best next action?* This is where the model's genuine
capability lives, and it's why model choice matters more for agents than for
chat. A weaker model produces a chatbot that calls tools; a stronger one produces
something that plans.

### 3. Actions (tools)

What it can do. Covered in chapters 2–5, 7. Remember: the registry is the
complete set of capabilities. Nothing outside it is reachable.

### 4. Environment

Where actions take effect. `./generated-sites/` for the website builder. A
staging database. A Docker container. A cloud sandbox. **Defining the environment
narrowly is the primary safety control** — chapter 10.

### 5. Observation

How the agent learns what happened. This is the component most people skip and
it's what separates "fire and forget" from "actually works".

Weak observation: `write_file` returns `"ok"`. The agent believes the file is
fine because it says so.

Strong observation: `write_file` returns `"Wrote 4,182 bytes to index.html"`, and
the agent can then call `read_file("index.html")` to check what's actually there,
or `list_files(".")` to confirm structure. Now the environment can *contradict*
the model, and the model can correct.

> This is the deep reason the video gives the agent a `read_file` tool even
> though the agent wrote the content itself: **the agent needs a channel through
> which reality can disagree with it.** Without that channel there's no feedback,
> so there's no loop — just a sequence of hopeful writes.

Design principle: **every action tool should have a matching observation tool.**
`write_file` ↔ `read_file`. `create_directory` ↔ `list_files`.
`deploy` ↔ `get_deployment_status`. `send` ↔ `get_status`.

### 6. Loop

"What should I do next?" repeated. Chapter 6. The loop is what turns five
independent capabilities into goal-directed behaviour.

## 8.5 The Goal → Decide → Act → Observe cycle in code

Concretely, in the message list:

```
messages = [ GOAL ]

┌─► DECIDE   client.messages.create(...)  → tool_use: create_directory("portfolio")
│            messages.append(assistant turn)
│
│   ACT      registry.call("create_directory", {...}) → "Created ./portfolio"
│
│   OBSERVE  messages.append(tool_result: "Created ./portfolio")
│            ↑ the observation IS the next turn's input. That's the whole loop.
│
└─  repeat until stop_reason == "end_turn"
```

There is no separate "observe" step in the code. **Appending the tool result to
`messages` is the observation.** The model reads it on the next forward pass.
The elegance is that the loop needs no state machine — the message list *is* the
state.

## 8.6 Where "reasoning" fits

You'll see ReAct (Reason + Act) cited constantly. Historically it was a prompting
trick: make the model write `Thought: … Action: … Observation: …` as text and
regex it out. Modern tool calling supersedes the parsing part — the API gives you
structured calls — but the *reasoning* half remains valuable:

- Models often emit a text block before a tool_use block ("Let me check today's
  rate first"). That's the Thought. Surface it in your UI; it's the best
  free explainability you'll get.
- Extended/adaptive thinking gives the model dedicated tokens to plan before
  acting. On multi-step tool tasks this measurably improves tool selection.
- A `plan` step — one deliberate call asking for a numbered plan before any tool
  runs — is often worth it for long tasks. It gives the model something to check
  itself against and gives you something to show the user.

```python
PLANNING_SUFFIX = """
Before calling any tool, write a short numbered plan of the files you will
create and what goes in each. Then execute the plan step by step, and after
each step state briefly whether it succeeded.
"""
```

## 8.7 Multi-agent, and why you probably don't want it yet

The natural next thought is "what if agents call other agents?" It's real
(a tool whose implementation runs another agent loop), and it's occasionally
right — genuinely independent subtasks, separate tool sets, or context isolation
so one agent's 50 KB of search results doesn't pollute another's context.

But the failure modes compound: error rates multiply across agents, context gets
lost at every boundary, cost multiplies, and debugging becomes archaeology. A
single agent with well-designed tools beats three agents passing messages, almost
every time, at almost every scale you'll encounter first.

Get one agent genuinely reliable before adding a second.

➡️ Next: [09 · Building the website builder agent](09-website-builder-agent.md)
