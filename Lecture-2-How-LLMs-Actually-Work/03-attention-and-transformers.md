# Part 3 — Attention, Q/K/V and the Transformer

> Continues from [Part 2](02-embeddings-and-vectors.md). We have a matrix of independent token vectors. Now they need to talk.

---

## 1. The problem attention solves

### 1.1 Meaning is not local

```
Rohit gave Aditya his laptop.
```

What does `his` refer to? You said "Rohit" instantly. Now:

```
Rohit gave Aditya his laptop because his laptop was broken.
```

First `his` → Rohit. Second `his` → Aditya. Same token, same ID, same embedding, **opposite referent**, resolved only by reading the causal structure of the sentence.

A model that processes each token independently cannot do this. `his` in isolation carries: *possessive, third person, masculine singular*. That's it. Everything else must come from elsewhere in the sequence.

Generalising: **a token's useful representation is a function of the whole context, not of the token itself.** Attention is the mechanism that computes that function.

### 1.2 What came before, and why it lost

| Approach | How context flowed | Why it failed |
|---|---|---|
| **Bag of words / TF-IDF** | It didn't | No order, no syntax |
| **RNN** | Hidden state passed left→right, one step at a time | Sequential (no GPU parallelism); information from token 1 is degraded by token 100 |
| **LSTM / GRU** | Gated hidden state | Better memory, still sequential, still ~100s of tokens practical range |
| **Seq2seq + attention (Bahdanau 2014)** | Decoder attends over encoder states | Worked well — and revealed that *attention was doing the heavy lifting* |
| **Transformer (2017)** | Every token attends to every token, in parallel | Won |

Two properties made the transformer win:

1. **Path length is O(1).** In an RNN, information from token 1 must survive 99 sequential updates to reach token 100. In a transformer, they are one dot product apart. Gradients flow directly.
2. **Full parallelism during training.** Every position is computed simultaneously. This is what let people spend $100M of GPU time productively — the architecture scales with hardware.

The cost: **O(n²)** compute in sequence length. That trade was worth it, and the last five years of research (Flash Attention, sparse attention, sliding window, linear attention, Mamba/SSMs) is largely about reclaiming it.

---

## 2. Attention as a soft dictionary lookup

The cleanest mental model, and the one that makes Q/K/V obvious.

A normal Python dict:
```python
d = {"apple": 5, "banana": 3, "cherry": 9}
d["banana"]   # → 3
```
- `"banana"` is the **query**
- `"apple"`, `"banana"`, `"cherry"` are the **keys**
- `5`, `3`, `9` are the **values**
- Matching is *hard*: exactly one key matches, you get exactly one value.

Attention is the **soft** version:
- The query is compared to *every* key, producing a similarity score.
- Scores are normalised into weights summing to 1.
- The output is a **weighted blend of all values**.

```python
# pseudo-attention
scores  = {k: similarity(query, k) for k in keys}
weights = softmax(scores)                      # sums to 1
output  = sum(weights[k] * values[k] for k in keys)
```

You never retrieve one value. You retrieve a mixture, weighted by relevance. That mixture *is* the contextualised representation.

### 2.1 The Google-search analogy from the lecture

| Search | Attention |
|---|---|
| You type `best java course` | **Query vector** of the token being processed |
| Google compares it against its index entries | **Key vectors** of every token in the sequence |
| Some pages match strongly, some weakly | **Attention scores** |
| Google fetches the content of the matching pages | **Value vectors** |
| You read a blend of the top results | **Weighted sum of values = output** |

Where the analogy breaks (usefully): Google returns a ranked list and you pick one. Attention *always* blends everything, including the low-scoring entries at tiny weights. Nothing is discarded, it's just down-weighted. That matters — the lecture makes this point well: `on` relates most to `mat`, but it also has real (smaller) relationships to `cat` and `sat`, and all of them contribute.

---

## 3. Q, K, V — the actual computation

### 3.1 They are projections, not stored vectors

> ⚠️ **Important correction to the lecture's framing.** The lecture describes it as "for each token we form three vectors". That's the right intuition but slightly misleading about the mechanism. There is **one** embedding per token. Q, K and V are *computed from it* by three learned weight matrices that are **shared across all tokens**:

