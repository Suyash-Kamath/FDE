# Vectors and Embeddings — Lecture Notes

*Based on your transcript and Excalidraw diagram. The timestamps are approximate places to revisit in the lecture.*

## 1. The central question

When you hear **Batman**, you may think of Gotham, Bruce Wayne, superheroes, darkness, and action. You also know that **Batman** is more closely related to **Superman** than to **Banana**.

How can a computer represent that relationship with numbers?

Giving each word a number is insufficient:

```text
Batman  → 43
Superman → 23
Banana  → 12
```

These are **identifiers**. The fact that `43` is closer to `23` than to `12` tells us nothing about the words’ meanings. The numbers were assigned arbitrarily. [00:00–00:05]

A computer can check whether two strings are identical:

```text
"Batman" == "Batman" → true
```

But exact matching does not capture that these two different sentences are related:

```text
"I forgot my password"
"I lost my credentials"
```

The lecture moves from asking **“Are these strings identical?”** to **“How similar are these things?”**

> **Remember:** A token ID identifies a token. Its numerical value does not, by itself, express the token’s meaning.

## 2. First attempt: one meaningful number per movie

Imagine a movie recommendation system. Give each movie an **action score**:

| Movie        | Action score |
| ------------ | -----------: |
| Avengers     |           10 |
| Batman       |            9 |
| Interstellar |            6 |
| Titanic      |            2 |
| Hera Pheri   |            2 |

These are illustrative scores. Unlike an arbitrary ID, each number now has a defined meaning.

Avengers and Batman are `|10 − 9| = 1` apart. Avengers and Hera Pheri are `|10 − 2| = 8` apart. If we recommend movies with nearby action scores, Batman is a closer match for Avengers. [00:12–00:19]

### Why one number fails

Titanic and Hera Pheri both have an action score of `2`, so their distance on this single axis is zero. But they are very different movies.

**Zero distance means they match on the one property we recorded.** It does not mean they are identical movies. We need more information. [00:18–00:20]

## 3. Add another property: vectors and dimensions

Add a **comedy score**. Write each movie in the same order:

```text
[action, comedy]

Avengers   = [10, 7]
Batman     = [9, 1]
Hera Pheri = [2, 10]
```

A **vector** is an ordered collection of numbers. Each position is a **dimension**:

```text
Avengers = [10, 7]
             │   │
             │   └─ comedy
             └───── action
```

The order matters. `[2, 10]` describes low action and high comedy; `[10, 2]` describes high action and low comedy. [00:19–00:24]

With two dimensions, we can imagine each movie as a point on a graph: action on one axis, comedy on the other. Titanic and Hera Pheri can now separate along the comedy axis even though they had the same action score.

### Add more dimensions

Titanic may score low in both action and comedy, while scoring high in **romance**. So we can use:

```text
[action, comedy, romance]
```

The diagram then gives this six-dimensional teaching example:

```text
Batman = [9, 1, 2, 9, 8, 7]
          │  │  │  │  │  └─ mystery
          │  │  │  │  └──── violence
          │  │  │  └─────── darkness
          │  │  └────────── romance
          │  └───────────── comedy
          └──────────────── action
```

We can draw one, two, or three dimensions. We struggle to picture six or a thousand dimensions, but mathematical comparisons still work. Adding a **useful** dimension can capture a difference that the earlier representation missed. [00:24–00:29]

When movies are represented with the **same coordinate scheme**, they occupy a common **vector space**. The recommendation system can compare their positions and select suitable matches. [00:28–00:32]

## 4. Three ways to compare vectors

The transcript and diagram introduce **Euclidean distance**, **cosine similarity**, and **dot product**. [00:32–00:40]

| Measure            | Main question                                     | How to read it                                 |
| ------------------ | ------------------------------------------------- | ---------------------------------------------- |
| Euclidean distance | How far apart are the positions?                  | Smaller distance means closer points.          |
| Cosine similarity  | How similar are their directions?                 | A score near `+1` means the same direction.    |
| Dot product        | What do direction and magnitude produce together? | It changes with both angle and vector lengths. |

