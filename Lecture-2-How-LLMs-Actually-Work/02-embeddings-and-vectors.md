# Part 2 — Embeddings, Vector Space & Why Order Matters

> Continues from [Part 1](01-next-token-prediction-and-tokenization.md). We now have a list of integers. We need meaning.

---

## 1. The problem embeddings solve

After tokenization we have `[464, 3139, 286, 3794, 318]`. These integers are **arbitrary labels** — Part 1 §8.1. There is no sense in which `3794` is "near" `3139`.

But we need the model to know that `king` and `queen` are related, that `Delhi` and `Mumbai` are the same kind of thing, that `bank` might be a building or a riverside. We need a representation where **distance means something**.

An embedding is that representation: every token ID is mapped to a point in a high-dimensional space, and the model *learns where to put each point* so that useful relationships become geometric relationships.

---

## 2. The mechanics: from ID to vector

The model holds an **embedding matrix** `E` of shape `(V × d_model)`:

```
                d_model = 4096 columns
              ┌───────────────────────────────┐
  ID    0     │ 0.02  -0.31   0.77  …   0.11  │
  ID    1     │-0.44   0.09  -0.02  …   0.63  │
  ID    2     │ 0.15   0.51   0.38  …  -0.20  │
   …          │              …                │
  ID 99999    │ 0.71  -0.08   0.44  …   0.02  │
              └───────────────────────────────┘
                V = 100,000 rows
```

"Embedding a token" = **selecting a row**. Formally it's written as a matrix multiply with a one-hot vector:

```
one_hot(3794) · E   =   row 3794 of E
[0,0,…,1,…,0]  (1×V)  ×  E (V×d)  =  vector (1×d)
```

but every implementation just does an array index (`E[3794]`), which is why it's called a lookup table.

**Size:** with `V = 100,000` and `d_model = 4096`, that's 409.6 million parameters *just for the dictionary*. These are learned like every other weight — by gradient descent.

---

## 3. The king / queen / banana walkthrough, done properly

Imagine a world with exactly 4 dimensions: `royalty`, `food`, `living`, `technology`. Each token gets a score on each axis.

| Token | royalty | food | living | technology |
|---|---|---|---|---|
| **king** | 0.95 | 0.01 | 0.91 | 0.02 |
| **queen** | 0.94 | 0.01 | 0.92 | 0.02 |
| **banana** | 0.01 | 0.95 | 0.20 | 0.01 |
| **laptop** | 0.01 | 0.00 | 0.00 | 0.97 |

Reading this table:

- `king` and `queen` are **almost the same point**. Their difference lies on a single unnamed axis (gender), which our 4 dimensions don't capture — which is precisely why 4 dimensions is not enough.
- `banana` is far from both, and near nothing they're near.
- `laptop` is far from everything here except in one dimension.

The vector `[0.95, 0.01, 0.91, 0.02]` *is* the meaning of `king` as far as the model is concerned. Not a definition, not a dictionary entry — a position.

### 3.1 Measuring similarity

Two standard operations:

**Dot product** — cheap, unbounded, sensitive to vector length:
```
a · b = Σᵢ aᵢbᵢ
king · queen = 0.95(0.94) + 0.01(0.01) + 0.91(0.92) + 0.02(0.02) = 1.730
king · banana = 0.95(0.01) + 0.01(0.95) + 0.91(0.20) + 0.02(0.01) = 0.201
```

**Cosine similarity** — normalises out length, gives a value in `[-1, 1]`:
```
cos(a,b) = (a · b) / (‖a‖ · ‖b‖)
```
```
‖king‖   = √(0.9025+0.0001+0.8281+0.0004) = √1.7311 = 1.3157
‖queen‖  = √(0.8836+0.0001+0.8464+0.0004) = √1.7305 = 1.3155
cos(king, queen)  = 1.730 / (1.3157 × 1.3155) = 0.9995   ← nearly identical
‖banana‖ = √(0.0001+0.9025+0.0400+0.0001) = √0.9427 = 0.9709
cos(king, banana) = 0.201 / (1.3157 × 0.9709) = 0.157    ← unrelated
```

Attention (Part 3) uses the **dot product**. Vector databases for RAG usually use **cosine**.

### 3.2 The famous property: vector arithmetic

Word embeddings learn *directions* that correspond to relationships:

```
vec("king") − vec("man") + vec("woman")  ≈  vec("queen")
vec("Paris") − vec("France") + vec("Italy") ≈ vec("Rome")
vec("walked") − vec("walk") + vec("swim") ≈ vec("swam")
```

Nobody programmed a "gender direction" or a "capital-of direction". They emerged because in the training corpus, `king` sits in the same syntactic and semantic slots as `queen`, differing systematically by the same contexts that distinguish `man` from `woman`. This is the **distributional hypothesis**: a word's meaning is determined by the company it keeps.