```
X   : (n × d_model)        input matrix — one row per token
W_Q : (d_model × d_k)      learned
W_K : (d_model × d_k)      learned
W_V : (d_model × d_v)      learned

Q = X · W_Q        (n × d_k)
K = X · W_K        (n × d_k)
V = X · W_V        (n × d_v)
```

So the three "roles" of a token are three different **linear views** of the same underlying vector. `W_Q`, `W_K`, `W_V` are part of `θ` — they're trained. Each transformer layer has its own set (and each head within a layer has its own set — §5).

Typical sizes in GPT-3: `d_model = 12288`, `d_k = d_v = 128` per head, 96 heads (96 × 128 = 12288).

### 3.2 The formula

```
Attention(Q, K, V) = softmax( (Q Kᵀ) / √d_k + M ) · V
```

Piece by piece:

| Term | Shape | Meaning |
|---|---|---|
| `Q Kᵀ` | `n × n` | Every query dotted with every key — the raw relevance matrix |
| `/ √d_k` | — | Scaling (§3.4) |
| `+ M` | `n × n` | Causal mask (§3.5) |
| `softmax(·)` | `n × n` | Each **row** becomes a probability distribution over positions |
| `· V` | `n × d_v` | Weighted sum of value vectors |

### 3.3 A fully worked numeric example

Sentence: `the cat sat`. Take `d_k = d_v = 2` for readability. Suppose the projections gave:

```
             q                k                v
the     [0.1, 0.2]      [0.2, 0.1]      [0.1, 0.0]
cat     [0.4, 0.9]      [0.9, 0.4]      [0.8, 0.3]
sat     [1.0, 0.5]      [0.6, 0.9]      [0.2, 0.9]
```

We process the token `sat`, so `q = [1.0, 0.5]`.

**Step 1 — raw scores (dot products):**
```
q · k_the = (1.0)(0.2) + (0.5)(0.1) = 0.25
q · k_cat = (1.0)(0.9) + (0.5)(0.4) = 1.10
q · k_sat = (1.0)(0.6) + (0.5)(0.9) = 1.05
```

**Step 2 — scale by √d_k = √2 = 1.4142:**
```
0.25 / 1.4142 = 0.1768
1.10 / 1.4142 = 0.7778
1.05 / 1.4142 = 0.7425
```

**Step 3 — softmax:**
```
e^0.1768 = 1.1934
e^0.7778 = 2.1767
e^0.7425 = 2.1012
sum      = 5.4712

w_the = 1.1934 / 5.4712 = 0.2181
w_cat = 2.1767 / 5.4712 = 0.3978
w_sat = 2.1012 / 5.4712 = 0.3840
                          ───────
                          1.0000  ✓
```

These are the **attention weights** — the lecture's `W1, W2, W3`.

**Step 4 — weighted sum of values:**
```
out = 0.2181·[0.1, 0.0] + 0.3978·[0.8, 0.3] + 0.3840·[0.2, 0.9]

x:  0.2181(0.1) + 0.3978(0.8) + 0.3840(0.2) = 0.0218 + 0.3182 + 0.0768 = 0.4168
y:  0.2181(0.0) + 0.3978(0.3) + 0.3840(0.9) = 0.0000 + 0.1193 + 0.3456 = 0.4649

out(sat) = [0.417, 0.465]
```

That vector replaces (well, gets added to — §6.2) the representation of `sat`. It is no longer "sat in the abstract"; it is "sat, in a context where a cat is the subject".

This is computed for **every** token simultaneously — that's what the matrix form `QKᵀ` does in one operation.

### 3.4 Why divide by √d_k

Suppose `q` and `k` have independent components with mean 0 and variance 1. Then

```
q · k = Σᵢ qᵢkᵢ        has mean 0 and variance d_k
                        → standard deviation √d_k
```

With `d_k = 128`, dot products routinely land at ±11 or larger. Feed logits of that magnitude into softmax and you get a near-one-hot distribution: one weight ≈ 1.0, everything else ≈ 0. Two bad consequences:

1. **No blending.** Attention degenerates into hard selection, losing the whole benefit of soft lookup.
2. **Vanishing gradients.** The gradient of softmax is `p(1−p)`; when `p ≈ 1` or `p ≈ 0`, the gradient is ≈ 0 and the layer stops learning.

Dividing by `√d_k` normalises the variance back to ~1, keeping softmax in its responsive range. It is a one-character fix that makes deep transformers trainable.

### 3.5 The causal mask — why GPT can't see the future

Decoder-only LMs (GPT, Llama, Claude) must not let position `i` attend to position `j > i`. Otherwise, during training, predicting token 5 would be trivial — the model could just look at token 5.

Implementation: add `−∞` to the upper triangle of the score matrix *before* softmax. `e^(−∞) = 0`, so those weights become exactly zero.

```
        the   cat   sat   on   the   mat
the  [  ✓    -inf  -inf  -inf  -inf  -inf ]
cat  [  ✓     ✓    -inf  -inf  -inf  -inf ]
sat  [  ✓     ✓     ✓    -inf  -inf  -inf ]
on   [  ✓     ✓     ✓     ✓    -inf  -inf ]
the  [  ✓     ✓     ✓     ✓     ✓    -inf ]
mat  [  ✓     ✓     ✓     ✓     ✓     ✓   ]
```

Two consequences worth internalising:

- **Training is massively efficient.** One forward pass over a 4,096-token document produces 4,096 next-token predictions at once, each correctly using only its own prefix. This is called *teacher forcing*, and it's why pretraining is feasible.
- **The lecture's `on` example is slightly idealised.** In a decoder-only model, `on` cannot attend to `mat`, because `mat` comes later. It attends to `the`, `cat`, `sat`, and itself. The `mat`→`on` relationship is captured in the *other* direction, when `mat` is processed. (BERT-style encoder models are bidirectional and *can* see both ways — they're used for classification/embedding, not generation.)

---

## 4. Why three separate vectors? (The deepest question in this topic)

The natural objection: we already have one vector per token. Why not just compare `vec(on)` with `vec(mat)` directly and use `vec(mat)` as the content? Why bother with three projections?

There are four distinct reasons. Understand all four and you understand attention.

### Reason 1 — A single vector forces symmetric relationships

With one vector per token, the score is `vec(a) · vec(b)`, which is **symmetric**: `a·b = b·a`. So "how much should `his` attend to `Aditya`" would necessarily equal "how much should `Aditya` attend to `his`".

But real linguistic relationships are directional:
- `his` desperately needs a preceding noun. `Aditya` doesn't particularly need a following pronoun.
- An adjective looks for the noun it modifies; the noun doesn't equally look back.
- A closing bracket looks for its opening bracket, strongly and specifically.

With separate `W_Q` and `W_K`, the score is `(x_a W_Q) · (x_b W_K)ᵀ = x_a (W_Q W_Kᵀ) x_bᵀ`. The matrix `W_Q W_Kᵀ` is **not symmetric**, so `score(a→b) ≠ score(b→a)`. Directionality is expressible.

### Reason 2 — A single vector makes every token attend mostly to itself

`vec(a) · vec(a) = ‖a‖²`, which is the largest dot product `a` can have with anything. Under a single-vector scheme, every token's strongest match would be itself, always. Attention would be a very expensive identity function. Separate Q and K projections break this: a token's query can point somewhere completely different from where its own key sits.

### Reason 3 — "How relevant are you" and "what do you contribute" are different questions

This is the cleanest separation, and it's why `V` exists apart from `K`.

Consider the token `not` in `"the design is not good"`. When `good` attends around:
- `not` should score **very high on relevance** — it completely determines the meaning. That's its **key**.
- But what `not` should *contribute* to `good`'s new representation is a negation signal — not the word `not` itself. That's its **value**.

Relevance-matching and content-delivery are decoupled. A token can be a strong retrieval trigger while carrying a small payload, or a weak trigger with a rich payload. One vector can't express both.

