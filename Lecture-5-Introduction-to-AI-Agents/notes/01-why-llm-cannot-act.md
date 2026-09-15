# 01 · Why an LLM Cannot Directly Perform Actions

## 1.1 What the model actually is

Strip away the chat UI, the SDK, the streaming — a raw LLM is one function call:

```
f(token_ids: list[int]) -> probability_distribution_over_vocabulary
```

That's it. One forward pass through the transformer takes your token IDs and
produces a vector of logits, one per vocabulary entry (roughly 100k–200k entries
for modern tokenizers). Softmax that vector and you have a probability
distribution over "what token comes next". Sampling picks one. That token is
appended to the input and the whole thing runs again.

```
input:  [The, capital, of, France, is]
        ↓ transformer forward pass
logits: {" Paris": 18.2, " a": 6.1, " located": 5.9, ...}
        ↓ softmax + sample
next:   " Paris"
        ↓ append, repeat
input:  [The, capital, of, France, is, " Paris"]
```

This is *autoregressive decoding*. The loop ends when the model emits a
stop token or you hit `max_tokens`.

Three consequences fall directly out of this, and they are the entire
justification for tools:

**(a) The output is text, and only text.** There is no code path in the model
that touches a file descriptor, opens a socket, or writes to disk. Producing the
characters `rm -rf /` and *executing* `rm -rf /` are unrelated events. The model
can only do the first.

**(b) The model is frozen in time.** The weights encode a compressed statistical
summary of a training corpus that ended on some date. Today's USD→INR rate was
never in that corpus and can never be in those weights. Asking for it produces a
*plausible-looking* number, which is worse than no number.

**(c) The model doesn't compute, it recalls.** When you ask for
`789_65 × 34_576`, no multiplication happens. The model predicts digits that
"look right" given the digits it has seen. For small numbers it often lands on
the correct answer, because `7 × 8 = 56` appeared millions of times in training —
that's memorisation, not arithmetic. For a novel 6-digit pair, the pattern isn't
memorised, so it fabricates. In the video the model returned `94,291,195`
against a true value of `94,256,438,941` — not just wrong, wrong by three orders
of magnitude.

> **The deep reason:** multiplication of *n*-digit numbers requires a variable
> number of sequential carry operations. A transformer has a fixed number of
> layers, so it has a fixed compute budget per token. A problem that needs
> unbounded sequential steps cannot be solved in bounded depth. Chain-of-thought
> partially fixes this by letting the model spend more tokens (more forward
> passes) on the problem — which is why reasoning models do better at arithmetic —
> but it is still statistical approximation of an algorithm, not the algorithm.
> A calculator runs the algorithm. Always prefer the algorithm.

## 1.2 Deterministic vs non-deterministic — the real point

The video makes a big deal of this and it deserves precision.

| | LLM | Your Python function |
|---|---|---|
| Same input → same output? | No (sampling; and even at `temperature=0`, GPU float non-associativity and batching make it only *near*-deterministic) | Yes |
| Failure mode | Confidently wrong, no signal | Exception, stack trace, exit code |
| Input format | Anything (natural language) | Rigid (typed signature) |
| Good at | Intent, ambiguity, language, planning | Exactness, side effects, I/O |

These two are *complementary*, not competing. An agent is the engineering
pattern that sticks the flexible-but-unreliable thing in front of the
rigid-but-exact thing and lets each do what it's good at.

```
natural language ──► [LLM] ──► structured JSON ──► [your function] ──► exact result
   (ambiguous)      flexible      (rigid)            deterministic
```

The LLM's job is **translation and routing**, not execution. Once you internalise
that, tool design becomes obvious: give the model the *decisions*, keep the
*execution* in code.

## 1.3 LLM = Brain, Tools = Hands

The video's analogy: a very smart person with no hands. It's a good one, but
there's a sharper version that matters for how you write code.

The model is a **brain in a jar with one input slot and one output slot**. Both
slots carry only text. It has:

- no memory between calls (every request re-sends the whole conversation)
- no clock (it does not know what time it is)
- no network (it cannot fetch anything)
- no filesystem
- no ability to run its own output

Everything it "does" is done by **you**, the host program, reading its text and
choosing to act on it. Tool calling is just a disciplined, machine-readable
format for that text, so your host program can act on it reliably instead of
regexing prose.

```
┌──────────────────────────────────────────────────────┐
│  YOUR PYTHON PROCESS  (the body — has hands)         │
│                                                      │
│   • the messages list (the memory)                   │
│   • the tool registry (the hands)                    │
│   • the loop (the will)                              │
│   • the sandbox (the conscience)                     │
│                                                      │
│        ┌──── HTTPS ────┐                             │
│        │               │                             │
│        ▼               │                             │
│  ┌──────────────────────────┐                        │
│  │  LLM (brain in a jar)    │  ← stateless, text-only│
│  │  text in → text out      │                        │
│  └──────────────────────────┘                        │
└──────────────────────────────────────────────────────┘
```

**Everything interesting lives in your process, not in the model.** The model is
one function call inside your loop. People building agents for the first time
usually get this backwards and think the "agent" is the model. The agent is your
code; the model is the decision-making subroutine it calls.

## 1.4 The AC-remote example, made precise

> "Turn on the AC." A raw LLM replies: *pick up the remote, press the power
> button.* It cannot press it.

The interesting question isn't *why can't it press the button* — obviously it has
no actuator. The interesting question is: **what is the minimum contract that
lets it press the button safely?** That contract is:

1. You tell the model, in the prompt, that a function `set_ac(state)` exists,
   what it does, and what arguments it takes.
2. The model emits, in a parseable format, `set_ac(state="on")`.
3. **You** decide whether to run it. (Maybe you run it. Maybe you ask the user.
   Maybe you refuse because the tool isn't in your registry.)
4. You run it and feed the result back as text.
5. The model turns that result into a sentence for the user.

Step 3 is the whole of agent security and it is entirely under your control.
The model never gains a capability you didn't hand it. Keep that fact in mind
through chapter 10 — it's why sandboxing works at all.

## 1.5 A concrete demonstration you should actually run

Prove the failure to yourself before you fix it. This is the exact experiment
from the video, in Python.

```python
# demo_no_tools.py
import os
from anthropic import Anthropic

client = Anthropic()   # reads ANTHROPIC_API_KEY

A, B = 789_654, 34_576          # pick numbers unlikely to be memorised

resp = client.messages.create(
    model="claude-sonnet-4-5",
    max_tokens=200,
    messages=[{
        "role": "user",
        "content": f"What is {A} * {B}? Reply with just the number, no working out.",
    }],
)

model_answer = resp.content[0].text.strip()
truth = A * B

print("model :", model_answer)
print("python:", truth)
print("match :", str(truth) in model_answer)
```

Two things to notice when you run it:

- Telling it *"no working out"* is what forces the failure. It removes the
  model's ability to spend extra forward passes on the problem. Remove that
  instruction and a modern model will often get it right by writing out long
  multiplication — but slowly, expensively, and still not guaranteed.
- `python: truth` is computed in **one CPU instruction**. That asymmetry — a free,
  exact, instant answer vs. an expensive, approximate, unreliable one — is the
  entire economic argument for tools.

## 1.6 What you should take away

- The model emits tokens. Full stop. All capability beyond that is yours.
- Anything that requires *exactness* (arithmetic, string length, dates, IDs),
  *freshness* (prices, weather, DB state) or *effect* (write, send, pay) must be a
  tool. Not "should be" — *must be*, or you are shipping hallucinations.
- The model's genuine, irreplaceable skill is going from messy human language to
  a precise structured request. That is what the next chapter is about.

➡️ Next: [02 · Tools and Function Calling](02-tools-and-function-calling.md)