(In modern LLMs these analogies are messier than in the classic Word2Vec results, but the underlying geometry is the same idea.)

---

## 4. How the dimensions are actually learned

The lecture is right that we don't pick the dimensions. Let's be precise about what does happen:

1. `E` is initialised **randomly** (small Gaussian noise).
2. The model is shown a chunk of text and asked to predict each next token.
3. The loss (cross-entropy — Part 4) measures how wrong the prediction was.
4. **Backpropagation** computes the gradient of the loss with respect to *every* parameter, including every number in `E`.
5. An optimiser (Adam / AdamW) nudges each parameter slightly in the direction that reduces loss.
6. Repeat trillions of times.

Over training, tokens that appear in similar contexts get pushed toward similar positions, because that's what minimises prediction error. Nobody labels anything. The "labels" are the next tokens, which come free with the text — this is called **self-supervised learning**, not unsupervised.

> ⚠️ Correction to a common phrasing: it is *not* true that "nobody knows how the model adjusts its weights". We know the algorithm exactly — it's calculus (chain rule) plus Adam, and it's fully deterministic given the data order. What we don't know is **what the learned dimensions mean**. That's the interpretability problem, and it's a different claim.

### 4.1 Real dimensionalities

| Model | d_model | Layers | Heads |
|---|---|---|---|
| GPT-2 small | 768 | 12 | 12 |
| GPT-2 XL | 1600 | 48 | 25 |
| GPT-3 175B | 12,288 | 96 | 96 |
| Llama-2 7B | 4,096 | 32 | 32 |
| Llama-2 70B | 8,192 | 80 | 64 |

So the lecture's "thousands of dimensions" is exactly right. GPT-3 describes every token with 12,288 numbers.

### 4.2 Are the dimensions interpretable?

Mostly no, for a specific reason: **superposition**. Models represent far more features than they have dimensions, by storing them as *directions* that are not axis-aligned and that overlap slightly. A single neuron/dimension typically participates in many unrelated features (polysemanticity).

Some directions *are* findable with effort — researchers have located directions encoding sentiment, truthfulness, "is this a country", refusal behaviour, and so on. Sparse autoencoders are the current tool for pulling interpretable features out of superposition. But you cannot open `E` and read column 47 as "royalty".

---

## 5. Static vs contextual embeddings — the crucial distinction

This is where the lecture's `bank` example lands, and it's the hinge between Part 2 and Part 3.

```
Sentence A:  "I deposited money at the bank."
Sentence B:  "I sat on the bank of a river."
```

- The **tokenizer** produces the same token for `bank` in both: ID `1001`.
- The **embedding lookup** produces the same vector for both: `E[1001]`.

So at the input, the model genuinely cannot tell them apart. Identical numbers.

The static embedding `E[1001]` is best understood as a **blend of all senses of "bank"**, weighted by how often each sense appears in training data — plus the syntactic information that it's a noun. It's an average, a starting point.

**Attention's job is to move that vector.** After the first transformer layer, the vector at the `bank` position in sentence A has absorbed information from `deposited` and `money`; in sentence B it has absorbed `sat`, `river`. By the final layer, the two vectors are in completely different regions of space.

```
                    input embedding        after layer 12
Sentence A "bank":     [same]      →      [near: finance, account, teller]
Sentence B "bank":     [same]      →      [near: shore, edge, riverside]
```

That's the whole idea:

| | Static embedding | Contextual embedding |
|---|---|---|
| Where | Embedding matrix `E` | Output of transformer layers |
| Depends on | Token ID only | Token ID + entire surrounding sequence |
| One per | Vocabulary entry | Token *occurrence* |
| Examples | Word2Vec, GloVe, `E[i]` in a transformer | ELMo, BERT, every hidden state in GPT |

Historically: Word2Vec (2013) gave us static embeddings. ELMo (2018) and BERT (2018) made them contextual. That jump is most of why modern NLP works.

---

## 6. Why word order matters, and how the model is told

### 6.1 The problem

```
"Dog bites man"   →  [23, 34, 56]
"Man bites dog"   →  [56, 34, 23]
```

Same tokens. Same token IDs. Same embeddings. **Completely opposite meaning.**

Now here is the uncomfortable fact: **self-attention has no notion of order.** It computes a weighted sum over all positions. Addition is commutative. If you shuffle the input rows, you get the same output rows, just shuffled — the operation is *permutation-equivariant*. A pure transformer would read "Dog bites man" and "Man bites dog" as the same bag of words.

(Contrast with RNNs, which are inherently sequential and got order for free — but paid for it with no parallelism.)

So order must be **injected explicitly**. The model needs two pieces of information per position: *what* token, and *where* it is.

### 6.2 Absolute positional encoding — sinusoidal (original "Attention Is All You Need")