### Euclidean distance

In two dimensions, the straight-line distance between `(x₁, y₁)` and `(x₂, y₂)` is:

$$
d=\sqrt{(x_2-x_1)^2+(y_2-y_1)^2}
$$

For the lecture’s Avengers `[10, 7]` and Batman `[9, 1]`:

$$
d=\sqrt{(10-9)^2+(7-1)^2}
 =\sqrt{37}
 \approx 6.08
$$

Their **action-only** difference was `1`. Their distance changes when we also include comedy. That is the purpose of representing more than one property. [00:21–00:22; 00:32–00:33]

### Why distance is sometimes insufficient

The lecture compares two users’ preferences, ordered as `[action, comedy]`:

```text
User A = [1, 10]    scores out of 10
User B = [10, 100]  scores out of 100
```

Both express the same action-to-comedy ratio, `1:10`. Yet their coordinates are far apart because they used different scales.

User B’s vector is exactly ten times User A’s. They point in the **same direction**, despite having different lengths. This motivates cosine similarity. [00:33–00:35]

### Cosine similarity

For nonzero vectors \(A\) and \(B\):

$$
\operatorname{cosineSimilarity}(A,B)
=\frac{A\cdot B}{\lVert A\rVert\lVert B\rVert}
$$

The intuition is to compare **directions**:

* `+1`: same direction
* `0`: perpendicular directions
* `−1`: opposite directions

User A `[1, 10]` and User B `[10, 100]` have cosine similarity `1` because they point in precisely the same direction. Cosine similarity ignores the difference caused solely by multiplying one nonzero vector by a positive number. [00:35–00:38]

### Dot product

The diagram gives:

$$
A\cdot B=\lVert A\rVert\lVert B\rVert\cos\theta
$$

Here, \(\theta\) is the angle between the vectors. The dot product depends on their direction **and** lengths. The lecture’s advice is to choose a comparison based on what matters for the particular problem. [00:39–00:40]

**A precision point for revision:** The `−1` to `+1` range applies to *cosine similarity scores*. It does not mean every individual coordinate of an embedding must lie in that range. Also, a cosine score of `0` tells you the vectors are perpendicular; it does not, on its own, prove the real-world concepts are unrelated.

## 5. Why manually choosing features becomes difficult

So far, humans decided that movies should have features such as action, comedy, romance, and darkness. Humans also assigned the scores. The diagram calls these **handcrafted vectors** and links them to **feature engineering**. [00:40–00:43]

The lecture identifies three problems.

### Problem 1: How many features?

Should we include thriller, science fiction, space, family themes, time, VFX, and more? Where do we stop? People may also disagree about a movie’s score for any given feature. [00:42–00:44]

### Problem 2: Useful features depend on the task

To recommend movies by viewing taste, genre features might help. To compare their box-office prospects, budget and actor popularity might matter more.

**The same object can need different representations for different questions.** [00:44–00:46]

### Problem 3: Language depends on context

“Bank” might mean a financial institution or the side of a river. Its meaning depends on the sentence.

Words such as **King**, **Queen**, and **Banana** also invite very different human labels. If we give each one its own set of dimensions, we cannot directly compare corresponding positions. If we force every possible label into one enormous shared list, manual scoring becomes unwieldy. [00:46–00:49]

This leads to the lecture’s next idea:

> Instead of choosing and scoring every coordinate ourselves, let a model learn useful numerical representations.

The desired result is a space where King and Queen are meaningfully related, while King and Banana are less related. We do not need a simple human name for every coordinate to use the result. [00:49–00:52]

## 6. How the lecture explains learned embeddings

The teaching example uses four words:

```text
King
Queen
Banana
Apple
```

Initially, assign each word a random two-dimensional vector. Those starting positions have no useful meaning. King might accidentally appear closer to Banana than to Queen. [00:52–00:53]

### Learn from context

The lecturer shows repeated patterns such as:

```text
The king ruled the kingdom.
The queen ruled the kingdom.

The king lived in the palace.
The queen lived in the palace.

The banana is a fruit.
The apple is a fruit.
```