Same idea in search: a page's *title and keywords* (key) are what make it match your query; the *article body* (value) is what you actually read. You don't want them to be the same text.

### Reason 4 — The lecture's own analogy, formalised

The lecture describes one person — Aditya — represented three different ways: by technical skills, by location, by moral values. Same person, three projections into three different subspaces, each useful for a different question.

Formally, `W_Q`, `W_K`, `W_V` project the same `d_model`-dimensional token representation into three lower-dimensional subspaces (`d_k`, `d_k`, `d_v`), each optimised for its job:

```
                      ┌──── W_Q ───→  "what am I looking for?"     (query subspace)
x  (d_model = 4096) ──┼──── W_K ───→  "what can I be found by?"    (key subspace)
                      └──── W_V ───→  "what do I contribute?"      (value subspace)
```

The 4096-dim vector contains *everything* about that token. Each head only cares about a 128-dim slice relevant to one kind of relationship. The projection matrices are learned filters that extract the right slice.

**Bonus (the parameter-efficiency argument):** you could in principle learn one big `n × n` relevance function directly, but it would have to be relearned for every sequence length and would have `O(n²)` parameters. Factorising through `W_Q`/`W_K` gives a *length-independent*, low-rank, reusable relevance function.

---

## 5. Multi-head attention — one attention isn't enough

One attention operation produces one weighted average. But a token participates in **many** relationships at once:

```
"The cat that the dog chased sat on the mat"
```
For `sat`, simultaneously:
- syntactic subject → `cat`
- what's being sat on → `mat`
- relative clause boundary → `chased`
- tense/agreement → the whole clause

A single softmax must divide 100% of its attention among these. If it spends 60% on `cat`, only 40% remains for everything else. Different relationships **compete**.

**Solution:** run `h` independent attention operations in parallel, each with its own `W_Q^i, W_K^i, W_V^i`, each on a `d_model/h`-dimensional slice.

```
head_i = Attention(X W_Q^i, X W_K^i, X W_V^i)        for i = 1..h

MultiHead(X) = Concat(head_1, …, head_h) · W_O
```

- Each head has its own attention pattern — its own 100% to distribute.
- Concatenating gives back `d_model` dimensions.
- `W_O` (`d_model × d_model`) mixes the heads back together.

**Cost is roughly free:** with `h` heads of dimension `d_model/h`, total compute equals single-head attention at full dimension. You get specialisation for the same FLOPs.

### 5.1 What heads actually specialise in

Interpretability research has identified real, nameable heads in real models:

| Head type | What it does |
|---|---|
| **Previous-token head** | Attends almost entirely to position `i−1` |
| **Positional heads** | Fixed offsets, sentence starts, delimiters |
| **Syntactic heads** | Verb → its subject; noun → its determiner; preposition → its object |
| **Coreference heads** | Pronoun → its antecedent (the `his` → `Aditya` job) |
| **Duplicate-token heads** | Find earlier occurrences of the same token |
| **Induction heads** | The famous one: having seen `[A][B]` earlier, when `[A]` appears again, attend to and predict `[B]`. This is the mechanism believed to underlie **in-context learning** — the reason few-shot prompting works at all. |
| **Name-mover heads** | In the "indirect object identification" circuit, copy the correct name to the output |

GPT-3 has 96 layers × 96 heads = 9,216 attention heads. Most are not cleanly interpretable, but the ones that are, are strikingly modular.

---

## 6. The Transformer block — attention is only one component

A transformer layer is **not** just attention. Here is the full block (pre-LayerNorm variant, which is what modern models use):

```
        x  ─────────────────────────┐
        │                           │
   LayerNorm                        │  (residual)
        │                           │
  Multi-Head Self-Attention         │
        │                           │
        └────────── + ──────────────┘
                    │
        ┌───────────┴───────────────┐
        │                           │
   LayerNorm                        │  (residual)
        │                           │
   Feed-Forward Network (MLP)       │
        │                           │
        └────────── + ──────────────┘
                    │
                    ▼
              output → next layer
```

In code:
```python
x = x + attention(layernorm_1(x))
x = x + mlp(layernorm_2(x))
```

