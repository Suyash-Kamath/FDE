# Part 5 — Cheat Sheet, Corrections & Interview Q&A

> Companion to Parts [1](01-next-token-prediction-and-tokenization.md), [2](02-embeddings-and-vectors.md), [3](03-attention-and-transformers.md), [4](04-generation-logits-softmax-decoding.md).

---

## 1. The whole thing in twelve lines

1. Text is split into **tokens** (subword chunks) by a deterministic, non-AI **tokenizer**.
2. Each token becomes a **token ID** — an arbitrary integer, a row number in a vocabulary.
3. Each ID is looked up in the **embedding matrix** → a vector of `d_model` floats.
4. **Positional information** is injected, because attention alone is order-blind.
5. Each **transformer layer** projects every token into **Q, K, V** via three learned matrices.
6. **Attention scores** = `Q·Kᵀ / √d_k`, causally masked, softmaxed into weights summing to 1.
7. Each token's new vector = **weighted sum of all value vectors** → contextualised meaning.
8. **Multi-head** attention runs many of these in parallel, each capturing a different relation.
9. A **feed-forward network** processes each position independently; residual connections accumulate everything.
10. **Stacking layers** builds progressively richer relationships (syntax → semantics → reasoning).
11. The **last token's final vector** is multiplied by the unembedding matrix → **logits** over the whole vocabulary.
12. **Temperature → top-k/top-p → softmax → sample** picks one token. Append it and repeat.

---

## 2. Formula reference

| Concept | Formula |
|---|---|
| Language modelling objective | `P(t₁…tₙ) = Π P(tᵢ \| t₁…tᵢ₋₁)` |
| Entropy | `H = −Σ pᵢ log₂ pᵢ` |
| Perplexity | `2^H` |
| Training loss | `L = −log p(correct token)` (cross-entropy) |
| Embedding lookup | `x = E[token_id]` |
| Q, K, V | `Q = XW_Q`, `K = XW_K`, `V = XW_V` |
| Attention | `softmax(QKᵀ/√d_k + M) · V` |
| Multi-head | `Concat(head₁…head_h) · W_O` |
| Transformer block | `x = x + Attn(LN(x))` ; `x = x + FFN(LN(x))` |
| FFN | `W₂ · GELU(W₁x + b₁) + b₂`, hidden = 4·d_model |
| Logits | `z = h_final · W_U` |
| Temperature | `p = softmax(z / T)` |
| Softmax | `pᵢ = e^{zᵢ} / Σⱼ e^{zⱼ}` |
| Cosine similarity | `(a·b)/(‖a‖‖b‖)` |
| RoPE property | `q'_m · k'_n` depends only on `(m−n)` |
| Attention cost | `O(n² · d)` |
| KV cache size | `2 · layers · heads · d_head · seq_len · batch · bytes` |

---

## 3. Parameter budget of a typical decoder-only model

For `L` layers, `d = d_model`, `V` vocabulary:

| Component | Params | 7B example (`L=32, d=4096, V=32000`) |
|---|---|---|
| Token embeddings | `V·d` | 131 M |
| Attention (`W_Q,W_K,W_V,W_O`) per layer | `4d²` | 67 M × 32 = 2.1 B |
| FFN per layer | `8d²` (4d hidden, two matrices) | 134 M × 32 = 4.3 B |
| LayerNorms | negligible | ~0.3 M |
| LM head | `d·V` (or tied) | 131 M |
| **Total** | ≈ `12·L·d² + 2Vd` | **≈ 6.7 B** ✓ |

Memorise the shape: **≈ ⅓ attention, ≈ ⅔ feed-forward.**

---

## 4. Where the lecture simplifies (and the precise version)

None of these make the lecture wrong — they're the places where the simplification will trip you up later.

