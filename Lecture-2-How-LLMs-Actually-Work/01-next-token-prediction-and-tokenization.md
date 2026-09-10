# Part 1 — Next-Token Prediction & Tokenization

> Deep-dive notes from the "How LLMs Work" lecture (Forward Deployed Engineering course), expanded well past what the video covers.

---

## 1. The one-line answer, then the honest answer

**One line:** An LLM is a function that takes a sequence of tokens and returns a probability distribution over what the next token should be.

**Honest version:** An LLM is a very large parameterised function

```
f_θ : (t₁, t₂, …, tₙ)  →  p ∈ ℝ^V ,   Σ pᵢ = 1
```

where `V` is the vocabulary size (~50k–200k), `θ` is the set of learned weights (billions of floating point numbers), and `p` is a probability distribution over every token the model knows.

Three things follow from this definition, and almost every practical fact about LLMs is a consequence of one of them:

1. **The model itself never "chooses" a word.** It outputs a distribution. Something *outside* the model — the decoder / sampler — picks a token from that distribution. (This is the Model-vs-Decoding-Strategy distinction in Part 4.)
2. **The model has no memory.** It is a pure function of its input sequence. "Conversation history" is just text that gets re-fed every single turn.
3. **Everything is a token, not a word.** The unit of thought is a token ID. This causes a long list of weird behaviours you have probably already seen.

---

## 2. The prediction game, and what it is really measuring

The lecture starts with a game. Let's walk it again, but track *what changes* each time.

| Prompt | Plausible next words | Rough count of plausible options |
|---|---|---|
| `I drink coffee every ___` | morning, evening, night, day, weekend, few (hours) | ~15–30 |
| `I drink coffee every morning before going to ___` | office, work, gym, college, school, class, bed | ~8–12 |
| `I am a software engineer.`<br>`Every morning I drink coffee before going to ___` | office, work, my (desk), standup | ~4 |
| `I work remotely as a software engineer.`<br>`I drink coffee before going to ___` | my (desk), work, standup | ~2–3 |

Notice what actually happened: **the set of legal continuations never became empty and never became a single word. It just got narrower.** That narrowing has a name.

### 2.1 Entropy — the real quantity being reduced

Given a distribution `p` over the vocabulary, the *entropy* is

```
H(p) = − Σᵢ pᵢ · log₂ pᵢ      (measured in bits)
```

- If the model is certain (`p = [1, 0, 0, …]`), `H = 0` bits.
- If the model is clueless (uniform over 100,000 tokens), `H = log₂(100000) ≈ 16.6` bits.
- A well-trained LLM on ordinary English text sits around **1.5–4 bits per token**.

Adding context is *literally* reducing entropy. "I am a software engineer" removed the probability mass sitting on `school` and `college` and redistributed it onto `office` and `work`. That is all "understanding context" means in mathematical terms.

### 2.2 Perplexity — the number you see in papers

```
Perplexity = 2^H(p)
```

Perplexity is "the effective number of options the model is choosing between". Perplexity 4 means the model is, on average, as confused as if it were picking uniformly between 4 words. Lower is better. It is the standard metric for how good a language model's raw prediction is.

### 2.3 Why "just predicting the next token" is not a small claim

The joint probability of an entire document factorises by the **chain rule of probability**:

```
P(t₁, t₂, …, tₙ) = P(t₁) · P(t₂|t₁) · P(t₃|t₁,t₂) · … · P(tₙ | t₁…tₙ₋₁)
```

Every term on the right is exactly "predict the next token given everything before". So a perfect next-token predictor is a perfect model of *the entire probability distribution of human text*. To predict the token after

```
"The murderer, as Poirot finally revealed in the drawing room, was ___"
```

you must have modelled the plot. To predict the token after

```
"def is_prime(n):\n    for i in range(2, int(n**0.5)+1):\n        if n % i == ___"
```

you must have modelled Python semantics. Next-token prediction is a *training objective*, not a ceiling on capability. This is the single most misunderstood point about LLMs.

---

## 3. The full pipeline (memorise this)

