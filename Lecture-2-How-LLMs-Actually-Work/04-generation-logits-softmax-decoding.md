# Part 4 — Logits, Softmax, Sampling, Temperature & Generation

> Continues from [Part 3](03-attention-and-transformers.md). We have `h_final`, one context-saturated vector. Now we turn it into text.

---

## 1. From the final vector to logits

After the last transformer layer and a final LayerNorm, we take the hidden state at the **last position only**:

```
h_final ∈ ℝ^d_model            e.g. 4096 numbers
```

Then apply the **LM head** (also called the unembedding matrix):

```
logits = h_final · W_U          W_U : (d_model × V)

logits ∈ ℝ^V                    e.g. 100,000 numbers, one per vocabulary token
```

### 1.1 What this operation actually is

`W_U` has one **column per vocabulary token** — a `d_model`-dimensional vector representing "what a context looks like when this token should come next". The multiply computes a dot product between `h_final` and each of those columns:

```
logit_i = h_final · W_U[:, i]        = similarity between the context and token i
```

So the last step of an LLM is exactly what the lecture describes: **compare the final vector against every token's vector, and see which scores highest.** Not a mystery, just 100,000 dot products.

### 1.2 Weight tying

Many models set `W_U = Eᵀ` — the unembedding matrix *is* the transposed embedding matrix. Same weights used to read tokens in and write tokens out. Saves `V × d_model` parameters (409M for our example) and often improves quality. GPT-2 ties; GPT-3 and Llama do not.

### 1.3 Why only the last position?

At **inference**, only the last position's prediction is needed — the earlier positions are predicting tokens you already have.

At **training**, logits are computed at *every* position, because each one is a supervised prediction of the token that actually followed. A 4,096-token document yields 4,096 training signals from a single forward pass. This efficiency is why pretraining is possible at all.

---

## 2. Logits — raw, unbounded scores

```
logits = [ ..., "Delhi": 12.1, ..., "Kolkata": 8.1, ..., "banana": −4.3, ... ]
```

Properties:
- **Unbounded reals.** Can be negative, can exceed 20.
- **Not probabilities.** They don't sum to anything meaningful.
- **Interpretable as unnormalised log-probabilities.** Differences are what matter: a logit gap of `ln(10) ≈ 2.30` means one token is 10× more likely than the other.
- **Shift-invariant under softmax.** Adding a constant to every logit changes nothing.

Everything that shapes generation — temperature, top-k, top-p, repetition penalty, logit bias, grammar-constrained decoding — is an operation applied to this vector *before or after* softmax. This is the "control panel" of an LLM.

---

## 3. Softmax — turning scores into a distribution

```
                exp(zᵢ)
softmax(z)ᵢ = ─────────────
              Σⱼ exp(zⱼ)
```

Properties:
- Every output is in `(0, 1)`; they sum to exactly 1.
- **Monotonic** — never changes the ranking.
- **Exponential** — magnifies differences. A logit gap of 4 becomes a probability ratio of `e⁴ ≈ 55`.
- Nothing ever gets probability exactly 0 (unless masked to `−∞`). Every token in the vocabulary is always technically possible. This matters in §7.

### 3.1 Numerical stability

`exp(50)` overflows float32. Every real implementation subtracts the max first:
```python
z = z - z.max()          # shift-invariant, so the result is identical
p = np.exp(z) / np.exp(z).sum()
```

### 3.2 Worked example

Logits `[5, 4, 3, 2, 1]`:

```
e^5 = 148.413   e^4 = 54.598   e^3 = 20.086   e^2 = 7.389   e^1 = 2.718
sum = 233.204

p = [0.6364, 0.2341, 0.0861, 0.0317, 0.0117]        Σ = 1.0
```

Note how a linear gap of 1 in logit space becomes a ~2.7× ratio in probability space.

### 3.3 Softmax at training time = cross-entropy loss

During training, the loss for one position is
```
L = − log p(correct_token)
```
- Model gave the right token p=0.9 → loss 0.105 (small nudge)
- Model gave it p=0.01 → loss 4.6 (large nudge)