King and Queen appear in similar contexts. Banana and Apple do too.

The lecture captures the intuition with: **“You shall know a word by the company it keeps.”** Surrounding words give clues about a word’s meaning. Likewise, seeing “The king wears a crown” may help predict **crown** in “The queen wears a ____.” Similar context is useful evidence, although it does not mean two words share every property. [00:53–01:00]

### The simplified training loop

The diagram’s example goes like this:

1. Start with random vectors.
2. Give the model a prediction objective.
3. For **King**, imagine it incorrectly predicts **Banana** when the desired answer is **Queen**.
4. Compare its prediction with the desired answer to calculate **error**, or **loss**.
5. Adjust numerical parameters and repeat across many examples.
6. Over time, the learned positions become more useful: King relates to Queen, and Banana relates to Apple.

In this explanation, **learning means adjusting numbers to improve the objective**. It does not mean someone simply told the computer a dictionary definition and it understood immediately. [01:00–01:06]

This four-word story is a teaching simplification. The transcript does not specify every step of an actual embedding-training algorithm.

## 7. Vector versus embedding versus latent space

| Term                   | Meaning in the lecture                                                  | Example                                                   |
| ---------------------- | ----------------------------------------------------------------------- | --------------------------------------------------------- |
| **Vector**             | An ordered collection of numbers                                        | Avengers `[10, 7]`                                        |
| **Handcrafted vector** | A vector whose features and scores humans chose                         | Batman’s six named movie features                         |
| **Embedding**          | A learned vector representation meant to encode useful relationships    | A trained representation of King                          |
| **Latent dimensions**  | Learned coordinates without guaranteed individual human-readable labels | Coordinate 1 need not mean “royalty”                      |
| **Latent space**       | The space containing those learned representations                      | The conceptual King/Queen and Banana/Apple arrangement    |
| **Embedding model**    | A model that maps a suitable input to a vector of a configured size     | Text → a 1024-dimensional vector in the lecture’s example |

**Every embedding described here is a vector.** The word *embedding* emphasizes that the representation was learned so its relationships can be useful. [01:05–01:08]

In the handcrafted movie example, we know exactly what each coordinate means because we named it. In a learned embedding, we cannot assume that one coordinate equals “royalty” and another equals “fruit.” Meaning may be distributed across many coordinates. The lecture calls these dimensions **latent**, meaning hidden from a simple human interpretation.

The transcript discusses vector representations for words, sentences, longer text, images, videos, movies, and users. Its simplified embedding model is:

```text
Suitable input → embedding model → fixed-dimensional vector
```

The `1024` dimensions mentioned in the lecture are an example, not a requirement for every model. [01:02–01:08]

## 8. Uses and the lecture’s boundary

The clearest practical use in the lecture is **recommendation**:

1. Represent movies using a shared vector scheme.
2. Compare a watched movie with candidate movies.
3. Select candidates according to the chosen similarity measure.

This also gives the basic idea behind **similarity search**: represent a query and candidates as vectors, then find the most similar candidates. [00:15–00:29]

Your transcript and diagram do **not** separately teach the mechanics of an embedding table, embedding lookup, a document retrieval pipeline, a clustering algorithm, or a vector index. Those topics need another lesson; the underlying ideas here are vectors, learned representations, and comparison.

## Quick revision

1. **Why isn’t `Batman → 43` enough?**
   Because `43` is an arbitrary identifier with no defined semantic scale.

2. **Why did Titanic and Hera Pheri look identical in the first system?**
   The system recorded only their action scores, which were both `2`.

3. **Why are `[2, 10]` and `[10, 2]` different?**
   The position of each number determines which property it describes.

4. **Why can `[1, 10]` and `[10, 100]` be similar under cosine similarity?**
   They point in the same direction, despite having different lengths.

5. **Does coordinate 1 of a learned embedding necessarily mean “royalty”?**
   No. Learned dimensions need not have individual human-readable labels.

**One-sentence takeaway:** An embedding is a learned vector representation that places inputs in a space where useful relationships can be compared mathematically.