```
"The capital of India is"
        │
        ▼
┌──────────────────┐
│   TOKENIZER      │   outside the neural network — plain deterministic code
│  text → tokens   │
│  tokens → IDs    │
└──────────────────┘
        │  [464, 3139, 286, 3794, 318]
        ▼
┌──────────────────┐
│ EMBEDDING LOOKUP │   ID → vector of d_model floats   (Part 2)
│  + POSITION INFO │
└──────────────────┘
        │  matrix of shape (5 tokens × 4096 dims)
        ▼
┌──────────────────┐
│ TRANSFORMER      │   N stacked blocks:
│ BLOCK × N        │   self-attention (Part 3) + feed-forward
└──────────────────┘
        │  contextualised vectors, same shape
        ▼
┌──────────────────┐
│  LM HEAD         │   take LAST token's vector, project onto vocabulary
│  h → logits      │
└──────────────────┘
        │  logits: 100,000 raw scores
        ▼
┌──────────────────┐
│ TEMPERATURE →    │   Part 4
│ SOFTMAX → SAMPLE │
└──────────────────┘
        │
        ▼
    " Delhi"   ← append to input, repeat the whole loop
```

Everything in this document is one box of that diagram.

---

## 4. Why we need numbers at all

Neural networks do exactly four things: multiply matrices, add biases, apply a non-linear function, and compare against a target. All four operate on floating point numbers. There is no operation in the entire stack that consumes the character `H`.

So before anything else, `"Hello"` must become numbers. That conversion happens **outside** the model, in a separate piece of software. When you use ChatGPT or Claude:

- you type into a frontend,
- the backend runs a tokenizer,
- the tokenizer emits a list of integers,
- *those integers* are what the model receives.

The tokenizer contains no AI. It is a lookup table plus a merge algorithm. You could write one in an afternoon.

---

## 5. Design attempt #1 — one token per word (and why it fails)

The obvious idea: give every word in the dictionary an ID.

```
cat → 1, dog → 2, apple → 3, car → 4, …
```

Four separate problems kill this:

### 5.1 Vocabulary explosion
English has ~170k words in current use, but real text contains far more: inflections (`run/runs/ran/running`), compounds, product names, usernames, typos, URLs, code identifiers (`getUserByIdOrThrow`). A realistic word-level vocabulary for the open web is millions of entries.

### 5.2 The cost is quadratic in the wrong places
Two matrices scale directly with `V`:

- **Embedding matrix:** `V × d_model`
- **Unembedding / LM head:** `d_model × V`

With `d_model = 4096`:

| Vocab size | Embedding params | Both matrices |
|---|---|---|
| 50,000 | 205 M | 410 M |
| 200,000 | 819 M | 1.6 B |
| 2,000,000 | 8.2 B | 16.4 B |

A 2M-word vocabulary would burn more parameters on the dictionary than on the actual reasoning layers. Worse, the final softmax must be computed over all `V` entries **for every generated token**.

### 5.3 Out-of-vocabulary (OOV) is fatal
Whatever cutoff you pick, the very next word the user types might be outside it. Word-level models handled this with a single `<UNK>` token — which throws the information away. A model that renders `Suyash`, `RabbitMQ` and `kubectl` all as `<UNK>` is useless for real work.

### 5.4 No morphological sharing
`automatic`, `automate`, `automation`, `automatically` are obviously related. Under word-level tokenization they get four unrelated IDs, and the model must learn each one's meaning independently, from scratch, from however many times it happened to appear. Rare words never get enough training signal.

---

## 6. Design attempt #2 — one token per character (and why that fails too)

Flip to the other extreme: 26 letters + digits + punctuation ≈ 100 tokens. Vocabulary problem solved permanently, OOV impossible.

But now:

### 6.1 Sequences explode
```
"Hi ChatGPT, my name is Aditya and I want to learn about
 forward deployed engineering."
```
That is ~95 characters → ~95 tokens instead of ~18.

### 6.2 Attention is O(n²)
Self-attention compares every token with every other token. Cost grows with the **square** of sequence length:

| Tokens | Pairwise comparisons |
|---|---|
| 18 | 324 |
| 95 | 9,025 |
| 1,000 | 1,000,000 |
| 8,000 | 64,000,000 |

A 5× longer sequence is ~25× more attention compute. Character-level would make a 100k-context model economically impossible.

### 6.3 Each token carries almost no meaning
The letter `e` has no semantics. The model must spend its early layers just re-assembling letters into words before it can start reasoning — wasting depth on a problem the tokenizer could have solved for free.

### 6.4 Long-range dependencies get further away
A coreference that spans 20 words spans 100+ characters. Every relationship the model must learn is stretched.

---

## 7. The compromise — subword tokenization (BPE)

Modern tokenizers sit deliberately in the middle:

> **Common sequences of characters get their own token. Rare words get split into reusable pieces.**

The dominant algorithm is **Byte-Pair Encoding (BPE)**, borrowed from 1994 data compression and applied to text by Sennrich et al. (2015).

### 7.1 How a BPE tokenizer is *trained* (this happens once, before the LLM exists)

1. Start with a vocabulary of all individual bytes (256 entries). Every possible input is now representable.
2. Take a huge corpus. Split it into characters.
3. Count every adjacent pair of symbols. Find the most frequent pair.
4. Merge it into a new single symbol. Record the merge in an ordered **merge table**. Add it to the vocabulary.
5. Repeat until the vocabulary reaches the target size (50k, 100k, 200k…).

**Toy worked example.** Corpus: `low low low lower lowest newer newest`

```
Start:  l o w · l o w · l o w · l o w e r · l o w e s t · n e w e r · n e w e s t

Pair counts:  "l o" = 5,  "o w" = 5,  "e r" = 2,  "e s" = 2,  "n e" = 2, …
Merge 1: (l, o) → "lo"        vocab += "lo"
Merge 2: (lo, w) → "low"      vocab += "low"
Merge 3: (e, r) → "er"        vocab += "er"
Merge 4: (e, s) → "es"        vocab += "es"
Merge 5: (es, t) → "est"      vocab += "est"
Merge 6: (n, e) → "ne"  …

Result:  "lowest"  →  ["low", "est"]        2 tokens, both reused elsewhere
         "newest"  →  ["ne", "w", "est"]
```

Notice the algorithm discovered the morpheme `est` **without anyone teaching it English morphology**. It is pure frequency statistics.

### 7.2 How a BPE tokenizer is *used* at runtime (encoding)

1. Split the input text on a regex (whitespace/punctuation pre-tokenizer).
2. Convert each chunk to bytes.
3. Apply the merges from the merge table **in the order they were learned**, greedily, until no more apply.
4. Look up each resulting symbol in the vocabulary → integer ID.

This is fully deterministic. Same tokenizer + same string = same IDs, always. That's why `" of"` is `286` in GPT-3 in *every* sentence it appears in — as the lecture demonstrates by writing two different lines and getting `286` both times.

### 7.3 Byte-level BPE and why emoji behave strangely

GPT-2 onwards use **byte-level** BPE: the base alphabet isn't characters, it's the 256 possible byte values. This guarantees *any* UTF-8 input can be encoded — no OOV is possible, ever.

But a fire emoji 🔥 is `U+1F525`, which is **4 bytes** in UTF-8 (`F0 9F 94 A5`). If the tokenizer never learned merges for that byte sequence, you get 3–4 tokens for one emoji, and the debug UI shows `���` because individual bytes aren't valid characters on their own. That's exactly the "your input contains unicode characters that map to multiple tokens" warning in the lecture.

Newer tokenizers with 200k vocabularies learned merges for common emoji, so 🔥 becomes a single token. Nothing about "understanding" changed — the merge table got bigger.

### 7.4 The other algorithms (so you recognise the names)

| Algorithm | Used by | Idea |
|---|---|---|
| **BPE** | GPT-2/3/4/5, Llama, Mistral | Greedily merge the most frequent pair |
| **WordPiece** | BERT, DistilBERT | Merge the pair that maximises corpus likelihood, not raw frequency; marks continuations with `##` |
| **Unigram LM** | T5, ALBERT (via SentencePiece) | Start with a huge vocab, *prune* pieces that hurt likelihood least; probabilistic, supports sampling different segmentations |
| **SentencePiece** | Llama, T5, many multilingual models | A *wrapper* that treats the raw string (including spaces, as `▁`) as input, so no language-specific pre-tokenizer is needed — critical for Japanese/Chinese/Thai which have no spaces |

---

## 8. Tokens vs Token IDs — keep them separate in your head

```
"Capital of India is Delhi"

TOKENS      →  ["Capital", " of", " India", " is", " Delhi"]     ← strings
TOKEN IDS   →  [ 39315,    286,    3794,    318,   16112     ]   ← integers
```

- **Token** = the chunk of text.
- **Token ID** = the row number of that chunk in the vocabulary file.

### 8.1 Token IDs are *nominal*, not *ordinal*

This is the point the lecture hammers with the Rohit(104) / Aditya(204) example, and it matters.

`cat = 1`, `car = 4` does **not** mean:
- car > cat
- car is 4× cat
- car and cat are close because 1 and 4 are close

