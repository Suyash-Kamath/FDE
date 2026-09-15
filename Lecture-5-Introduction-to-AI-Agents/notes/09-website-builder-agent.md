# 09 · Building the AI Website Builder Agent

This is the video's capstone, in Python. It's the first thing in these notes that
is unambiguously an **agent**: the model decides how many files to create, what
to call them, what goes in them, whether to re-read and fix, and when it's done.

## 9.1 What makes this different from the calculator

| | Calculator | Website builder |
|---|---|---|
| Tool type | information | **action** (writes to disk) |
| Steps | 1, known | unknown — 4 to 20 |
| Model decides… | which operation | file names, count, content, order, when to stop |
| Failure | wrong number | **files on your disk** |
| Needs sandbox | no | **yes** |
| Needs observation | no | yes — must verify what it wrote |

The gap between the two is the gap between "LLM with tools" and "agent".

## 9.2 The security decision, up front

The obvious design is one tool:

```python
run_command(cmd: str)     # ← never do this
```

The model would send `mkdir portfolio`, `cd portfolio`, `touch index.html`,
`cat > index.html <<EOF …`. Elegant, and catastrophic. Its maximal effect is
anything your OS user can do. One hallucinated `rm -rf ~`, or one prompt
injection arriving through some other tool's output, and your home directory is
gone. See chapter 7 §7.6.

Instead: **four narrow tools, all rooted in one directory.**

```
create_directory(path)   — mkdir -p, inside workspace only
write_file(path, content)— create + write, inside workspace only
read_file(path)          — read, inside workspace only          ← observation
list_files(path)         — ls, inside workspace only            ← observation
```

There is no `delete`, no `move`, no `execute`, no `chmod`, no network. Deleting
your files is not *forbidden* — it is **unrepresentable**. That's the design goal.

## 9.3 The sandbox primitive

Everything hinges on one function. It is short, and every line matters.

```python
# sandbox.py
from pathlib import Path


class SandboxViolation(Exception):
    """The agent tried to touch something outside its workspace."""


WORKSPACE = Path("./generated-sites").resolve()
WORKSPACE.mkdir(parents=True, exist_ok=True)


def safe_path(user_path: str) -> Path:
    """Resolve a model-supplied path, guaranteeing it stays inside WORKSPACE."""
    if not isinstance(user_path, str) or not user_path.strip():
        raise SandboxViolation("Path must be a non-empty string.")

    p = user_path.strip()

    # ① reject absolute paths explicitly, rather than silently rebasing
    if Path(p).is_absolute() or p.startswith("~"):
        raise SandboxViolation(
            f"Absolute paths are not allowed. Use a path relative to the "
            f"workspace, e.g. 'my-site/index.html'. You sent: {p!r}"
        )

    # ② resolve() collapses '..' AND follows symlinks — do this BEFORE checking
    candidate = (WORKSPACE / p).resolve()

    # ③ containment check on the RESOLVED path
    if candidate != WORKSPACE and WORKSPACE not in candidate.parents:
        raise SandboxViolation(
            f"Path {p!r} resolves outside the workspace. All files must live "
            f"under the workspace directory."
        )

    return candidate
```

### Why each line is there

**① Absolute paths.** This is the subtle one, and it bites nearly everyone:

```python
>>> Path("/home/me/workspace") / "/etc/passwd"
PosixPath('/etc/passwd')          # ← the join is SILENTLY DISCARDED
```

`pathlib` treats an absolute right-hand operand as replacing the left. If you
only do `(WORKSPACE / p).resolve()` and check containment, you're fine — the check
catches it. But rejecting explicitly gives the model a **useful error message**,
which is how it self-corrects (chapter 6 §6.5). Also reject `~`, which
`expanduser()` would later turn into a home-directory escape.

**② Resolve before checking, not after.** This defeats two attacks at once:

```python
"../../../../etc/passwd"          → resolves to /etc/passwd → caught
"site/../../../.ssh/id_rsa"       → resolves outside        → caught
```

and, crucially, **symlinks**. If the agent (or anything else) creates
`workspace/evil -> /etc`, then `workspace/evil/passwd` *looks* contained by
string comparison but resolves to `/etc/passwd`. `resolve()` follows the link, so
the containment check sees the truth. **Never do a string `startswith` check on
an unresolved path** — that is the single most common sandbox bug.

**③ The containment check.** `WORKSPACE in candidate.parents` is exact — it
compares path components, not string prefixes. Compare with the naive version:

```python
# ❌ BROKEN: string prefix
str(candidate).startswith(str(WORKSPACE))
# /home/me/generated-sites-evil  startswith  /home/me/generated-sites  → True (!)
```

On Python 3.9+ you can equivalently write
`candidate.is_relative_to(WORKSPACE)`, which reads better; the explicit
`parents` form is shown so the mechanism is visible.

**Residual risks worth knowing** (see chapter 10 for the full treatment): a TOCTOU
race if something else can create symlinks in the workspace concurrently;
Windows 8.3 short names and case-insensitivity; and the fact that a sandbox on
paths is not a sandbox on *resources* — the agent can still fill your disk.

## 9.4 The four tools

```python
# tools/filesystem.py
from pathlib import Path
from sandbox import safe_path, SandboxViolation, WORKSPACE
from toolkit import registry

MAX_FILE_BYTES = 512 * 1024          # 512 KB per file
MAX_TOTAL_BYTES = 10 * 1024 * 1024   # 10 MB per run
MAX_FILES = 60

ALLOWED_SUFFIXES = {
    ".html", ".htm", ".css", ".js", ".json", ".md", ".txt", ".svg", ".webmanifest",
}


def _rel(p: Path) -> str:
    return str(p.relative_to(WORKSPACE))


def _budget_check(new_bytes: int) -> None:
    files = [f for f in WORKSPACE.rglob("*") if f.is_file()]
    if len(files) >= MAX_FILES:
        raise SandboxViolation(f"File limit reached ({MAX_FILES}).")
    total = sum(f.stat().st_size for f in files)
    if total + new_bytes > MAX_TOTAL_BYTES:
        raise SandboxViolation("Workspace size limit reached.")


# ─────────────────────────────────────────────────────────────── ACTION ──
@registry.tool(
    description=(
        "Creates a new directory inside the website workspace, including any "
        "missing parent directories. Use this once at the start of each new "
        "website to make a project folder. Succeeds silently if the directory "
        "already exists. Paths must be relative to the workspace; absolute "
        "paths and paths containing '..' are rejected."
    ),
    path="Relative directory path, e.g. 'suyash-portfolio' or "
         "'suyash-portfolio/assets'. Use lowercase kebab-case names.",
)
def create_directory(path: str) -> str:
    target = safe_path(path)
    target.mkdir(parents=True, exist_ok=True)
    return f"Created directory: {_rel(target)}/"


@registry.tool(
    description=(
        "Writes text content to a file inside the workspace, creating the file "
        "and any missing parent directories. OVERWRITES the file if it already "
        "exists, so read it first if you intend to modify rather than replace. "
        "Allowed extensions: .html .htm .css .js .json .md .txt .svg. Maximum "
        "512 KB per file. Returns the number of bytes written, which you should "
        "sanity-check against the content you intended to write."
    ),
    path="Relative file path including the directory and extension, "
         "e.g. 'suyash-portfolio/index.html'.",
    content="The complete text content of the file. Write the whole file every "
            "time; partial content will truncate the file.",
)
def write_file(path: str, content: str) -> str:
    target = safe_path(path)

    if target.suffix.lower() not in ALLOWED_SUFFIXES:
        raise SandboxViolation(
            f"Extension {target.suffix!r} is not allowed. Allowed: "
            f"{', '.join(sorted(ALLOWED_SUFFIXES))}."
        )

    data = content.encode("utf-8")
    if len(data) > MAX_FILE_BYTES:
        raise SandboxViolation(
            f"File too large ({len(data)} bytes, max {MAX_FILE_BYTES}). "
            f"Split it into smaller files."
        )
    _budget_check(len(data))

    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")
    return f"Wrote {len(data)} bytes to {_rel(target)}"


# ────────────────────────────────────────────────────────── OBSERVATION ──
@registry.tool(
    description=(
        "Reads and returns the full text content of a file in the workspace. "
        "Use this to verify what you actually wrote, to check that a file links "
        "to its stylesheet correctly, or to modify an existing file (read it, "
        "then write the complete modified version back). Returns an error if "
        "the file does not exist."
    ),
    path="Relative file path, e.g. 'suyash-portfolio/index.html'.",
)
def read_file(path: str) -> str:
    target = safe_path(path)
    if not target.exists():
        raise SandboxViolation(
            f"File {path!r} does not exist. Use list_files to see what is there."
        )
    if not target.is_file():
        raise SandboxViolation(f"{path!r} is a directory, not a file.")

    text = target.read_text(encoding="utf-8", errors="replace")
    if len(text) > 40_000:
        return text[:40_000] + "\n... [truncated at 40,000 characters]"
    return text


@registry.tool(
    description=(
        "Lists files and directories inside the workspace. Returns one entry "
        "per line with a trailing slash for directories and a byte size for "
        "files. Use this to verify the project structure you have built, or to "
        "check what exists before writing. Pass '.' for the workspace root."
    ),
    path="Relative directory path to list, or '.' for the workspace root.",
)
def list_files(path: str = ".") -> str:
    target = safe_path(path)
    if not target.exists():
        raise SandboxViolation(f"Directory {path!r} does not exist.")
    if not target.is_dir():
        raise SandboxViolation(f"{path!r} is a file, not a directory.")

    entries = sorted(target.iterdir(), key=lambda e: (e.is_file(), e.name))
    if not entries:
        return f"{_rel(target) or '.'}/ is empty."

    lines = [
        f"{e.name}/" if e.is_dir() else f"{e.name}  ({e.stat().st_size} bytes)"
        for e in entries
    ]
    return f"Contents of {_rel(target) or '.'}/:\n" + "\n".join(lines)
```