Add a fixed vector to each token embedding, computed from the position:

```
PE(pos, 2i)   = sin( pos / 10000^(2i/d_model) )
PE(pos, 2i+1) = cos( pos / 10000^(2i/d_model) )

x_input = E[token_id] + PE(pos)
```

Each dimension is a sinusoid of a different wavelength — from ~2 positions to ~10,000 positions. Together they form a unique "fingerprint" per position, like a binary counter written in continuous values. Two useful properties:

- Nothing to learn (no parameters).
- `PE(pos + k)` is a **linear function** of `PE(pos)`, so the model can in principle learn relative offsets.

### 6.3 Learned absolute positions (GPT-1/2/3, BERT)

Just have a second lookup table `P` of shape `(max_positions × d_model)` and learn it:

```
x_input = E[token_id] + P[pos]
```

Simple and effective — but it hard-caps your context length at `max_positions`, and positions never seen during training are meaningless.

### 6.4 RoPE — Rotary Position Embedding (Llama, Mistral, most modern models)

Instead of *adding* a position vector, **rotate** the Q and K vectors by an angle proportional to their position, in 2D subspaces:

```
q'_m = R_m · q_m        where R_m rotates by angle m·θ
k'_n = R_n · k_n

q'_m · k'_n  depends only on (m − n), the RELATIVE distance
```

That last line is the magic: the dot product between a query at position `m` and a key at position `n` automatically encodes their relative distance, with no extra parameters. RoPE also extrapolates to longer contexts reasonably well (and can be stretched further with tricks like NTK-aware scaling / YaRN — this is literally how "128k context" versions of 4k-trained models are made).

### 6.5 ALiBi

Even simpler: add a linear penalty to attention scores proportional to distance:
```
score(i,j) = qᵢ·kⱼ − m · |i − j|
```
Nearby tokens are favoured; far tokens are penalised. Extrapolates to arbitrary lengths.

### 6.6 Summary table

| Method | Where applied | Relative? | Extrapolates? | Used by |
|---|---|---|---|---|
| Sinusoidal | Added to input | Weakly | Somewhat | Original Transformer |
| Learned absolute | Added to input | No | No | GPT-2/3, BERT |
| RoPE | Rotates Q,K in every layer | Yes | Yes (with scaling) | Llama, Mistral, Qwen, most 2023+ |
| ALiBi | Bias on attention scores | Yes | Yes | BLOOM, MPT |

**Takeaway:** the lecture's claim — "the model needs tokens *plus* positions" — is exactly right, and positional encoding is the mechanism.

---

## 7. Token embeddings vs the "embeddings" in a vector database

These are related but not the same thing, and confusing them causes real bugs.

| | Token embedding | Sentence / document embedding |
|---|---|---|
| Input | One token | A whole sentence, paragraph or document |
| Produced by | Lookup in `E` (or a hidden state) | A dedicated encoder model (`text-embedding-3`, `bge`, `e5`, sentence-transformers) |
| Output | One vector per token | **One** vector for the whole text |
| Typical dims | 768–12,288 | 384–3,072 |
| Used for | Input to the transformer | Semantic search, RAG retrieval, clustering, dedup |

A sentence embedding is usually produced by running an encoder over the text and pooling (mean-pooling the token vectors, or taking a `[CLS]` position), then normalising, then trained with a contrastive objective so that semantically similar texts land close together under cosine similarity.

**Practical rules:**
- Your embedding model and your generation model are different models with different tokenizers. Chunk with the embedding model's tokenizer.
- Never mix vectors from two different embedding models in the same index. The spaces are unrelated.
- Cosine similarity between embeddings of different lengths is fine; cosine between embeddings from different models is meaningless.

---

## 8. Putting Part 2 together

After this stage, a sequence like `The cat sat on the mat` is:

```
token:    The     cat     sat      on      the     mat
id:        23      34      45      56      67      78
          ┌──┐    ┌──┐    ┌──┐    ┌──┐    ┌──┐    ┌──┐
E[id]:    │0.2│   │…│     │…│     │…│     │0.2│   │…│      ← note: "The" and "the"
          │0.4│   │…│     │…│     │…│     │0.4│   │…│         may or may not be the
          │-0.5│  │…│     │…│     │…│     │-0.5│  │…│         same token, but if they
          └──┘    └──┘    └──┘    └──┘    └──┘    └──┘        are, same vector
           +       +       +       +       +       +
          pos0    pos1    pos2    pos3    pos4    pos5
```

A matrix of shape `(6 × d_model)`. Six independent points in space, each knowing what it is and where it sits, but **none of them knowing anything about the others yet**.

Making them talk to each other is attention.

---

**Next:** [Part 3 — Attention, Q/K/V and the Transformer](03-attention-and-transformers.md)
