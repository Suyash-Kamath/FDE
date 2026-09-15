# 11 · Human-in-the-Loop

Full autonomy is the wrong default for anything irreversible. The engineering
question is not *whether* to involve a human, but **where in the loop, how
often, and with what information.**

## 11.1 Four places a human can enter

```
GOAL ──► [1] clarify ──► DECIDE ──► [2] approve ──► ACT ──► OBSERVE ──► [3] review
                            ▲                                              │
                            └──────────── [4] correct / steer ─────────────┘
```

| # | Pattern | When | Cost to user |
|---|---|---|---|
| 1 | **Clarify before starting** | Goal is ambiguous or expensive to get wrong | low, once |
| 2 | **Approve before acting** | The action is irreversible | medium, per action |
| 3 | **Review after acting** | Action is reversible and the agent is usually right | low, batchable |
| 4 | **Interrupt and steer** | Long-running task going off the rails | low, optional |

Most systems need 1 and 2. Pattern 3 is underused and often the best trade.
Pattern 4 is what makes long-running agents tolerable.

## 11.2 Pattern 1 — clarify before starting

The cheapest intervention by far. An agent that runs 20 turns on a
misunderstood goal has wasted everything.

Give the agent a tool to ask:

```python
@registry.tool(
    description=(
        "Asks the user a clarifying question and waits for their answer. Use "
        "this ONCE at the start if the request is ambiguous in a way that "
        "would change what you build — for example if you don't know the "
        "user's name, the site's purpose, or a required colour scheme. Do not "
        "use it for preferences you can reasonably decide yourself, and do not "
        "use it more than twice in a run. Prefer making a sensible choice and "
        "stating your assumption."
    ),
    question="A single, specific question. Not a list.",
    options="Optional pipe-separated suggested answers, e.g. 'dark|light'.",
)
def ask_user(question: str, options: str = "") -> str:
    print(f"\n❓ {question}")
    if options:
        print(f"   ({options})")
    return input("> ").strip() or "(no answer given; use your judgement)"
```

The description is doing careful work. Without *"do not use it more than twice"*
and *"prefer making a sensible choice"*, models become interrogators — twelve
questions before writing a line. The failure mode of a clarification tool is
always over-use, never under-use.

**Alternative that's often better: ask once, in structured form.** Instead of a
tool, run one cheap LLM call up front that returns a JSON list of the 0–3
genuinely blocking unknowns, present them as a form, then start the agent with a
complete brief. One interruption instead of several, and the agent loop stays
clean.

## 11.3 Pattern 2 — approve before acting

The core control for irreversible tools. Three things determine whether it
actually works:

**(a) The human must see what will happen, not what was requested.**

```
❌  The agent wants to call send_email. Approve? [y/N]

✅  ┌─ Approval required ──────────────────────────────────┐
    │ Action:    Send email                                │
    │ To:        priya@example.com                         │
    │ Subject:   Re: Q3 invoice discrepancy                │
    │ Body:      Hi Priya, I've reviewed the invoice and…  │
    │            [ full text, scrollable ]                 │
    │                                                      │
    │ Why:       You asked me to reply to Priya's email    │
    │            about the invoice. (turn 4 of this run)   │
    │ Risk:      Irreversible — cannot be unsent           │
    │                                                      │
    │ [Approve]  [Edit]  [Reject with reason]              │
    └──────────────────────────────────────────────────────┘
```

**(b) Rejection must be informative.** A bare "no" makes the agent guess. Feed the
reason back as a tool result:

```python
if not decision.approved:
    return (f"The user declined this action. Their reason: "
            f"{decision.reason or 'none given'}. Do not retry the same action. "
            f"Either propose a different approach or ask a clarifying question."), True
```

Without *"do not retry the same action"* many models will simply try again,
identically, and you've built an approval-fatigue machine.

**(c) `Edit` is the most valuable button.** Users often want *that, but with a
change*. Let them edit the arguments and approve the edited version. Return the
edit to the model as part of the result so it learns what the user actually
wanted:

```python
if decision.edited_args != args:
    result = execute(name, decision.edited_args)
    return (f"The user approved this action but edited it first. "
            f"Executed with: {json.dumps(decision.edited_args)}. "
            f"Result: {result}"), False
```

That feedback is genuinely valuable — the model adapts subsequent calls in the
same run, and your logs of edits are the best dataset you'll have for improving
the prompt.

## 11.4 Approval fatigue is a security failure

If you ask for approval on everything, users click Approve reflexively within a
week, and you've *converted a security control into a liability* — now there's a
paper trail saying a human authorised something nobody read.

Guidelines that keep approvals meaningful:

- **Approve by risk tier, not by tool.** `READ` never asks. `WRITE` asks only in
  unusual cases. `IRREVERSIBLE` always asks.
- **Batch.** "The agent will create 4 files: [list]. Approve all?" beats four
  prompts.
- **Scoped standing approval.** "Allow `write_file` under `./my-site/` for this
  run" — bounded by run, path and tool. Not "always allow write_file".
- **Thresholds.** Refunds under ₹500 auto-approve; above that, ask. Encode it as
  policy, not as a prompt rule.
- **Show the diff, not the action.** For a file edit, a diff is reviewable in two
  seconds; a 4 KB blob is not.