### Design notes worth absorbing

- **`write_file` replaces `touch` + `cat`.** The video uses two shell steps; one
  tool that creates-and-writes is fewer round trips, fewer failure modes, and
  fewer tools competing for the model's attention. Anthropic's guidance on
  consolidating related operations applies directly.
- **The extension allowlist** is why the agent can't write `.sh`, `.py` or
  `.bashrc`. Combined with "no execute tool", written content is inert.
- **Byte and file budgets** stop a looping agent from filling your disk. A path
  sandbox alone doesn't protect resources.
- **`write_file` returns the byte count**, not `"ok"`. That's an observation the
  model can check: if it intended 4 KB of HTML and gets back "Wrote 312 bytes",
  something truncated.
- **`read_file` truncates at 40 K chars** so one big file can't blow the context
  window — and says so explicitly, so the model knows it's seeing a prefix.

## 9.5 The system prompt

For an agent, the system prompt is the specification. This one is long on
purpose. Each paragraph fixes a specific failure I'd otherwise expect.

```python
SYSTEM_PROMPT = """You are an expert front-end developer agent. You build
complete, working static websites by actually creating files with your tools.

## Your tools
- create_directory(path)  — make a project folder
- write_file(path, content) — write a complete file (overwrites)
- read_file(path)         — read back what you wrote
- list_files(path)        — inspect the structure

## Process (follow in order)
1. Briefly state a plan: the folder name and the files you will create.
2. create_directory with a lowercase kebab-case project folder name.
3. write_file for index.html — complete, valid HTML5, linking ./style.css and
   ./script.js.
4. write_file for style.css — complete styling. Never inline large CSS in HTML.
5. write_file for script.js only if there is real interactivity to implement.
6. list_files on the project folder to confirm every file exists.
7. read_file on index.html and check that the <link> and <script> paths match
   the filenames you actually created. Fix any mismatch with write_file.
8. Report what you built and the exact path to open.

## Hard rules
- DO NOT put website code in your chat response. Your chat response is a status
  report. The website only exists if you called write_file.
- Use only HTML, CSS and vanilla JavaScript. No React, Vue, Tailwind, bootstrap
  CDN links, or any build step.
- No external network requests in the generated site: no Google Fonts, no CDN
  scripts, no remote images. Use system font stacks and CSS gradients or inline
  SVG instead. The site must work fully offline.
- Every section you mention in your plan must contain real content. Never leave
  a section empty or with placeholder lorem ipsum if the user gave you real
  information.
- Write mobile-responsive CSS with at least one media query.
- Include semantic HTML: <header>, <nav>, <main>, <section>, <footer>, alt text
  on images, and a <title>.
- All paths are relative to the workspace. Never use absolute paths or '..'.

## Finishing
You are finished ONLY when list_files shows every file you planned, and
read_file confirms index.html references them correctly. Then stop calling
tools and give your status report."""
```

Why these specific lines:

| Rule | Failure it prevents |
|---|---|
| "DO NOT put website code in your chat response" | The #1 failure: model writes beautiful HTML into the chat and calls it done. Creating files is *not* its default behaviour. |
| "No external network requests" | Model reflexively adds Google Fonts / CDN links, and the site looks broken offline. |
| "read_file and check the `<link>` path" | Model writes `styles.css` but links `style.css`. Unstyled page, no error anywhere. |
| "list_files to confirm" | Forces at least one real observation before claiming success. |
| "You are finished ONLY when…" | Explicit termination criterion (chapter 8 §8.4) — stops early exit. |
| "state a plan first" | Gives the model something to check itself against, and gives you explainability. |

## 9.6 The complete agent

```python
# website_agent.py
from __future__ import annotations
import json, sys, time, webbrowser
from pathlib import Path

from anthropic import Anthropic
from toolkit import registry
from sandbox import WORKSPACE, SandboxViolation
import tools.filesystem          # registers the four tools

client = Anthropic()
MODEL = "claude-sonnet-4-5"
MAX_TURNS = 25
MAX_TOOL_CALLS = 50

SYSTEM_PROMPT = """..."""        # from §9.5


def build_website(brief: str) -> dict:
    messages = [{"role": "user", "content": brief}]
    calls = 0
    started = time.monotonic()

    for turn in range(1, MAX_TURNS + 1):
        resp = client.messages.create(
            model=MODEL,
            max_tokens=8192,                    # HTML is long; don't starve it
            system=SYSTEM_PROMPT,
            tools=registry.anthropic_schemas(),
            messages=messages,
        )
        messages.append({"role": "assistant", "content": resp.content})

        # surface the model's reasoning — free explainability
        for b in resp.content:
            if b.type == "text" and b.text.strip():
                print(f"\n\033[36m[think]\033[0m {b.text.strip()}")

        if resp.stop_reason == "max_tokens":
            messages.append({"role": "user", "content":
                "Your last message was cut off by the token limit. Write the "
                "file again, splitting it into smaller files if necessary."})
            continue

        if resp.stop_reason != "tool_use":
            elapsed = round(time.monotonic() - started, 1)
            return {
                "ok": True, "turns": turn, "tool_calls": calls,
                "seconds": elapsed,
                "report": "".join(b.text for b in resp.content if b.type == "text"),
            }

        results = []
        for b in resp.content:
            if b.type != "tool_use":
                continue
            calls += 1
            if calls > MAX_TOOL_CALLS:
                results.append({"type": "tool_result", "tool_use_id": b.id,
                                "is_error": True,
                                "content": "Error: tool-call budget exhausted. "
                                           "Stop and report what you completed."})
                continue

            preview = {k: (v[:60] + "…" if isinstance(v, str) and len(v) > 60 else v)
                       for k, v in b.input.items()}
            print(f"\033[33m[tool]\033[0m {b.name}({json.dumps(preview)})")

            content, is_error = registry.call(b.name, b.input)
            print(f"      \033[{'31' if is_error else '32'}m→\033[0m "
                  f"{content.splitlines()[0][:100] if content else ''}")

            results.append({"type": "tool_result", "tool_use_id": b.id,
                            "content": content, "is_error": is_error})

        messages.append({"role": "user", "content": results})

    return {"ok": False, "turns": MAX_TURNS, "tool_calls": calls,
            "report": "Stopped: exceeded maximum turns."}


def newest_index() -> Path | None:
    candidates = sorted(WORKSPACE.rglob("index.html"),
                        key=lambda p: p.stat().st_mtime, reverse=True)
    return candidates[0] if candidates else None


if __name__ == "__main__":
    brief = " ".join(sys.argv[1:]) or (
        "Create a portfolio website for Suyash Kamath, a backend engineer based "
        "in Mumbai. He works with Go, PostgreSQL, RabbitMQ and AWS. Include a "
        "hero section with his name and title, an about section, a skills "
        "section, a projects section with three placeholder projects, and a "
        "contact section. Use a dark theme with a teal accent. Make it "
        "responsive."
    )

    result = build_website(brief)
    print("\n" + "─" * 70)
    print(result["report"])
    print(f"\n{result['turns']} turns, {result['tool_calls']} tool calls, "
          f"{result.get('seconds', '?')}s")

    if (idx := newest_index()):
        print(f"open: {idx}")
        webbrowser.open(idx.as_uri())
```

### Notes on the runner

- **`max_tokens=8192`.** A complete `style.css` can be 3–5 K tokens. At the
  default cap the model gets truncated mid-file and writes broken CSS with no
  error. Handle `stop_reason == "max_tokens"` explicitly; it's silent otherwise.