### 6.1 The feed-forward network (where most parameters live)

```
FFN(x) = W₂ · activation(W₁ · x + b₁) + b₂

W₁ : d_model → 4·d_model     (expand)
W₂ : 4·d_model → d_model     (contract)
activation: GELU / SwiGLU
```

For `d_model = 4096`: `W₁` is 4096×16384 = 67M params, `W₂` is another 67M. **~134M per layer, versus ~67M for the whole attention block.** Roughly **two-thirds of a transformer's parameters are in the FFNs**, not in attention.

What does it do? Attention *moves information between positions*. The FFN *processes information within a position*. Current best understanding: the FFN acts as a **key-value memory store** — `W₁` rows act as pattern detectors ("is this vector about Paris-and-capitals?") and `W₂` rows write out associated facts. Most of a model's factual knowledge is believed to live here, which is why model-editing techniques (ROME, MEMIT) target FFN weights.

Also crucial: it's the **non-linearity**. Stack a hundred attention layers with no FFN and you have a very expensive linear map.

### 6.2 Residual connections and the "residual stream"

`x = x + sublayer(x)` — the sublayer's output is *added* to the input rather than replacing it.

Why this matters:
- **Gradients flow.** The derivative of `x + f(x)` includes an identity term, so gradient reaches early layers even in a 96-layer stack. Without residuals, deep transformers simply don't train.
- **The mental model:** think of a **residual stream** — a `d_model`-wide "bus" running from input to output. Every attention head and every FFN *reads* from the bus, computes something, and *writes* its contribution back by addition. Nothing is ever overwritten; contributions accumulate.

This reframes the lecture's picture nicely: the token's vector isn't replaced at each layer, it's progressively *edited*.

### 6.3 LayerNorm

```
LN(x) = γ · (x − μ) / σ + β        (μ, σ computed over the d_model dimension)
```

Keeps activations at a stable scale so the next layer sees a consistent input distribution. **Pre-LN** (normalise before the sublayer) is now standard because it makes training stable without a learning-rate warmup. Modern models often use **RMSNorm**, which drops the mean-centring and is cheaper.

---

## 7. Stacking layers — what depth buys you

The lecture's photo-editing analogy is exactly right: a raw photo doesn't become a finished product in one step. Brightness → contrast → colour grade → retouch → compress.

Empirically, in a decoder-only LM:

| Depth | What tends to happen |
|---|---|
| **Layers 0–3** | Detokenization: reassembling multi-token words, local n-grams, part-of-speech, "is this the start of a sentence" |
| **Layers 4–12** | Syntax: subject-verb links, phrase boundaries, bracket matching, basic coreference |
| **Middle layers** | Semantics and factual recall: entity resolution, "Delhi is a capital", relational facts. Most FFN knowledge lookups fire here |
| **Later layers** | Task-level reasoning, in-context pattern matching (induction heads), integrating instructions |
| **Final layers** | Converging on the output distribution: the representation rotates toward the unembedding directions of likely next tokens |

Take the `Rohit gave Aditya his laptop because his laptop was broken` example:
- Layer 1 might link `laptop` to `his` (adjacent, syntactic).
- Layer 5 might resolve the *first* `his` to `Rohit` using subject position.
- Layer 20 might resolve the *second* `his` to `Aditya`, which requires understanding `because` and the causal logic of "you give someone a laptop when theirs is broken".

**Crucially, each layer's output becomes the next layer's input.** So layer 20 is not attending over raw word embeddings — it's attending over vectors that already encode 19 layers of accumulated relational structure. That compounding is where depth's power comes from.

Real depths: GPT-2 small = 12, GPT-3 = 96, Llama-2-70B = 80, GPT-4 (rumoured) = ~120.

---

## 8. Cost, and the tricks that manage it

### 8.1 The O(n²) wall

Attention builds an `n × n` matrix per head per layer.

| Context | Score matrix entries (per head/layer) | With 96 layers × 96 heads |
|---|---|---|
| 1k | 1 M | 9.2 T |
| 8k | 64 M | 590 T |
| 128k | 16.4 B | astronomically impractical naively |