| Lecture says | More precisely |
|---|---|
| "LLMs predict the next word" | Next **token**. And they don't predict *a* token — they output a full **probability distribution**; the sampler picks. |
| "For each token we create three vectors Q, K, V" | There's **one** embedding per token. Q, K, V are computed from it by three **learned projection matrices** shared across all tokens, separately in every layer and every head. |
| "Compare the query vector with key vectors to get a relevance score" | Correct, plus two essential steps: divide by `√d_k` (stops softmax saturating), then **softmax across all positions** so the weights sum to 1. Also the causal mask blocks future tokens. |
| `on` attends to `mat` | In a decoder-only model `on` **cannot** see `mat` — it comes later. The mask forbids it. That relationship is learned when `mat` is processed. |
| "There is no supervised learning here" | Pretraining is **self-supervised** — labels (the next token) come free from the data, but there *is* a target and a loss. Chat models then add **SFT** (genuinely supervised) and **RLHF/RLAIF**. |
| "Nobody knows how it adjusts its weights" | We know exactly: backpropagation + Adam, fully specified calculus. What nobody knows is **what the learned features mean** — that's interpretability, a different problem. |
| "200 characters → multiply by 4 to estimate tokens" | **Divide** by 4. 200 chars ≈ 50 tokens. (The rule itself is only a rough English-text average — see Part 1 §11.) |
| "GPT-5 has a richer vocabulary so it understands more" | Its **tokenizer** has a larger vocabulary (~200k vs ~50k), so text compresses into fewer tokens. Understanding comes from the model weights, not the tokenizer. They're trained separately. |
| "GPT-3 doesn't understand emoji" | Byte-level BPE encodes 🔥 as its 4 UTF-8 bytes. Older tokenizers had no merges for those byte sequences, so it took 3 tokens and the debug UI shows `���`. Nothing to do with comprehension. |
| Temperature "increases the gap" | At `T<1`, logit gaps widen and probability concentrates on the leader. At `T>1`, gaps shrink and the distribution flattens. **The ranking never changes at any temperature.** |
| Sampling described as "lottery tickets" | Exactly right, and the formal name is **inverse transform sampling**. Production systems additionally truncate the tail with top-k / top-p first. |
| "Guardrails stop it from saying rude things" | Guardrails are not in the sampler. They come from RLHF/SFT (which pushes those tokens' probabilities near zero), system prompts, and external classifiers. |
| Transformer diagram = "multiple neural layers" | A transformer block is specifically: LayerNorm → multi-head attention → residual → LayerNorm → FFN → residual. The FFN holds ~⅔ of the parameters. |

---

## 5. Glossary

| Term | Meaning |
|---|---|
| **Token** | A subword chunk of text; the model's atomic unit |
| **Token ID** | Integer index of a token in the vocabulary — arbitrary, nominal |
| **Vocabulary (V)** | The full set of tokens a model knows; 50k–200k |
| **Tokenizer** | Deterministic program mapping text ↔ token IDs (BPE / WordPiece / Unigram) |
| **Fertility** | Average tokens per word for a language under a tokenizer |
| **Embedding** | Vector representation of a token |
| **`d_model`** | Width of the residual stream; size of each token's vector |
| **Static embedding** | Context-free vector from the lookup table |
| **Contextual embedding** | Hidden state after transformer layers; depends on the whole sequence |
| **Positional encoding** | Mechanism injecting order (sinusoidal / learned / RoPE / ALiBi) |
| **Q, K, V** | Query ("what am I looking for"), Key ("what can I be found by"), Value ("what do I contribute") |
| **Attention weights** | Softmaxed, scaled relevance scores; sum to 1 per query |
| **Causal mask** | `−∞` on future positions, so tokens can't see ahead |
| **Head** | One independent attention operation within a layer |
| **Induction head** | Head implementing "saw `A B` before, now `A` again → predict `B`"; basis of in-context learning |
| **Residual stream** | The additive `d_model`-wide bus every sublayer reads from and writes to |
| **FFN / MLP** | Per-position two-layer network; holds most parameters and most factual knowledge |
| **LayerNorm / RMSNorm** | Activation normalisation for stable training |
| **Logits** | Raw unbounded scores over the vocabulary, before softmax |
| **Softmax** | Converts logits to a probability distribution |
| **Temperature** | Divisor applied to logits; controls distribution sharpness |
| **Greedy decoding** | Always take the argmax; deterministic |
| **Sampling** | Draw from the distribution; introduces variety |
| **Top-k / Top-p** | Truncate the distribution before sampling (fixed count / cumulative mass) |
| **Autoregressive** | Output is fed back as input, one token at a time |
| **KV cache** | Stored keys/values of previous tokens, so each step is O(n) not O(n²) |
| **Prefill / decode** | Parallel prompt processing vs sequential token generation |
| **TTFT / TPOT** | Time to first token / time per output token |
| **Perplexity** | `2^entropy`; effective number of choices the model is deciding between |
| **Self-supervised** | Labels derived from the data itself (the next token) |
| **SFT / RLHF** | Supervised fine-tuning / reinforcement learning from human feedback — the post-pretraining stages that make a chat assistant |
| **Context window** | Max tokens (prompt + output) the model can attend over |
| **GQA / MQA** | Share K/V across heads to shrink the KV cache |
| **MoE** | Mixture of Experts — route each token to a few of many FFNs |

---

## 6. Interview / viva questions with answers

**1. In one sentence, what does an LLM do?**
Given a sequence of tokens, it outputs a probability distribution over the next token.

**2. Why tokens and not words?**
A word-level vocabulary would be millions of entries (huge embedding + softmax matrices), would still hit out-of-vocabulary words, and would learn nothing shared between `automate` and `automation`. Subwords cap the vocabulary while keeping sequences short and morphology shareable.

**3. Why not characters?**
Sequences get 4–5× longer, attention is O(n²) so cost grows ~20×, and each token carries almost no meaning, wasting early layers on reassembly.

**4. Are token IDs meaningful numbers?**
No — nominal labels, like roll numbers. Meaning appears only after the embedding lookup, where geometry is learned.

**5. Why do two models tokenize the same string differently?**
Different vocabulary sizes, different tokenizer training corpora, different algorithms/pre-tokenizer regexes. Token counts are not portable across providers.

**6. Why can't an LLM count the r's in "strawberry"?**
It never sees characters — only subword tokens like `str`/`aw`/`berry`. Character-level questions are structurally hard for it.

**7. What is an embedding?**
A learned vector representation of a token; a row of the `V × d_model` embedding matrix, positioned so that semantic relationships become geometric ones.

**8. Static vs contextual embedding?**
Static is the lookup-table vector, identical everywhere the token appears. Contextual is the hidden state after attention layers, different in every context. `bank` has one static vector and infinitely many contextual ones.

**9. Why does word order matter, and how does the model know it?**
Attention is a weighted sum, which is permutation-equivariant, so raw attention can't distinguish "dog bites man" from "man bites dog". Order is injected by positional encodings (learned absolute, sinusoidal, RoPE, ALiBi).

**10. What is attention, in one sentence?**
A soft dictionary lookup: compare one token's query against every token's key, softmax the scores into weights, and return the weighted sum of the values.

**11. What are Q, K and V?**
Three learned linear projections of the same token vector: what I'm looking for, what I can be matched by, what I contribute if matched.

**12. Why three vectors instead of one?**
(a) One vector makes relevance symmetric, but linguistic relations are directional; (b) a token's strongest match would always be itself; (c) "how relevant am I" and "what do I contribute" are genuinely different quantities (think `not` in "not good"); (d) it lets the model view the same token through three task-specific subspaces.

**13. Why divide by √d_k?**
Dot products of `d_k`-dimensional vectors have standard deviation ~√d_k. Unscaled, softmax saturates into near-one-hot, killing both blending and gradients.

**14. What is the causal mask and why?**
`−∞` added to future positions before softmax, so position `i` can only attend to ≤ `i`. It prevents the model from trivially seeing the answer during training and lets every position in a document be a training example in one parallel pass.

**15. Why multi-head attention?**
A single softmax must split 100% of its attention among competing relationships. Multiple heads let syntax, coreference, and positional patterns be tracked simultaneously at the same total compute.

**16. What's in a transformer block besides attention?**
LayerNorms, residual connections, and a feed-forward network that holds ~⅔ of the parameters and most of the model's stored factual knowledge.

**17. Why stack many layers?**
Each layer refines the previous layer's representation. Early layers handle local syntax, middle layers do entity/fact resolution, later layers do task-level reasoning. Depth compounds because layer N attends over already-contextualised vectors.

**18. What are logits?**
The raw, unbounded scores produced by multiplying the final hidden state with the unembedding matrix — one per vocabulary token, interpretable as unnormalised log-probabilities.

**19. What does softmax do?**
Exponentiates and normalises logits into a probability distribution. Monotonic (never reorders) and exponential (magnifies gaps).

**20. Explain temperature from first principles.**
Divide logits by `T` before softmax. `T<1` widens logit gaps → mass concentrates on the top token → deterministic and repetitive. `T>1` shrinks gaps → flatter distribution → more diverse and eventually incoherent. `T=0` reduces to argmax. The ranking is never changed.

**21. Greedy vs sampling?**
Greedy takes the argmax: deterministic, fast, prone to repetition loops, and locally-optimal rather than globally-optimal. Sampling draws from the distribution: varied and natural, but can hit the long tail — which is why top-k/top-p truncation exists.

**22. Top-k vs top-p?**
Top-k keeps a fixed number of candidates regardless of confidence. Top-p keeps the smallest set covering `p` of the probability mass, so it adapts — narrow when the model is sure, wide when it isn't.

**23. Why does ChatGPT stream?**
Because generation is genuinely autoregressive: token `n+1` doesn't exist until token `n` is produced and fed back. Streaming just forwards each token as it appears, cutting perceived latency.

**24. Why do output tokens cost more than input tokens?**
Prompt processing (prefill) is one parallel, compute-bound pass. Generation (decode) is sequential and memory-bandwidth-bound — every output token requires reading the whole model's weights from GPU memory again.

**25. Model vs decoding strategy — why does it matter?**
The weights produce a fixed distribution; the decoder chooses from it. Variability, "creativity", and reproducibility are decoding-side properties you control with API parameters, not properties of the model. Constrained JSON output, for instance, is implemented by masking invalid tokens' logits — no retraining involved.

**26. Same prompt, `temperature=0`, different outputs. How?**
Floating-point non-associativity in GPU reductions, batch composition, MoE routing, or a silent model update. Near-tied top logits flip and the difference cascades. Use `seed` and pinned model versions; expect approximate, not bit-exact, reproducibility.

---

## 7. Exercises that actually build intuition

1. Open a tokenizer playground. Tokenize your own name, `automatically`, `strawberry`, an emoji, a Hindi sentence, and a JSON blob. Note the token counts and where splits land.
2. Compute softmax by hand for logits `[3, 1, 0]` at `T = 0.5`, `1`, `2`. Confirm the ranking never changes.
3. Write a 30-line pure-Python top-p sampler over a hardcoded distribution. Run it 1,000 times and check the empirical frequencies match the probabilities.
4. Implement single-head attention in NumPy for a 4-token, `d=8` toy input. Print the `4×4` weight matrix and confirm each row sums to 1. Then add the causal mask and confirm the upper triangle is zero.
5. Call any LLM API with the same prompt at `T=0` five times, then at `T=1.2` five times. Diff the outputs.
6. Take a paragraph, count its characters, divide by 4, then compare against the real token count. Repeat in a second language.

---

## 8. Primary sources worth reading

- **Attention Is All You Need** — Vaswani et al., 2017. The original architecture.
- **Language Models are Few-Shot Learners** — Brown et al., 2020 (GPT-3). Where in-context learning was documented.
- **Neural Machine Translation of Rare Words with Subword Units** — Sennrich et al., 2015. BPE for NLP.
- **The Curious Case of Neural Text Degeneration** — Holtzman et al., 2019. Why greedy/beam fail and nucleus sampling works.
- **The Illustrated Transformer** — Jay Alammar. The best visual explanation available.
- **Let's build GPT: from scratch, in code** — Andrej Karpathy. Two hours, and you will have written one.
- **A Mathematical Framework for Transformer Circuits** — Elhage et al., Anthropic, 2021. Residual stream, induction heads, QK/OV circuits.
- **RoFormer** — Su et al., 2021. RoPE.
- **FlashAttention** — Dao et al., 2022. How the O(n²) memory problem was tamed.