- **`webbrowser.open`** is deliberately *outside* the agent. The agent creates
  files; **your program** decides to open one. Never give the model a tool that
  launches things.
- **Truncated argument preview in logs.** A 4 KB HTML string in your terminal on
  every call makes the trace unreadable.
- **The `[think]` output** is the model's text block before its tool calls. Show
  it to users — it's the best progress indicator you'll get for free.

## 9.7 A real trace

```
[think] I'll create a portfolio site for Suyash. Plan:
        1. create_directory suyash-portfolio
        2. index.html — hero, about, skills, projects, contact
        3. style.css — dark theme, teal accent, responsive
        4. script.js — smooth scroll + mobile nav toggle
        5. verify with list_files and read_file

[tool] create_directory({"path": "suyash-portfolio"})
      → Created directory: suyash-portfolio/
[tool] write_file({"path": "suyash-portfolio/index.html", "content": "<!DOCTYPE…"})
      → Wrote 5418 bytes to suyash-portfolio/index.html
[tool] write_file({"path": "suyash-portfolio/style.css", "content": ":root{--bg…"})
      → Wrote 6902 bytes to suyash-portfolio/style.css
[tool] write_file({"path": "suyash-portfolio/script.js", "content": "document.a…"})
      → Wrote 1204 bytes to suyash-portfolio/script.js

[think] Now verifying the structure and the asset links.

[tool] list_files({"path": "suyash-portfolio"})
      → Contents of suyash-portfolio/:
[tool] read_file({"path": "suyash-portfolio/index.html"})
      → <!DOCTYPE html>

[think] Links are correct: ./style.css and ./script.js both match. Done.

──────────────────────────────────────────────────────────────────────
Built suyash-portfolio/ with index.html, style.css and script.js...

6 turns, 6 tool calls, 41.3s
open: /home/suyash/generated-sites/suyash-portfolio/index.html
```

Read the trace against chapter 8's six components:

| Component | Where it is in the trace |
|---|---|
| Goal | the brief |
| Decide | each `[think]` + the choice of tool |
| Act | each `[tool]` line |
| Environment | `./generated-sites/` |
| Observe | `list_files` + `read_file` — reality can disagree |
| Loop | six turns until `stop_reason == "end_turn"` |

## 9.8 Failure modes you will actually see

| Symptom | Cause | Fix |
|---|---|---|
| Beautiful HTML in the chat, zero files | Model's default is to answer, not act | The "DO NOT put code in your response" rule, stated twice |
| Unstyled page, no error | `<link href="styles.css">` vs `style.css` | Mandate the read-back verification step |
| CSS cuts off mid-rule | `max_tokens` hit | Raise it; handle `stop_reason == "max_tokens"` |
| Site broken offline | CDN / Google Fonts links | Explicit "no external requests" rule |
| Empty sections | Model ran out of enthusiasm | "Every section must contain real content" |
| Agent loops rewriting index.html | No clear done criterion | Explicit termination criterion + `MAX_TOOL_CALLS` |
| `SandboxViolation` on every write | Model using absolute paths | Your error message already teaches it — check it's reaching the model |
| Costs ₹40 per site | 8 K-token files re-sent every turn | Don't echo file content back; `read_file` already truncates |

## 9.9 Extending it

**Serve the site.** Don't give the agent a server tool. Your program serves it:

```python
import http.server, functools, threading
handler = functools.partial(http.server.SimpleHTTPRequestHandler,
                            directory=str(site_dir))
threading.Thread(target=http.server.HTTPServer(("", 8000), handler).serve_forever,
                 daemon=True).start()
```

**Iterative refinement.** Keep `messages` across requests and the agent can edit:
*"make the hero taller and change the accent to orange"* → it calls `read_file`,
then `write_file` with the modified content. Because history persists, it knows
what it built. This is exactly the "just keep the history" point from the video.

**A validation tool.** Add `validate_html(path)` that runs a parser and returns
errors. This is a *pure observation* tool and it makes self-correction much
sharper — the environment can now tell the agent something it genuinely didn't
know.

**Screenshot feedback.** Render the page headless, return the PNG as an image
block in the tool result, and let a vision model critique its own layout. This
closes the loop on *appearance*, which text observation cannot reach. It is also
where the agent starts to feel genuinely capable.

➡️ Next: [10 · Sandboxing and security, properly](10-sandboxing-and-security.md)