IDs are labels, like roll numbers or Aadhaar numbers. Arithmetic on them is meaningless. `token_id` is a *categorical* variable that happens to be stored as an integer.

**Where the meaning actually lives:** in the embedding matrix (Part 2). Row 1 and row 4 of that matrix are 4096-dimensional vectors whose *geometry* is meaningful. So:

```
ID space:         arbitrary, no structure         ←  tokenizer's output
Embedding space:  rich geometric structure        ←  learned by the model
```

The one-hot encoding is the bridge: ID `4` becomes the vector `[0,0,0,1,0,…]`, and multiplying that by the embedding matrix is just "select row 4". The arbitrariness of the ID is intentionally destroyed at the very first layer.

### 8.2 The leading space is part of the token

This trips up everyone:

```
"of"    →  token 1659
" of"   →  token 286      ← different token!
"Of"    →  token 5189
" Of"   →  token 1041
```

Consequences you will actually hit:
- If your prompt ends with a trailing space, you have changed the tokenization of the *next* word, and usually degraded the model's output.
- Few-shot prompts should be formatted with the space attached to the *following* word, the way natural text is.
- Stop sequences must account for the leading space.

---

## 9. Why two models tokenize the same text differently

The lecture demonstrates:

| Text | GPT-3 (r50k, 50,257 tokens) | GPT-4 (cl100k, ~100,277) | GPT-5 (o200k, ~200,000) |
|---|---|---|---|
| `Elephants are huge` | 5 tokens (`Ele`,`ph`,`ants`,` are`,` huge`) | 4 | 4 |
| `automatically` | 2 (`autom`,`atically`) | 2 | 1 |
| 🔥 | 3 | 3 | 1 |
| `Co-founder of Coder Army` | 6 | 6 | — |

Three independent reasons:

1. **Vocabulary size.** More slots → more whole words fit as single tokens. This is the dominant factor. Roughly: doubling vocab reduces token count on English by ~10–15%, much more on other languages and code.
2. **Training corpus of the tokenizer.** A tokenizer trained with more code sees `getElementById` often and may merge it; one trained on books won't. A tokenizer trained with Devanagari text will tokenize Hindi efficiently.
3. **Algorithm and pre-tokenizer regex.** BPE vs Unigram produce different splits. The regex that decides where chunks can even begin (does a digit attach to a letter? are numbers split into 1-digit or 3-digit groups?) changes everything downstream.

**Important corollary:** token counts are not portable across providers. A prompt that is 1,000 tokens for GPT-4 might be 1,150 for Llama and 950 for Claude. Never hardcode a token budget computed with the wrong tokenizer.

---

## 10. Fertility, and the multilingual / Indian-name tax

**Fertility** = average tokens per word for a given language under a given tokenizer.

| Language / content | Approx. tokens per word (cl100k) |
|---|---|
| English prose | 1.2–1.3 |
| Code (Python) | 1.5–2.5 |
| Spanish / French / German | 1.5–2.2 |
| Hindi (Devanagari script) | 3–6 |
| Transliterated Indian names in Latin | 2–4 |
| Tamil, Telugu, Malayalam | 4–8 |

The lecture's own examples: `Aditya` → 3 tokens on GPT-3, 2 on GPT-5. `Rohit` → 3 tokens (`R`, `oh`, `it`).

Why this matters, concretely:

- **Cost.** You pay per token. The same meaning in Hindi can cost 4× what it costs in English.
- **Context window.** A 128k-token window holds ~96,000 English words but maybe ~25,000 Hindi words.
- **Latency.** More output tokens = more forward passes = slower response.
- **Quality.** Fragmented tokens carry weaker signal; models are measurably worse at languages their tokenizer handles badly. This is a well-documented equity problem in LLMs, and it is why Indic-focused labs build their own tokenizers rather than reusing OpenAI's.

---

## 11. "1 token ≈ 4 characters" — where it comes from, when it lies

The heuristic exists because on average English web text with the GPT tokenizers lands near ~4 characters or ~0.75 words per token. It is a *statistical average over a corpus*, useful for cost estimation:

```
200 characters ≈ 200 / 4 = 50 tokens        # rough budget estimate
```