Averaged over trillions of tokens, minimising this is the entire pretraining objective. Note the connection to Part 1: average cross-entropy *is* entropy in nats, and `exp(loss)` is perplexity.

---

## 4. Temperature — from first principles

Temperature divides the logits **before** softmax:

```
p = softmax( z / T )
```

Order of operations, which the lecture correctly emphasises:

```
h_final → logits → ÷ T → softmax → probabilities → sample
                    ▲
                temperature acts HERE
```

### 4.1 Full worked comparison

Take logits `[5, 4, 3, 2, 1]` representing, say, `Python, Java, C++, Rust, Go`.

**T = 1.0** (unchanged):
```
scaled: [5, 4, 3, 2, 1]
probs:  [0.6364, 0.2341, 0.0861, 0.0317, 0.0117]
```

**T = 0.5** (colder → sharper). Divide by 0.5 = multiply by 2:
```
scaled: [10, 8, 6, 4, 2]
exp:    [22026.5, 2981.0, 403.4, 54.6, 7.4]     sum = 25472.9
probs:  [0.8647, 0.1170, 0.0158, 0.0021, 0.0003]
```

**T = 2.0** (hotter → flatter):
```
scaled: [2.5, 2.0, 1.5, 1.0, 0.5]
exp:    [12.182, 7.389, 4.482, 2.718, 1.649]    sum = 28.420
probs:  [0.4286, 0.2600, 0.1577, 0.0957, 0.0580]
```

Side by side:

| Token | logit | T=0.5 | T=1.0 | T=2.0 |
|---|---|---|---|---|
| Python | 5 | **86.5%** | **63.6%** | **42.9%** |
| Java | 4 | 11.7% | 23.4% | 26.0% |
| C++ | 3 | 1.6% | 8.6% | 15.8% |
| Rust | 2 | 0.2% | 3.2% | 9.6% |
| Go | 1 | 0.03% | 1.2% | 5.8% |

### 4.2 What to actually take away