### 8.2 KV cache (the single most important inference optimisation)

During generation, you produce one token at a time, re-running the model on the whole sequence each step. But the K and V vectors of all *previous* tokens don't change — they depend only on tokens that are already fixed.

So: **cache them.** Each new step computes Q, K, V for only the newest token, appends its K,V to the cache, and attends over the cached K,V.

- Without cache: step `t` costs O(t²). Generating `n` tokens costs O(n³).
- With cache: step `t` costs O(t). Generating `n` tokens costs O(n²).

**Cost:** memory. `2 × layers × heads × d_head × seq_len × batch × bytes`. For a 70B model at 8k context this runs into tens of GB — the KV cache, not the weights, is often what limits how many concurrent users a GPU can serve. This is why **MQA (multi-query attention)** and **GQA (grouped-query attention)** exist: share K and V across heads to shrink the cache 8–64×. Llama-2-70B and most modern models use GQA.

### 8.3 The rest of the toolbox

- **Flash Attention** — mathematically identical output, but tiles the computation so the `n×n` matrix is never materialised in slow GPU memory. 2–4× speedup, big memory saving.
- **Sliding-window attention** (Mistral) — each token attends only to the last `w` tokens; information travels further via layer stacking.
- **Sparse / strided attention** — attend to a fixed pattern of positions.
- **Mixture of Experts (MoE)** — replace the FFN with `N` experts, route each token to 2 of them. Big parameter count, small per-token compute.
- **State-space models (Mamba)** — O(n) alternative to attention entirely; strong, but transformers still dominate.

---

## 9. Architecture families (so the names stop being confusing)

| Family | Attention | Good at | Examples |
|---|---|---|---|
| **Encoder-only** | Bidirectional (no mask) | Understanding: classification, NER, embeddings, retrieval | BERT, RoBERTa, DeBERTa, most embedding models |
| **Decoder-only** | Causal (masked) | Generation | GPT-2/3/4/5, Llama, Mistral, Claude, Gemini |
| **Encoder-decoder** | Encoder bidirectional; decoder causal + cross-attention to encoder | Transduction: translation, summarisation | Original Transformer, T5, BART, Whisper |

The lecture describes a decoder-only model, which is what all modern chat LLMs are. "Attention Is All You Need" (Vaswani et al., 2017) introduced the encoder-decoder version for machine translation; the field then discovered that the decoder alone, scaled up, does everything.

**Cross-attention** (encoder-decoder only) is the same formula with Q coming from the decoder and K,V coming from the encoder. Worth knowing because it reappears in multimodal models: text queries attending over image patch keys/values.

---

## 10. End-to-end trace of one layer

Input: `The capital of India is` → 5 tokens.

```
1.  Tokenize            → [464, 3139, 286, 3794, 318]
2.  Embed + position    → X, shape (5 × 4096)

    ── Transformer Layer 1 ──
3.  LayerNorm(X)
4.  For each of 32 heads:
       Q = X·W_Q^i, K = X·W_K^i, V = X·W_V^i      (5 × 128 each)
       S = QKᵀ / √128                              (5 × 5)
       S += causal_mask                            (upper triangle → −inf)
       A = softmax(S, dim=-1)                      rows sum to 1
       head_i = A·V                                (5 × 128)
5.  Concat 32 heads → (5 × 4096); multiply by W_O
6.  X = X + that                                   ← residual
7.  LayerNorm(X) → FFN (4096→16384→4096)
8.  X = X + that                                   ← residual

    ── repeat for layers 2 … 32 ──

9.  Final LayerNorm
10. Take the LAST row only: h_final = X[4]         ← the vector for "is"
```

By step 10, `h_final` is a 4096-dimensional vector that encodes not "the word *is*", but **"this is the position immediately after a phrase asking for the capital of India, and the next thing should be a city name, specifically Delhi"**.

That vector is everything the model knows about what should come next. Part 4 turns it into a token.

---

**Next:** [Part 4 — Logits, Softmax, Sampling and Temperature](04-generation-logits-softmax-decoding.md)
