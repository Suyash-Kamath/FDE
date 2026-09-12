# 02 — Why an LLM Cannot Work Alone

The video demonstrates three failures. This file explains the *mechanism* behind each,
because once you know the mechanism you can predict the next failure instead of
discovering it in production.

---

## 1. Failure one: no live data (the knowledge cutoff)

### What happened
Playground, `gpt-5.6-luna`: *"What is the weather of Delhi right now?"* →
*"I can't assist with live weather data."* And: *"My general knowledge cutoff is
June 2024."*

### The mechanism

A model's knowledge lives in its **weights**, which are frozen the moment
pre-training ends. Training pipeline, simplified:

```
 crawl + license + clean text  ──►  tokenize  ──►  pre-train (months, $$$$)
        (corpus snapshot)                             │
                                                      ▼
                                          post-train: SFT + RLHF
                                                      │
                                                      ▼
                                       evaluate, red-team, release
```

The **knowledge cutoff** is the date of the corpus snapshot. Everything after it is,
to the model, as if it never happened.

### Why not just keep training it every day?

Four real reasons:

1. **Cost.** A pre-training run is a very large capital expense on thousands of GPUs
   for weeks. You do not redo it for Tuesday's news.
2. **Catastrophic forgetting.** Naively continuing training on a fresh slice of data
   degrades performance on everything else. Keeping a model stable while updating it
   is an open research problem.
3. **Evaluation and safety.** Every new checkpoint has to be re-benchmarked and
   red-teamed. You can't ship that daily.
4. **Reproducibility.** Enterprises pin a model version so behaviour doesn't drift
   under them. A silently-updating model is unshippable in a regulated context.

So the industry's answer is not "retrain" — it's **retrieve at inference time**.

### A caution the video doesn't give

**A model's self-reported cutoff is unreliable.** The date is not stored in a
register; it's a fact the model was told (or inferred) during post-training. Models
routinely state the wrong cutoff, or a cutoff earlier than reality because late data
is sparse in the corpus. Treat it as a hint, not ground truth — check the provider's
model card.

Also: a cutoff is fuzzy, not a hard wall. Coverage thins out over the months
approaching the cutoff, so the model "knows" March much better than it knows the
final month.

### The fix: retrieval

| Need | Tool |
|---|---|
| Public current events | Web search tool |
| Live weather / stocks / flights | A specific API |
| Your company's data | RAG over your own store (embeddings + vector search) |
| Your operational data | A direct DB / service tool call |

For anything about *your* business, note that even a model with a perfect cutoff
never saw your data at all. RAG isn't only a recency fix — it's a *privacy scope* fix.

---

## 2. Failure two: arithmetic

### What happened
`87345 × 5623` — the Playground model produced a confident, wrong number.
ChatGPT produced `491,180,...` correctly, having shown "Calculating the product of
two integers."

### The mechanism

The model is not executing multiplication. It is predicting the token sequence that
*looks like* the answer to this multiplication, based on patterns in training data.

Multiplication has three properties that break that approach:

1. **Compositional, not memorisable.** There are ~10^10 five-digit-by-four-digit
   products. No corpus contains them all. The model has to *generalise* an algorithm
   from examples — and transformers learn a shaky, position-dependent approximation
   of long multiplication.
2. **No scratchpad by default.** Long multiplication needs intermediate state.
   Producing the answer directly is asking a human to multiply two 5-digit numbers
   in their head with no paper. (This is exactly why chain-of-thought and reasoning
   models improve arithmetic — they generate the scratchpad as tokens. They are still
   worse than a calculator.)
3. **Digits tokenize badly.** `87345` may be split as `873` + `45`, and `5623` as
   `56` + `23`. The model doesn't see five digits; it sees two chunks. Place value is
   smeared across token boundaries.

And critically: **the model has no internal signal that it is wrong.** It produces a
plausible number with the same confidence as a correct one. This is the dangerous
part — a wrong invoice total looks exactly like a right one.