1. **The ranking never changes.** Python is the top candidate at every temperature. Temperature only redistributes *how much* mass sits on the leader versus the tail. (The lecture's phrasing about "gaps increasing" is right in *logit* space at low T — the gaps go from 1 to 2 — and the corresponding probability mass concentrates. At T=2 the logit gaps shrink from 1 to 0.5 and the distribution flattens.)
2. **Low T = confident, deterministic, repetitive.** Mass collapses onto the argmax.
3. **High T = creative, diverse, and eventually incoherent.** As `T → ∞` the distribution becomes uniform over 100,000 tokens — i.e. gibberish.
4. **T → 0** would divide by zero; implementations special-case it to pure argmax (greedy).
5. **Entropy view:** temperature is a direct knob on the entropy of the output distribution. `T < 1` reduces entropy; `T > 1` increases it. The name comes from statistical physics — this is the Boltzmann distribution, where `T` is literal temperature.

### 4.3 Practical settings

| Task | Temperature | Why |
|---|---|---|
| Data extraction, classification, JSON output | 0 | You want the single most likely valid answer, reproducibly |
| Code generation | 0 – 0.3 | Correctness beats variety |
| Factual Q&A, RAG answers | 0 – 0.4 | Reduce fabrication and drift |
| Chat / general assistant | 0.7 – 1.0 | Natural, non-robotic |
| Brainstorming, story writing, ideation | 0.9 – 1.3 | Reach into the tail |
| Above ~1.5 | rarely useful | Coherence degrades fast |

---

## 5. Greedy decoding

```python
next_token = argmax(logits)
```

- Fully deterministic: same input → same output, every time.
- Fast, no randomness to manage.
- Equivalent to `temperature = 0`.

**Why it isn't the default for chat:** greedy decoding produces *degenerate repetition*. Once the model emits a phrase, that phrase becomes context, which raises the probability of emitting it again, which reinforces it further. You get loops:

```
I think that's a great idea. I think that's a great idea. I think that's...
```

It's also flatly wrong as a way to find the best *sequence*. Greedy maximises each token locally, which does not maximise the joint probability of the whole output. A slightly-lower-probability token now can unlock a far better continuation.

This is also why the lecture's ChatGPT demo gives two different answers to "hi how are you" — chat products don't use greedy.

---

## 6. Sampling

Instead of taking the max, **draw** from the distribution.

### 6.1 The mechanism (the lecture's lottery tickets, formalised)

This is **inverse transform sampling**:

1. Build the cumulative distribution.
2. Draw `r ~ Uniform(0, 1)`.
3. Return the first token whose cumulative probability exceeds `r`.

Example — the lecture's `hi how are you` case:

| Candidate | p | Cumulative | "Ticket range" (1–100) |
|---|---|---|---|
| `I'm fine` | 0.50 | 0.50 | 1–50 |
| `I'm doing well` | 0.40 | 0.90 | 51–90 |
| `Great` | 0.05 | 0.95 | 91–95 |
| everything else | 0.05 | 1.00 | 96–100 |

Draw `r = 0.56` → falls in 51–90 → `I'm doing well`.
Draw `r = 0.93` → falls in 91–95 → `Great`.

Same input, same distribution, different output. That is exactly the behaviour in the demo. **The model was 100% deterministic; the sampler was not.**

### 6.2 The problem with pure sampling: the tail

With `V = 100,000`, suppose the top 20 tokens hold 95% of the mass and the remaining 99,980 tokens split 5%. Individually each tail token has probability ~0.00005 — negligible. **Collectively they get picked 5% of the time.** Over a 500-token response, that's ~25 tail draws. Some will be nonsense.

This is the lecture's `shut up` example, and the mechanism is precisely right: nothing in sampling forbids a bad token, it just makes it unlikely. And "unlikely per token" becomes "likely somewhere in a long response".

(Sidenote: guardrails do **not** live in the sampler. Safety comes from training — SFT and RLHF push the probability of unwanted continuations near zero — plus system prompts and external classifiers. Sampling is neutral machinery.)

### 6.3 Truncation strategies — the actual fix

**Top-k sampling.** Keep only the `k` highest-probability tokens, renormalise, sample.
- `k=1` → greedy. `k=40` is a common default.
- Weakness: `k` is fixed regardless of how peaked the distribution is. After `The capital of France is`, the distribution is near-certain and `k=40` needlessly admits 39 wrong answers. After `Once upon a`, 40 may be too few.

**Top-p / nucleus sampling** (Holtzman et al., 2019) — the modern default.
- Sort tokens by probability, take the smallest set whose cumulative probability ≥ `p`, renormalise, sample.
- `p = 0.9` means "consider the tokens that make up the top 90% of the mass".
- **Adaptive:** on a confident distribution the nucleus might be 1–2 tokens; on an open-ended one, 200. That's the whole advantage over top-k.

```
sorted probs: 0.50, 0.30, 0.12, 0.05, 0.02, 0.01, ...
cumulative:   0.50, 0.80, 0.92 ← crosses p=0.9, stop
nucleus = {token1, token2, token3}, renormalised to sum to 1
```

**Min-p.** Keep tokens with `p ≥ min_p × p_max`. Scales the threshold with the model's own confidence; increasingly popular in open-source stacks.

**Typical sampling / eta / epsilon sampling.** Variants that target the tokens near the distribution's entropy rather than its top.

### 6.4 Repetition controls

- **Repetition penalty** — divide logits of already-seen tokens by `r > 1`.
- **Frequency penalty** — subtract a value proportional to how many times the token has appeared.
- **Presence penalty** — subtract a flat amount if the token appeared at all.
- **No-repeat n-gram** — hard-ban any n-gram that already occurred.

Use sparingly; aggressive penalties damage code and structured output, which legitimately repeat tokens.

### 6.5 Beam search — and why chat doesn't use it

Keep the `b` highest-probability *partial sequences*, extend all of them, keep the best `b`, repeat. Approximates the highest-joint-probability sequence rather than a greedy path.

- Great for translation and summarisation, where there's a single "correct" target.
- Bad for open-ended generation: the highest-probability sequence in English is bland and repetitive. Humans don't speak in maximum-likelihood sentences. Beam search output reads flat and generic.
- Also `b×` more expensive.

### 6.6 The stacking order

Real inference stacks apply these in sequence:

```
logits
  → logit_bias (manual per-token nudges)
  → repetition / frequency / presence penalties
  → ÷ temperature
  → top-k filter
  → top-p filter
  → softmax (renormalise over survivors)
  → sample
```

Beware of **double-tuning**: raising temperature *and* raising top-p compounds. Convention is to tune one and leave the other at default. Most providers recommend adjusting temperature **or** top_p, not both.

---

## 7. Autoregressive generation — the loop

```python
tokens = tokenize(prompt)

while True:
    logits  = model(tokens)[-1]        # last position only
    logits  = apply_penalties(logits)
    probs   = softmax(logits / T)
    probs   = truncate(probs, top_k, top_p)
    nxt     = sample(probs)

    if nxt == EOS_TOKEN:      break
    if len(generated) >= max_tokens: break
    if detokenize(generated).endswith(stop_sequence): break

    tokens.append(nxt)                 # ← the output becomes the input
    yield detokenize([nxt])            # ← streaming happens here
```

`tokens.append(nxt)` is the definition of **autoregressive**: the model's own output is fed back as input. The lecture's `The capital of India is` → `Delhi` → `.` → `which is also a Union Territory` sequence is exactly this loop running four times.

### 7.1 Consequences of the loop

- **Errors compound.** A wrong token becomes context the model must then be consistent with. This is a large part of why hallucinations, once started, get elaborated rather than corrected.
- **The model cannot revise.** There is no backspace. Once a token is emitted it is part of the prompt. (This is a core motivation for chain-of-thought: give the model tokens to "think in" *before* committing to an answer, because computation per token is fixed and the only way to spend more compute on a hard problem is to emit more tokens.)
- **Output length costs linearly in forward passes.** A 1,000-token answer means 1,000 full passes through 80 layers.

### 7.2 Prefill vs decode (and why output tokens cost more)

| Phase | What happens | Parallel? | Bottleneck |
|---|---|---|---|
| **Prefill** | Process the whole prompt, build the KV cache | Yes — all prompt tokens at once | Compute-bound (GPU FLOPs) |
| **Decode** | Generate one token, append to cache, repeat | No — strictly sequential | Memory-bandwidth-bound (must stream all weights from VRAM per token) |

A 2,000-token prompt is one big parallel matmul. A 2,000-token *output* is 2,000 sequential passes, each of which reads the entire model's weights from GPU memory. That asymmetry is why:
- output tokens are priced 2–5× input tokens,
- **TTFT** (time to first token) and **TPOT** (time per output token) are tracked as separate latency metrics,
- batching many users together improves throughput dramatically (weights get read once for the whole batch),
- long prompts are cheap-ish; long outputs are not.

### 7.3 Why ChatGPT streams

Now it's obvious: **the tokens genuinely arrive one at a time.** The server isn't holding back a finished answer and dribbling it out for effect — token 200 does not exist until token 199 has been generated and fed back.

Streaming is therefore free (server-sent events / chunked transfer) and drops perceived latency from "wait 12 seconds" to "wait 400ms". Practical notes for building on it:
- Buffer partial UTF-8 bytes so multi-byte characters don't render as `�` (Part 1 §14).
- Streaming and structured output conflict: you can't validate JSON until it's complete. Either stream to a tolerant parser or don't stream structured responses.
- Client disconnects should cancel generation server-side, or you pay for tokens nobody sees.

### 7.4 Stopping

Generation ends when:
1. The model samples the **EOS** token — it learned during fine-tuning where responses naturally end. This is the model deciding it's done.
2. `max_tokens` is hit — a hard cut, often mid-sentence. If your outputs are truncating, this is usually why.
3. A **stop sequence** matches (e.g. `"\nUser:"`).

---

## 8. Model vs Decoding Strategy — the conceptual separation

This is the highest-leverage idea in Part 4. Keep these two things apart in your head permanently:

| | **The Model** | **The Decoding Strategy** |
|---|---|---|
| What it is | Weights `θ` — the trained network | Sampling code around the model |
| What it produces | A probability distribution over the vocabulary | One chosen token |
| Changes by | Training / fine-tuning (expensive, slow) | An API parameter (instant, free) |
| Deterministic? | Yes — same input, same distribution | Not unless greedy |
| Knobs | none at runtime | temperature, top_k, top_p, penalties, seed, logit_bias, beams |

**Everything the lecture demonstrates about variability lives on the right-hand side.** The model gave the same distribution for `hi how are you` both times. The sampler picked differently.

Consequences worth internalising:

- **"The model is creative" is imprecise.** The distribution is fixed; you chose to sample from its tail. Creativity is a decoding decision.
- **"The model is hallucinating because temperature is high"** is only partly true. High temperature makes low-probability tokens more likely, which *increases* fabrication. But a model at `T=0` still hallucinates confidently, because the wrong answer genuinely has the highest probability. Temperature is not a truth knob.
- **Reproducibility.** For evaluations and regression tests, use `temperature=0`. Then differences between runs are your fault, not the sampler's.
- **Structured output.** Constrained/grammar-based decoding works by masking invalid tokens to `−∞` in the logits before softmax — a pure decoding-side intervention that makes malformed JSON *impossible* rather than merely unlikely. This is how `response_format: json_schema` and libraries like Outlines/llama.cpp grammars work.
- **`logit_bias`** lets you hand-tune specific tokens up or down. Useful for banning a word or forcing a choice between fixed options.

### 8.1 Why `temperature=0` still isn't perfectly reproducible

Even with greedy decoding, providers rarely guarantee bit-identical output:
- **Floating-point non-associativity** — GPU reductions sum in non-deterministic order, so logits differ in the last bits. When the top two tokens are nearly tied, that flips the argmax, and one different token cascades through the whole rest of the generation.
- **Batching** — your request is batched with others; batch composition changes kernel paths.
- **MoE routing** — in mixture-of-experts models, expert assignment can depend on the batch.
- **Silent model updates** — the endpoint behind a model name may change.

Use `seed` where offered, pin explicit model versions, and treat LLM outputs as *approximately* reproducible.

---

## 9. The complete picture, one page

```
"The capital of India is"
    │
    │  TOKENIZER (deterministic code, not AI)
    ▼
[464, 3139, 286, 3794, 318]
    │
    │  EMBEDDING LOOKUP + POSITIONAL ENCODING
    ▼
X : 5 × 4096 matrix
    │
    │  ┌─── TRANSFORMER LAYER 1 ─────────────────┐
    │  │  LN → Multi-Head Self-Attention → +res  │  tokens exchange info
    │  │  LN → Feed-Forward Network      → +res  │  each token processes info
    │  └─────────────────────────────────────────┘
    │  … × 96 layers, relationships get progressively richer …
    ▼
X : 5 × 4096, now fully contextualised
    │
    │  TAKE LAST POSITION ONLY
    ▼
h_final : 4096 numbers   ← contains the entire meaning of the prompt
    │
    │  LM HEAD:  h_final · W_U
    ▼
logits : 100,000 raw scores    [Delhi: 12.1, Kolkata: 8.1, banana: −4.3, …]
    │
    │  ÷ TEMPERATURE  →  TOP-K / TOP-P FILTER
    ▼
    │  SOFTMAX
    ▼
probabilities : [Delhi: 0.90, Kolkata: 0.05, …]  Σ = 1
    │
    │  SAMPLE (or argmax if greedy)
    ▼
" Delhi"
    │
    └──────► append to input, run the whole thing again
             (KV cache means previous work is reused)
```

**And the one-sentence definition it all reduces to:**

> Given the preceding tokens, produce a probability distribution over the next token. Everything else — Q/K/V, layers, softmax, sampling — is machinery in service of that, or a choice about what to do with the result.

---

**Next:** [Part 5 — Cheat sheet, corrections and interview questions](05-cheatsheet-corrections-and-qa.md)