- **Measure it.** If your approval rate is >95%, you're asking too often. If it's
  <60%, your agent is badly prompted. Both are fixable; neither is fixed by
  adding more prompts.

## 11.5 Pattern 3 — review after acting

For reversible actions, acting first and reviewing later is usually a better
trade than gating. The agent does 20 things, the human reviews a summary and
undoes the two that were wrong.

This requires **undo**, which requires designing for it:

```python
@dataclass
class Undoable:
    tool: str
    args: dict
    undo: typing.Callable[[], None]
    label: str

# write_file records the previous content before overwriting
def write_file(path, content):
    target = safe_path(path)
    before = target.read_text() if target.exists() else None
    target.write_text(content)

    def _undo():
        if before is None: target.unlink(missing_ok=True)
        else:              target.write_text(before)

    journal.append(Undoable("write_file", {"path": path}, _undo,
                            f"wrote {len(content)} bytes to {path}"))
    return f"Wrote {len(content.encode())} bytes to {path}"
```

Even simpler and often better: **do the whole run in a git branch, or a copy of
the directory.** Then undo is `git checkout .` and review is `git diff`. This is
why coding agents work on branches — the entire run becomes atomically
reviewable and revertible. Steal the pattern for non-code work too: snapshot the
workspace before the run, diff after.

## 11.6 Pattern 4 — interrupt and steer

For runs over ~30 seconds, users need to see progress *and* be able to intervene.
Emit events from the loop (chapter 6's `on_event`) and check a cancellation flag
between turns:

```python
import threading

class RunControl:
    def __init__(self):
        self.cancelled = threading.Event()
        self.injected: list[str] = []
        self._lock = threading.Lock()

    def cancel(self):
        self.cancelled.set()

    def steer(self, message: str):
        """User types guidance mid-run; delivered at the next turn boundary."""
        with self._lock:
            self.injected.append(message)

    def drain(self) -> list[str]:
        with self._lock:
            msgs, self.injected = self.injected, []
        return msgs
```

In the loop, between turns:

```python
if control.cancelled.is_set():
    messages.append({"role": "user", "content":
        "[system] The user cancelled this run. Stop calling tools and briefly "
        "summarise what you completed and what state things are in."})
    # final turn with tool_choice: none
    ...

for note in control.drain():
    messages.append({"role": "user", "content": f"[user guidance] {note}"})
```

Two details that matter:

- **Deliver steering at turn boundaries, not mid-tool.** Interrupting a running
  tool leaves inconsistent state. Between turns everything is committed.
- **On cancel, always run one final summarisation turn.** The user needs to know
  what state their filesystem/database is in. "Cancelled" with no report is worse
  than useless — they now have to go and check manually.

## 11.7 Asynchronous approval — the production shape

An interactive `input()` works on a laptop. A real system has the agent running
on a server and the human approving from a phone hours later. That means the loop
must **suspend and resume**, which means the agent's state must be
**serialisable**.

Good news: it already is. The state is the `messages` list plus your budget
counters. That's it.

```python
class PendingApproval(Exception):
    def __init__(self, approval_id): self.approval_id = approval_id


def run(run_id: str):
    state = store.load(run_id)          # {messages, budget, ...}
    try:
        result = run_agent_loop(state, approver=async_approver(run_id))
    except PendingApproval as p:
        store.save(run_id, state)       # freeze exactly here
        notify_user(run_id, p.approval_id)
        return {"status": "awaiting_approval", "approval_id": p.approval_id}
    store.complete(run_id, result)
    return result


def on_approval_webhook(run_id, approval_id, approved, reason, edited_args):
    state = store.load(run_id)
    state["messages"].append({"role": "user", "content": [{
        "type": "tool_result",
        "tool_use_id": state["pending_tool_use_id"],
        "content": (execute_approved(approval_id, edited_args) if approved
                    else f"The user declined. Reason: {reason}. Do not retry."),
        "is_error": not approved,
    }]})
    run(run_id)                          # resume from the frozen state
```

This is the design that makes agents usable in real organisations: the agent
works, hits something that needs authority it doesn't have, freezes, asks, and
resumes. It's the same shape as a workflow engine's human task, and if you
already run something like Temporal or a durable queue, use it — you get retries,
timeouts and visibility for free.

Both vendors now expose approval/guardrail primitives in their agent SDKs; the
underlying pattern is exactly this.

## 11.8 The trust ladder

Don't ship at full autonomy. Earn it, per tool, with data.

```
Level 0  Suggest only        — agent proposes, human does everything
Level 1  Approve each        — agent acts, human approves each action
Level 2  Approve by risk     — reads auto, writes auto, irreversible asks
Level 3  Approve by threshold— small auto, large asks
Level 4  Review after        — agent acts, human reviews a summary, can undo
Level 5  Autonomous + audit  — agent acts, humans sample the logs
```

Move a tool up a level when you have evidence: *"`write_file` has run 2,400 times
with a 0.3% rejection rate over six weeks."* Move it back down the moment
something goes wrong. Keep the level per-tool and per-context — the same tool can
sit at level 5 in a scratch workspace and level 1 in production.

And track the boring metric that actually predicts trouble: **how often does a
human reject, and does that rate move when you change the prompt or the model?**
A rejection-rate regression after a model upgrade is the earliest signal you'll
get that something has drifted.

➡️ Next: [12 · Production checklist](12-production-checklist.md)