It is **wrong as a rule**. Counter-examples from the lecture and beyond:
- `auto` = 1 token = 4 chars ✓
- `automatically` = 1 token = 13 chars (GPT-5) ✗
- `🔥` = 3 tokens = 1 character ✗
- ` の` = 1 token, `हैं` = 3+ tokens ✗
- A base64 blob = ~1 token per 2 chars ✗

Never use the heuristic for hard limits (truncation, chunking for RAG, billing guarantees). Call the actual tokenizer (`tiktoken` for OpenAI, the model's own tokenizer file otherwise).

---

## 12. Consequences of tokenization you have already experienced

This section is the payoff. Almost every "why is the LLM stupid about this?" moment traces back to tokens.

| Symptom | Tokenizer explanation |
|---|---|
| **Can't count the r's in "strawberry"** | The model never sees letters. It sees `str`,`aw`,`berry`. Asking for letter counts is like asking you to count the pixels in a word you read. |
| **Bad at reversing strings** | Same reason — character-level operations on subword units. |
| **Rhyming is harder than it should be** | Rhyme is phonetic and suffix-based; tokens don't align to phonemes. |
| **Arithmetic errors on long numbers** | `1234567` might split as `123`,`45`,`67` on one model and `1`,`234`,`567` on another. Digit grouping is inconsistent, so carrying is learned unreliably. (Newer tokenizers deliberately force 1- or 3-digit number groups to fix this.) |
| **Whitespace/indentation sensitivity in code** | Python indentation is tokenized; GPT-4's tokenizer added dedicated multi-space tokens specifically to make code cheaper and more reliable. |
| **Glitch tokens** (`SolidGoldMagikarp`, `davidjl`) | Strings that got a vocabulary slot from the tokenizer corpus but were then filtered out of the LLM's training data. Their embeddings never got trained, so they sit at random positions in vector space and cause bizarre outputs. Proof that tokenizer and model are trained separately. |
| **Trailing-space prompts perform worse** | You split the next word's natural token. |
| **Same prompt, different token count on different providers** | Different tokenizers (§9). |

---

## 13. Special tokens

The vocabulary contains entries that are not text at all:

| Token | Purpose |
|---|---|
| `<\|endoftext\|>` / `</s>` / EOS | Model emits this to say "I'm done". The generation loop stops here. |
| BOS | Marks the start of a sequence. |
| `<\|im_start\|>`, `<\|im_end\|>` | Chat-template delimiters that separate system / user / assistant turns. |
| PAD | Filler to make batch sequences equal length (masked out). |
| Tool/FIM tokens | Function calling, fill-in-the-middle for code models. |

**This is how "roles" work.** There is no magic "system prompt" mechanism inside the network. Your system prompt, user message and assistant reply are concatenated into a single string with special delimiter tokens, and the whole thing is fed as one sequence. The model learned during fine-tuning to respect those delimiters. That is also why prompt injection is possible: it is all one string, and the boundary is a learned convention, not a hardware barrier.

---

## 14. Detokenization

Going back is trivial: map IDs → token strings → concatenate. Because byte-level BPE preserves spaces inside the tokens themselves, `"".join(tokens)` reconstructs the original exactly. No post-processing, no guessing where spaces went. That's a deliberate design property, and it's why byte-level BPE beat earlier schemes that needed detokenization heuristics.

One catch for streaming: a single token may be a *partial* UTF-8 character (one byte of a 4-byte emoji). Streaming implementations buffer bytes until they form valid characters — otherwise you'd render `�` mid-stream.

---

## 15. Practical checklist for a backend engineer

```python
# Count tokens for real, don't estimate
import tiktoken
enc = tiktoken.get_encoding("cl100k_base")     # GPT-3.5/4
ids = enc.encode("Capital of India is Delhi")
print(len(ids), ids)
print([enc.decode([i]) for i in ids])          # see the actual chunks
```

- **Cost model:** `cost = input_tokens × price_in + output_tokens × price_out`. Output is typically 3–5× the price of input, because output requires one full forward pass *per token* while input is processed in one parallel batch (see Part 4, prefill vs decode).
- **Context budgeting:** reserve room for the output. `max_tokens` counts against the same window.
- **RAG chunking:** chunk by tokens, not characters, and use the *same* tokenizer as your embedding model — which is often different from your generation model's.
- **Truncation:** truncate on token boundaries, not string slices, or you'll split a multi-byte character.
- **Logging:** store token counts per request; it's the only reliable input to capacity planning.

---

**Next:** [Part 2 — Embeddings, vectors and why order matters](02-embeddings-and-vectors.md)