### The fix
Route arithmetic to a deterministic tool: a calculator function, or a sandboxed code
interpreter. See [`03`](03-tool-calling-architecture.md).

---

## 3. Failure three: counting the `*` characters

This is the most instructive demo and the video's explanation is incomplete.

### What happened
32 asterisks in the prompt. Playground answered **16**. ChatGPT answered **32**
(after "Counting stars in a string").

### The mechanism — tokenization

The model does not see characters. It sees **tokens**, produced by a
Byte-Pair Encoding tokenizer that merges frequent byte sequences into single IDs.

A run of asterisks compresses aggressively:

```
text:    * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *   (32 chars)

what the tokenizer may emit:
         [" ****"]  ["****"]  ["****"]  ["****"]  ["****"]  ["**"]   ← ~6 tokens
              ↑
   each of these is ONE integer ID to the model.
   Nothing inside it is visible. There is no "length" field.
```

So the model is being asked "how many characters are inside these six opaque
integers?" It can only *estimate* from what similar-looking sequences were labelled
in training. Getting 16 is not a reasoning error — it's a **perception** error. The
information was destroyed before the model saw anything.

This is the same reason for the famous *"how many r's in strawberry"* failure, and for
models mangling rhyme, acrostics, reversed strings, and character-level edits.

### Rule of thumb

> **Any task that depends on the literal characters inside a token is a task the
> model is structurally bad at.** Counting, indexing, spelling, exact string
> manipulation, base64 by hand, checksum digits. Delegate all of it.

### The fix, exactly as ChatGPT does it

1. Model recognises the task is deterministic.
2. Model **writes code** (the video's point — Python, JS, whatever):
   ```python
   s = "********************************"
   print(s.count("*"))
   ```
3. The **application** runs that code in a sandbox.
4. The sandbox returns `32`.
5. The model wraps it in natural language: *"There are 32 stars."*

That last step is worth naming: the LLM's real job in a tool-using system is
**translation at both ends** — natural language → structured tool call, and tool
result → natural language. The middle is done by ordinary deterministic software.

---

## 4. The general taxonomy of tools

| Category | Examples | Solves |
|---|---|---|
| **Retrieval** | web search, vector search / RAG, file search | Stale or private knowledge |
| **Computation** | calculator, code interpreter, unit conversion | Exactness |
| **Live data** | weather, stock, maps, flight status, your own REST APIs | Freshness |
| **Actions (write)** | send email, create ticket, refund, update DB | Effect on the world |
| **Structure** | JSON schema / structured output, validators | Machine-parseable results |

The first four rows change what the model *knows or can do*. The last row changes
what your code can *safely consume*.

**Read/write is the important split.** Read tools are cheap to get wrong. Write tools
need authorization, idempotency keys, audit logs, and often human approval — the model
is a non-deterministic caller and you must design as if it will sometimes call the
wrong thing with the wrong arguments.

---

## 5. What the LLM is actually good at

Having listed the failures, be fair about the strengths, because this determines what
to build:

- Summarising, rewriting, tone-shifting, translating
- Classification and extraction from messy text
- Turning unstructured language into structured data (the entire FDE bread-and-butter)
- Drafting code, queries, configs
- Deciding *which* tool fits a request (routing/reasoning)
- Explaining and teaching

Notice the video's chosen example — summarise a support ticket — sits squarely in the
strength zone. That is not an accident. Good LLM product design means picking tasks
where fuzziness is acceptable and exactness is delegated.

---

## 6. Checkpoint questions

1. A teammate proposes fine-tuning the model nightly on your order database so it can
   answer "where is order #8842?". Give two reasons this is wrong and name the right
   design.
2. Why does the model get `2 + 2` right but `87345 × 5623` wrong? What changed?
3. You ask a model to write an acrostic poem where the first letters spell a word, and
   it gets it wrong. Which of the three failure mechanisms is this?
4. Which tools in your current system would be read-only, and which would need an
   approval step?

→ Next: [`03-tool-calling-architecture.md`](03-tool-calling-architecture.md)
