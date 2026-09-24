# Deep Dive into Vector Databases, ANN, IVF, HNSW & Product Quantization

## Master Notes Built from the Lecture Transcript + SVG Diagram + Existing Notes

---

# 0. The Big Picture

A modern AI application often converts unstructured information—text, images, audio, products, movies, documents, etc.—into **embeddings**.

An embedding is a vector such as:

```text
Interstellar -> [0.21, -0.45, 0.81, ...]
The Martian  -> [0.19, -0.41, 0.79, ...]
Titanic      -> [-0.61, 0.11, -0.08, ...]
```

The embedding model tries to arrange the vector space so that semantically related objects land close to one another.

The complete idea is:

```text
Meaning
  |
  v
Embedding model
  |
  v
Vector geometry
  |
  v
Similar things are close together
  |
  v
Vector search finds nearby points
```

This distinction is extremely important:

> **The embedding model creates the semantic geometry.  
> The vector database does not "understand" meaning; it stores and searches that geometry efficiently.**

For example, HNSW does not know that *Interstellar* and *The Martian* are space movies. It only sees vectors and distances between vectors.

---

# 1. The Fundamental Vector Search Problem

Suppose we have millions of movie embeddings.

```text
Movie 1 -> V1
Movie 2 -> V2
Movie 3 -> V3
...
Movie N -> VN
```

A user asks:

```text
"I want a space sci-fi movie"
```

The same embedding model converts that query into:

```text
Q = [q1, q2, q3, ..., qD]
```

Now the database has to answer:

> Which stored vectors are closest to `Q`?

If we want the best `K` results, the problem is called **K-Nearest Neighbor search**, commonly abbreviated KNN.

The simplest algorithm is:

```python
scores = []

for vector in vector_list:
    score = similarity(query_vector, vector)
    scores.append((score, vector))

scores.sort(reverse=True)

return scores[:k]
```

This is conceptually correct.

The problem is not correctness.

The problem is **scale**.

---

# 2. Exact Nearest Neighbor Search

In exact search, the query is compared against **every stored vector**.

```text
Query Q
   |
   +--> compare with V1
   +--> compare with V2
   +--> compare with V3
   |
   ...
   |
   +--> compare with VN
             |
             v
       keep the top K
```

If all distances are evaluated using the chosen metric, exact search can return the true mathematical nearest neighbors.

Therefore:

```text
Exact search
= maximum possible recall
= no candidate intentionally skipped
```

This is sometimes called:

- exact nearest-neighbor search
- brute-force vector search
- flat search

There is nothing mathematically wrong with it.

It simply becomes computationally expensive as `N` and `D` become large.

---

# 3. Why One Vector Comparison Is Not "One Operation"

Suppose:

```text
Q = [q1, q2, q3, ..., qD]
V = [v1, v2, v3, ..., vD]
```

For a dot product:

```text
Q . V =
q1*v1 +
q2*v2 +
q3*v3 +
...
+
qD*vD
```

For a 1,536-dimensional vector, one vector-to-vector comparison involves work across roughly 1,536 dimensions.

Therefore the naïve search cost grows approximately as:

```text
O(N * D)
```

where:

```text
N = number of stored vectors
D = dimensionality
```

Example:

```text
N = 10,000,000
D = 1,536

N * D
= 10,000,000 * 1,536
= 15,360,000,000
≈ 15.36 billion dimension-level operations
```

And this is for **one query** in the simplified model.

If thousands of users are searching simultaneously, the workload becomes huge.

### Important arithmetic correction from the lecture transcript

At one point, the transcript combines:

```text
100,000,000 vectors
1,536 dimensions
```

and describes that as roughly 15 billion operations.

The correct product is:

```text
100,000,000 * 1,536
= 153,600,000,000
= 153.6 billion
```

The prepared PDF uses the internally consistent example:

```text
10 million * 1,536 ≈ 15.36 billion
```

The conceptual lesson remains identical:

> Scanning every vector is extremely expensive at large scale.

---

# 4. Computing the Top K Efficiently Does Not Solve the Main Problem

After calculating all similarities, one simple implementation sorts all scores.

That can add approximately:

```text
O(N log N)
```

work.

A better implementation can maintain only the top `K` results using a heap.

That can reduce the ranking portion.

But there is a deeper issue:

> Even if top-K maintenance becomes cheap, exact search still computed a distance against every vector.

The dominant problem is often:

```text
N huge vectors
x
D dimensions
```

So the key optimization question becomes:

> Can we avoid examining most vectors at all?

This question leads directly to **Approximate Nearest Neighbor search**.

---

# 5. Why Ordinary Indexing Is Hard for Embeddings

For ordinary scalar data such as:

```text
age = 23
price = 1000
date = 2026-09-24
```

databases can build ordered structures such as B-trees.

For vector similarity, however, there is no single scalar ordering that perfectly preserves neighborhood relationships across hundreds or thousands of dimensions.

A simple spatial tree such as a k-d tree works nicely in low-dimensional spaces.

But as dimensionality becomes high, a phenomenon usually called the **curse of dimensionality** makes spatial partitioning less selective.

The search may be forced to visit many partitions.

Eventually the behavior can drift closer to brute force.

That is why modern high-dimensional vector retrieval commonly relies on ANN-specific indexing strategies instead.

---

# 6. Human Intuition Behind ANN

Imagine Rohit is standing in Delhi and asks:

```text
"Find the nearest coffee shop in India."
```

A naïve exact strategy would conceptually compare Rohit's location with every coffee shop in:

- Delhi
- Mumbai
- Bengaluru
- Chennai
- Assam
- Goa
- etc.

A human does not reason like this.

We immediately realize:

```text
Rohit is in Delhi
   |
   v
Delhi-area candidates are promising
   |
   v
Very distant regions can probably be ignored
```

ANN applies the same fundamental optimization idea to vector space.

Instead of asking:

> How do I calculate the distance to all 10 million vectors faster?

ANN asks:

> How do I intelligently eliminate most of those 10 million vectors before detailed comparison?

That is the conceptual breakthrough.

---

# 7. Approximate Nearest Neighbor — ANN

ANN stands for:

> **Approximate Nearest Neighbor**

Suppose exact search says the true top three are:

```text
1. Interstellar
2. The Martian
3. Gravity
```

An ANN search might return:

```text
1. Interstellar
2. The Martian
3. Apollo 13
```

It missed one mathematically closer vector.

However, for applications such as:

- recommendations
- semantic document retrieval
- RAG candidate retrieval
- image similarity
- product recommendation

the returned set can still be extremely useful.

The benefit is that ANN may examine a tiny fraction of the dataset.

---

# 8. What Does ANN Actually Approximate?

This is one of the most important concepts in the lecture.

A beginner may think:

```text
ANN calculates a fake/incorrect distance.
```

That is not necessarily what is happening.

A better mental model is:

```text
All vectors
    |
    v
ANN index performs candidate selection
    |
    v
small promising candidate set
    |
    v
proper distance calculations
    |
    v
Top K
```

Example:

```text
Total vectors = 10,000,000

ANN candidate set = 10,000
```

The system may calculate perfectly valid distances for those 10,000.

Why can the answer still be approximate?

Because the true nearest vector may have been excluded during candidate selection.

So:

> ANN often approximates **which candidates deserve to be searched**, rather than deliberately computing the distance formula incorrectly.

---

# 9. Why ANN Can Miss the True Nearest Neighbor

Imagine:

```text
Region A             Region B

  A1                     B1
  A2           Q |       B2 <- true nearest
               |
```

Suppose the index decides that `Q` belongs to Region A and searches only Region A.

It may never evaluate:

```text
distance(Q, B2)
```

Even if `B2` is actually the closest vector.

The error did not happen because the distance formula failed.

The error happened because the candidate was never examined.

This explains the word:

```text
APPROXIMATE
```

---

# 10. Recall@K

We need a way to measure ANN quality.

Suppose exact search tells us that the true nearest 10 vectors are:

```text
A B C D E F G H I J
```

ANN returns:

```text
A B C D E F G H X Y
```

Eight of the ten true neighbors were recovered.

Therefore:

```text
Recall@10 = 8 / 10
          = 0.8
          = 80%
```

General definition:

```text
Recall@K =
(number of true top-K neighbors recovered)
/
K
```

If ANN returns all true top-10 neighbors:

```text
Recall@10 = 1.0 = 100%
```

If it returns nine:

```text
Recall@10 = 0.9 = 90%
```

ANN should therefore not be thought of as simply:

```text
accurate vs inaccurate
```

It is better understood as a tunable spectrum.

---

# 11. The Central ANN Trade-off

```text
Explore fewer candidates
        |
        +--> less computation
        +--> lower latency
        +--> potentially lower recall

Explore more candidates
        |
        +--> more computation
        +--> higher latency
        +--> potentially higher recall
```

Therefore:

# Speed <-------> Recall

This idea appears repeatedly in vector-search systems.

You normally tune how deeply the search explores the index.

---

# 12. Two Major ANN Strategies in the Lecture

The lecture focuses on two different ways of reducing candidate comparisons.

```text
ANN
|
+-- IVF
|    |
|    +-- partition vector space into regions
|    +-- search promising regions
|
+-- HNSW
     |
     +-- create a graph among vectors
     +-- navigate the graph toward the query
```

Both solve the same high-level problem:

> Avoid scanning every vector.

But their data structures are completely different.

---

# PART I — IVF

# 13. IVF — Inverted File Index

IVF stands for:

> **Inverted File Index**

The intuition is:

```text
Huge vector space
       |
       v
divide into clusters / regions
       |
       v
represent each region by a centroid
       |
       v
query arrives
       |
       v
find promising centroid(s)
       |
       v
search only corresponding lists
```

Imagine one million vectors.

Instead of one list:

```text
V1
V2
V3
...
V1,000,000
```

we create something conceptually like:

```text
Cluster 0 -> [V3, V12, V91, ...]
Cluster 1 -> [V8, V17, V102, ...]
Cluster 2 -> [V2, V44, V81, ...]
...
Cluster 999
```

If the clusters were perfectly balanced:

```text
1,000,000 vectors
/
1,000 clusters
≈
1,000 vectors per cluster
```

Now the search problem becomes much smaller.

---

# 14. Centroids

Each cluster needs a representative location.

That representative is the **centroid**.

If a cluster contains vectors:

```text
V1
V2
V3
...
Vn
```

its centroid is conceptually the central point of that group.

Think of a centroid as:

> The rough address of a vector neighborhood.

So instead of initially comparing a query with one million vectors, we can compare it with perhaps one thousand centroids.

```text
1,000,000 vectors
       |
       v
1,000 clusters
       |
       v
1,000 centroids
```

---

# 15. How Are IVF Clusters Built?

The lecture uses **K-means clustering** as the mental model.

Index creation happens **before query time**.

Conceptually:

```text
INDEX BUILD

all vectors
    |
    v
run clustering
    |
    v
learn centroids
    |
    v
assign each vector to its nearest centroid
    |
    v
build inverted lists
```

This distinction matters:

> We do not run expensive clustering every time a user searches.

Index construction is an offline/preprocessing or write-time cost that is then reused for many queries.

This is a general database idea:

```text
spend work while building the index
so future reads can become faster
```

---

# 16. Why "Inverted File"?

The useful mental model is:

```text
Centroid / Region -> list of vector IDs
```

For example:

```text
C0 -> [V3, V9, V81, V400, ...]
C1 -> [V2, V7, V33, V101, ...]
C2 -> [V1, V4, V18, V900, ...]
```

The query does not search the entire database.

Instead:

```text
Q
|
v
compare Q with centroids
|
v
choose promising centroid(s)
|
v
open the corresponding list(s)
|
v
compare Q with vectors inside those lists
|
v
return Top K
```

That is the core IVF algorithm.

---

# 17. A Small 2D IVF Example

Suppose:

```text
Interstellar   [8, 8]
Gravity        [8, 7]
The Martian    [7, 8]

Titanic        [2, 2]
The Notebook   [2, 3]
La La Land     [3, 2]

Avengers       [8, 2]
Iron Man       [7, 2]
Thor           [8, 3]
```

Clustering may produce:

```text
List 1 — Romance
Titanic
The Notebook
La La Land

List 2 — Space
Interstellar
Gravity
The Martian

List 3 — Superhero
Avengers
Iron Man
Thor
```

Suppose the query embedding is:

```text
Q = [7.7, 7.5]
```

First compare `Q` with:

```text
C1
C2
C3
```

If `C2` is the nearest centroid, the system searches:

```text
Interstellar
Gravity
The Martian
```

instead of all movies.

Then it calculates actual similarities inside that candidate list.

This is where the computational saving comes from.

---

# 18. IVF Complexity Intuition

Suppose:

```text
N = 1,000,000 vectors
clusters = 1,000
vectors/list ≈ 1,000
```

Exact scan:

```text
~1,000,000 candidates
```

If IVF searches five lists:

```text
5 * ~1,000
≈ 5,000 vector candidates
```

plus the much smaller work of choosing promising centroids.

That is drastically less than one million vector comparisons.

---

# 19. The IVF Boundary Problem

Clustering introduces artificial boundaries.

Vector space itself does not contain walls.

Imagine:

```text
        Cluster A | Cluster B
                  |
        A1        | B1 <- true nearest
            Q     |
                  |
```

Suppose `Q` lies slightly inside Cluster A.

If we search only Cluster A:

```text
search(A)
```

we may miss `B1`, even though `B1` is physically closer to the query.

Therefore:

> Searching only the single nearest cluster can reduce recall.

---

# 20. Search Multiple Clusters

The fix is simple in principle:

Instead of:

```text
search 1 cluster
```

search:

```text
5 clusters
```

or:

```text
20 clusters
```

The lecture refers to this using **probes**.

Conceptually:

```text
Few probes
   |
   +--> fewer candidates
   +--> faster
   +--> potentially lower recall

More probes
   |
   +--> more candidates
   +--> slower
   +--> potentially higher recall
```

Again:

# Speed <-------> Recall

The same fundamental trade-off appears again.

---

# 21. IVF in One Sentence

> IVF first asks **"Which neighborhoods are worth entering?"** and then searches vectors only inside those neighborhoods.

---

# PART II — HNSW

# 22. HNSW — Hierarchical Navigable Small World

HNSW stands for:

> **Hierarchical Navigable Small World**

IVF says:

```text
partition the space
```

HNSW says:

```text
connect nearby vectors and navigate
```

That is a fundamentally different approach.

---

# 23. Start with a Graph

Imagine vectors as nodes:

```text
A
B
C
D
E
F
```

Connect useful nearby nodes:

```text
A ---- B
|    / |
|   /  |
C ---- D ---- E
       |
       F
```

Each vector is a **node**.

Connections are **edges**.

A node may store conceptually:

```text
Node {
    vector
    metadata
    neighbor references
}
```

Now search can become movement through the graph.

---

# 24. Graph Navigation

Suppose the query `Q` is near `F`.

Start at `A`.

```text
distance(Q, A)
```

Inspect A's neighbors:

```text
B
C
```

If B looks closer:

```text
A -> B
```

Inspect B's neighbors.

Maybe D is better:

```text
A -> B -> D
```

Then perhaps F:

```text
A -> B -> D -> F
```

Instead of scanning every vector, the system has navigated through promising nodes.

The core mental model is:

```text
start somewhere
    |
    v
inspect neighbors
    |
    v
move toward promising nodes
    |
    v
repeat
```

---

# 25. Why a Flat Graph Is Not Enough

Suppose a huge graph has only local edges:

```text
A-B-C-D-E-F-G-H-I-J-K-L-M-N-O-P-...
```

If you start at `A` and the desired region is around `Z`, you could require many hops.

This is exactly like trying to travel from one city to another using only tiny neighborhood roads.

So HNSW adds:

> hierarchy

---

# 26. The Road/Highway Analogy

When travelling long distances, we use:

```text
local road
    |
main road
    |
highway
    |
main road
    |
local road
```

Why?

```text
Highway -> large jumps
Local road -> precise navigation
```

HNSW uses the same intuition.

---

# 27. Hierarchical Graph Layers

Conceptually:

```text
Layer 3
A ---------------- M ---------------- Z

Layer 2
A ------- F ------- M ------- S ------- Z

Layer 1
A -- C -- F -- H -- K -- M -- P -- S -- V -- Z

Layer 0
A-B-C-D-E-F-G-H-I-J-K-L-M-N-O-P-Q-R-S-T-U-V-W-X-Y-Z
```

Important properties:

```text
Higher layer
-> fewer nodes
-> longer-range edges
-> coarse navigation

Lower layer
-> more nodes
-> shorter/local edges
-> precise navigation

Layer 0
-> contains every vector
```

Not every vector appears in every layer.

Higher-layer membership becomes progressively sparser.

That is what creates the hierarchy.

---

# 28. Querying HNSW

Suppose the query lies near `X`.

At a high layer:

```text
A -------- M -------- Z
```

Starting at A, M may be much closer.

```text
A -> M
```

Drop one layer.

Maybe:

```text
M -> S
```

Drop again.

Maybe:

```text
S -> V
```

At the bottom layer:

```text
V -> W -> X
```

Therefore:

```text
Start high
   |
   v
make large jumps
   |
   v
reach approximately correct region
   |
   v
move down one layer
   |
   v
make smaller jumps
   |
   v
move down again
   |
   v
fine-grained search
```

This is the core HNSW intuition.

---

# 29. Why "Small World"?

A small-world graph is designed so that, despite a large number of nodes, useful long-range links allow relatively short navigation paths between distant regions.

In HNSW, hierarchy and carefully selected edges make that navigation practical.

You should remember the idea rather than memorizing the name:

> Some edges help with local precision; other higher-level links let us jump across large regions quickly.

---

# 30. HNSW Does Not "Understand Semantics"

Suppose:

```text
Interstellar
The Martian
Gravity
```

are near one another.

Why?

Because the embedding model produced vectors whose geometry reflects semantic similarity.

HNSW sees only something like:

```text
V1
V2
V3

distance(V1, V2)
distance(V1, V3)
```

Therefore:

```text
Embedding model
    |
    +--> creates semantic geometry

HNSW
    |
    +--> navigates that geometry
```

Do not mentally assign language understanding to the index itself.

---

# 31. Why Not Connect Every Vector to Every Other Vector?

Suppose we have:

```text
1,000,000 vectors
```

If every node connected to every other node, we would create an enormous number of edges.

That would destroy scalability.

So HNSW needs a **sparse but navigable graph**.

We need:

```text
enough neighbors
    |
    +--> good navigation

but not too many
    |
    +--> acceptable memory
    +--> acceptable construction cost
    +--> acceptable search cost
```

---

# 32. The HNSW `M` Parameter

Many implementations expose a parameter commonly called:

```text
M
```

A useful mental model from the notes is:

> `M` roughly controls how many graph connections a node is allowed to maintain.

Very small M:

```text
less memory
fewer edges
possibly harder navigation
```

Very large M:

```text
more edges
more memory
more construction work
more neighbors to inspect
potentially stronger connectivity
```

So HNSW also has a memory/quality/performance trade-off.

---

# 33. HNSW Is Not Simply "Follow the Best Edge"

A naïve graph search could do:

```text
current node
-> choose immediately closest neighbor
-> repeat
```

But purely greedy search can get stuck in a poor local region.

Real HNSW search can keep multiple promising candidates during exploration.

Conceptually:

```text
not:
keep only one possible path

but:
maintain a frontier of promising alternatives
```

This reduces the chance that one locally attractive decision traps the search.

The lecture's essential lesson is:

> HNSW is graph exploration, not blindly following a single edge forever.

---

# 34. HNSW Index Construction Has a Cost

When a new vector `X` is inserted, the system has to decide:

```text
Where should X live?
Which vectors should X connect to?
Which edges produce useful navigation?
Which layers should contain X?
```

Conceptually:

```text
new vector X
    |
    v
find promising neighbors
    |
    v
choose graph connections
    |
    v
insert X
```

Better graph construction can improve search quality, but index building becomes more expensive.

Therefore HNSW has trade-offs at:

```text
BUILD TIME
and
QUERY TIME
```

---

# 35. Why HNSW Can Consume Significant Memory

HNSW stores more than just vectors.

Conceptually:

```text
raw vectors
+
neighbor references / graph edges
+
hierarchical graph structure
```

With millions of vectors and multiple neighbor references per node, edge storage can become significant.

This is why HNSW may achieve very strong query performance partly by spending more memory.

---

# 36. IVF vs HNSW

## IVF

```text
Vector space
   |
   v
clusters
   |
   v
centroids
   |
   v
promising regions
   |
   v
candidate vectors
```

Think:

> Divide the world, then search promising neighborhoods.

## HNSW

```text
Vectors
   |
   v
graph nodes + edges
   |
   v
hierarchical layers
   |
   v
navigate from broad to local
```

Think:

> Navigate the world instead of scanning it.

## What they have in common

Both avoid:

```text
Q compared with every vector
```

They just reduce the search space in different ways.

---

# PART III — STORAGE COST

# 37. Search Is Only Half the Problem

ANN solves:

> How can I avoid comparing the query with every vector?

But vectors are also expensive to **store**.

Suppose:

```text
100,000,000 vectors
1,536 dimensions/vector
float32
```

A float32 uses:

```text
32 bits = 4 bytes
```

One vector therefore needs:

```text
1,536 * 4 bytes
= 6,144 bytes
≈ 6 KB
```

For 100 million vectors:

```text
6,144 * 100,000,000
= 614,400,000,000 bytes
≈ 614 GB
```

And this is only raw vector data.

It does not include:

- IDs
- metadata
- ANN index structures
- HNSW edges
- database page overhead
- replicas
- backups
- other application data

Therefore vector databases face two independent costs:

```text
SEARCH COST
and
STORAGE COST
```

---

# 38. Do We Need Full Numerical Precision?

An embedding may contain values like:

```text
0.183742
-0.729182
0.029117
0.441827
...
```

Do we always need every coordinate stored at float32 precision for retrieval?

Possibly not.

This motivates:

> **Quantization**

The general idea:

```text
high-precision representation
        |
        v
lower-precision representation
        |
        v
less storage
```

---

# 39. Simple Precision Reduction

For a 1,536-dimensional embedding:

## float32

```text
1,536 * 4
= 6,144 bytes
```

## float16

```text
1,536 * 2
= 3,072 bytes
```

Roughly:

```text
2x smaller
```

If a representation uses approximately one byte per dimension:

```text
1,536 bytes
```

roughly:

```text
4x smaller than float32
```

This is the general compression intuition.

But Product Quantization goes further.

---

# PART IV — PRODUCT QUANTIZATION

# 40. Product Quantization — PQ

Product Quantization solves a storage problem using a clever idea:

> Do not store every coordinate exactly. Split the vector into small chunks and replace each chunk by the ID of a representative pattern.

Take a tiny vector:

```text
V = [
  0.2,
  0.8,
 -0.4,
  0.1,
  0.7,
 -0.2,
  0.9,
  0.3
]
```

This is:

```text
8 dimensions
```

At float32:

```text
8 * 4 bytes = 32 bytes
```

---

# 41. Step 1 — Split the Vector into Sub-vectors

Split:

```text
[0.2, 0.8 | -0.4, 0.1 | 0.7, -0.2 | 0.9, 0.3]
```

into:

```text
S1 = [0.2, 0.8]
S2 = [-0.4, 0.1]
S3 = [0.7, -0.2]
S4 = [0.9, 0.3]
```

These are called:

> **sub-vectors**

So:

```text
8 dimensions
   |
   v
4 sub-vectors
   |
   v
2 dimensions each
```

---

# 42. Step 2 — Learn Common Patterns in Each Subspace

Suppose we have one million vectors.

Look only at the first chunk of every vector:

```text
Vector A -> [0.20, 0.80]
Vector B -> [0.21, 0.82]
Vector C -> [0.18, 0.84]
Vector D -> [0.21, 0.77]
...
```

Many sub-vectors may be close to one another.

Instead of storing each exact pair independently, cluster similar sub-vectors.

Example:

```text
[0.20, 0.80]
[0.22, 0.79]
[0.18, 0.84]
[0.21, 0.77]
```

These might be represented by a centroid:

```text
C0 = [0.20, 0.81]
```

Another group:

```text
[-0.42, 0.12]
[-0.39, 0.08]
[-0.45, 0.14]
```

might use:

```text
C1 = [-0.42, 0.11]
```

---

# 43. Codeword and Codebook

A representative centroid for a sub-vector cluster may be called a:

> **codeword**

The collection of representatives is a:

> **codebook**

Suppose a codebook contains:

```text
C0
C1
C2
...
C255
```

Now instead of storing:

```text
[0.22, 0.79]
```

we can store:

```text
0
```

meaning:

```text
"Use codeword C0."
```

The floating-point coordinates are replaced with a tiny integer identifier.

---

# 44. Compressing a Full Vector

Suppose the four chunks of a vector map to:

```text
S1 -> centroid #17
S2 -> centroid #203
S3 -> centroid #81
S4 -> centroid #6
```

Instead of storing:

```text
[0.2, 0.8, -0.4, 0.1, 0.7, -0.2, 0.9, 0.3]
```

we store:

```text
[17, 203, 81, 6]
```

That sequence of small codes becomes the compressed representation.

---

# 45. Why 256 Centroids Are Convenient

Suppose each subspace has:

```text
256 centroids
```

Because:

```text
256 = 2^8
```

one centroid ID needs:

```text
8 bits
= 1 byte
```

Therefore:

```text
one sub-vector
-> represented by one centroid ID
-> one byte
```

This is where extremely strong compression becomes possible.

---

# 46. PQ with a 1,536-Dimensional Vector

Original:

```text
D = 1,536
float32 = 4 bytes/dimension

1,536 * 4
= 6,144 bytes
```

Now divide into:

```text
96 sub-vectors
```

Each chunk contains:

```text
1,536 / 96
= 16 dimensions
```

Suppose every 16-dimensional chunk is replaced by one of 256 centroids.

Then:

```text
1 chunk -> 1 byte
96 chunks -> 96 bytes
```

Original:

```text
6,144 bytes
```

PQ codes:

```text
96 bytes
```

Compression ratio:

```text
6,144 / 96
= 64x
```

This is a huge reduction.

---

# 47. PQ Is Lossy

Suppose the original sub-vector is:

```text
[0.217, 0.794]
```

The nearest codeword might be:

```text
[0.200, 0.810]
```

They are similar, but not identical.

So Product Quantization is:

> **lossy compression**

It intentionally sacrifices some numerical precision in exchange for large storage savings.

The trade-off is:

```text
smaller memory footprint
        <->
numerical precision
```

---

# 48. Do Not Misunderstand PQ Dimensionality

A useful clarification:

PQ does not literally claim that the semantic embedding was originally only 96 dimensions.

The original vector still came from a 1,536-dimensional space.

What changed is its **stored representation**.

Instead of storing all 1,536 float values directly, the representation stores approximately:

```text
96 codeword IDs
```

where each ID represents a 16-dimensional sub-vector pattern.

So think:

```text
original vector geometry: 1,536 dimensions

compressed code:
96 small identifiers
```

That is more precise than saying "the vector has become 96-dimensional" in the ordinary embedding sense.

---

# PART V — WHAT MAKES A VECTOR DATABASE?

# 49. A Vector Database Is Not Just an Array Store

A production vector-search system contains multiple layers.

A useful full mental model is:

```text
Original application data
        |
        v
Embedding model
        |
        v
Vector representation
        |
        v
Vector storage
        |
        v
Distance / similarity metric
        |
        v
ANN index
        |
        v
candidate retrieval
        |
        v
metadata filtering
        |
        v
Top K results
```

At scale, compression may also appear:

```text
Original vector
      |
      v
Quantization / PQ
      |
      v
smaller representation
```

---

# 50. Layer 1 — Original Data

For a movie:

```json
{
  "id": 101,
  "title": "Interstellar",
  "description": "A team travels through a wormhole...",
  "genre": "Sci-Fi",
  "year": 2014
}
```

This is normal application information.

---

# 51. Layer 2 — Embedding

The record also has an embedding:

```text
embedding = [0.21, -0.45, 0.81, ...]
```

A practical vector record becomes conceptually:

```text
ID
+
VECTOR
+
METADATA
```

For example:

```json
{
  "id": "m1",
  "embedding": [0.21, -0.45, 0.81],
  "metadata": {
    "movieName": "Interstellar",
    "description": "...",
    "releaseDate": "2014-11-07"
  }
}
```

---

# 52. Layer 3 — Similarity / Distance Metric

A database must know what "near" means.

Common choices in the lecture are:

- cosine similarity
- dot product
- Euclidean / L2 distance

## Cosine similarity intuition

Cosine similarity focuses on vector direction.

Conceptually:

```text
cos(theta) =
(Q . V)
/
(||Q|| * ||V||)
```

If vectors point in very similar directions, cosine similarity is high.

## Dot product

```text
Q . V =
sum(q_i * v_i)
```

It incorporates alignment and magnitude.

## Euclidean distance

```text
distance(Q, V) =
sqrt(sum((q_i - v_i)^2))
```

Smaller means closer.

The index and query semantics must use an appropriate metric.

---

# 53. Layer 4 — ANN Index

Without an ANN index:

```text
Query
  |
  v
scan every vector
```

With an ANN index:

```text
Query
  |
  v
IVF / HNSW
  |
  v
promising candidates
  |
  v
Top K
```

The index exists primarily to reduce the number of vectors that must be considered during a search.

---

# 54. Layer 5 — Metadata Filtering

Real applications rarely ask only:

```text
"find the nearest vectors"
```

A query may instead mean:

```text
Find movies semantically similar to Q
where:
language = Hindi
genre = Comedy
year >= 2020
```

So vector databases commonly combine:

```text
vector similarity
+
structured filters
```

Example:

```text
Top 10 nearest to Q

WHERE
language = "Hindi"
AND genre = "Comedy"
AND year >= 2020
```

Efficiently combining metadata filtering with ANN is itself an important systems problem.

---

# 55. Dedicated Vector Databases

The lecture mentions systems such as:

- Pinecone
- Qdrant
- Weaviate

Their primary purpose includes efficient vector storage, indexing and similarity retrieval.

But a dedicated vector database is not always required.

---

# 56. PostgreSQL + pgvector

PostgreSQL can gain vector capabilities through **pgvector**.

Conceptually:

```sql
CREATE TABLE movies (
    id BIGSERIAL PRIMARY KEY,
    title TEXT,
    embedding VECTOR(1536)
);
```

Nearest-neighbor querying can look conceptually like:

```sql
SELECT *
FROM movies
ORDER BY embedding <=> :query_vector
LIMIT 10;
```

Vector indexes such as:

```text
HNSW
IVFFlat
```

can be used to accelerate retrieval.

This is important architecturally:

> If your application already needs PostgreSQL for relational data, you may be able to add vector search without immediately introducing a separate vector database.

---

# 57. Exact KNN vs ANN — Final Comparison

## Exact KNN

```text
Q
|
v
inspect every vector
|
v
calculate every distance
|
v
return true Top K
```

Advantages:

- mathematically exact
- maximum possible recall
- simple mental model

Disadvantages:

- expensive at large `N`
- query cost grows with dataset size
- heavy memory-bandwidth and compute requirements

---

## ANN

```text
Q
|
v
use an index
|
v
select promising candidates
|
v
evaluate candidates
|
v
return approximate Top K
```

Advantages:

- far fewer comparisons
- much faster at scale
- practical for large retrieval systems

Trade-off:

```text
Speed <-------> Recall
```

---

# 58. IVF vs HNSW vs PQ — Do Not Mix Their Jobs

This distinction is essential.

## IVF

Solves mainly:

```text
SEARCH COST
```

Method:

```text
cluster space
-> find promising clusters
-> search only those lists
```

## HNSW

Solves mainly:

```text
SEARCH COST
```

Method:

```text
graph navigation
-> hierarchical jumps
-> local refinement
```

## PQ

Solves mainly:

```text
STORAGE / MEMORY COST
```

Method:

```text
split vector
-> quantize each sub-vector
-> store codeword IDs
```

PQ is not simply another graph/search algorithm.

It addresses a different dimension of the system.

---

# 59. How These Techniques Can Work Together

A real system does not have to choose exactly one idea globally.

Conceptually, a system may combine:

```text
ANN indexing
+
compressed vectors
+
metadata filters
```

For example:

```text
Query
  |
  v
ANN index narrows candidate space
  |
  v
compressed representation helps memory efficiency
  |
  v
candidate scoring
  |
  v
metadata constraints
  |
  v
Top K
```

The lecture's deeper lesson is therefore not merely "learn IVF" or "learn HNSW."

It is:

> Vector databases are engineered around multiple trade-offs simultaneously.

---

# 60. The Four Big Trade-offs You Should Remember

## 1. Search speed vs recall

Search more candidates:

```text
slower
but
higher chance of recovering true nearest neighbors
```

Search fewer:

```text
faster
but
potentially lower recall
```

---

## 2. Memory vs graph connectivity

In HNSW:

```text
more edges
-> more memory
-> potentially easier graph navigation
```

Fewer edges:

```text
less memory
-> potentially weaker navigation
```

---

## 3. Build cost vs query quality

Better indexes often require more work during construction.

```text
more expensive index build
        |
        v
potentially better future searches
```

---

## 4. Precision vs storage

With quantization / PQ:

```text
less storage
        |
        v
less numerical precision
```

The whole field is largely about choosing good points on these trade-off curves.

---

# 61. The Lecture in One Giant Diagram

```text
                       USER QUERY
                           |
                           v
                    Embedding Model
                           |
                           v
                       Query Vector
                           |
                           v
              +---------------------------+
              |   Vector Search System    |
              +---------------------------+
                           |
             choose similarity/distance
                           |
                           v
              +---------------------------+
              | ANN Candidate Selection   |
              |                           |
              | IVF       OR       HNSW   |
              |                           |
              | Regions          Graph    |
              +---------------------------+
                           |
                           v
                 Promising Candidates
                           |
                           v
                    Similarity Scores
                           |
                           v
                    Metadata Filters
                           |
                           v
                       Top K Results


Storage side:

Original 1536-D float32 embedding
             |
             v
      optional quantization
             |
             v
             PQ
             |
             v
    compact codeword IDs
```

---

# 62. The Most Important Interview-Level Explanations

## Q: Why do we need a vector index?

Because brute-force nearest-neighbor search requires comparing the query against every stored vector. For `N` vectors of dimensionality `D`, the naïve distance-computation cost grows roughly with `O(ND)`. ANN indexes reduce latency by selecting a much smaller candidate set.

---

## Q: What does ANN approximate?

Usually the search space/candidate selection. The true neighbor may be missed because it was never evaluated, even though the distance computation for evaluated candidates is valid.

---

## Q: What is Recall@K?

The fraction of the true exact top-K neighbors that the ANN result successfully recovers.

```text
Recall@K =
true top-K neighbors recovered / K
```

---

## Q: How does IVF work?

IVF partitions vector space into clusters represented by centroids. At query time, the query is compared with centroids, a few promising inverted lists are selected, and only vectors inside those lists are searched.

---

## Q: What is the weakness of searching only one IVF cluster?

A true nearest neighbor can lie just across an artificial cluster boundary. Searching multiple nearby clusters improves recall but costs more work.

---

## Q: How does HNSW work?

HNSW builds a sparse multi-layer graph of vector relationships. Search begins at a sparse upper layer for large jumps, progressively descends through denser layers, and finally performs fine-grained exploration near the query.

---

## Q: Why is HNSW memory hungry?

Because it stores not only vectors but also neighbor references/graph edges across its hierarchical structure.

---

## Q: What problem does PQ solve?

Primarily vector storage/memory cost. It splits a vector into sub-vectors, quantizes each sub-vector using a codebook, and stores small centroid IDs instead of all original floating-point coordinates.

---

## Q: Is PQ lossless?

No. It is lossy. Each original sub-vector is replaced by an approximate representative centroid.

---

# 63. One Precise Mental Model to Carry Forward

Whenever you hear **vector database**, think of these questions:

```text
1. What is being embedded?
2. Which embedding model created the vectors?
3. What is the dimensionality?
4. Which similarity/distance metric is used?
5. Is retrieval exact or approximate?
6. If approximate, which ANN index?
7. How much of the index is explored?
8. What recall/latency trade-off is acceptable?
9. How are vectors stored?
10. Are they quantized/compressed?
11. What metadata travels with each vector?
12. How are metadata filters combined with ANN?
```

If you can answer those questions, you understand the architecture far more deeply than someone who merely says:

> "Vector DB stores embeddings."

---

# 64. Final Summary

The lecture can be compressed into four core ideas.

## A. Embeddings create geometry

```text
Similar meaning
-> similar vector representation
-> nearby points in vector space
```

## B. Exact search becomes expensive

```text
N vectors
x
D dimensions
```

creates enormous compute cost at scale.

## C. ANN reduces the candidate set

Two major approaches discussed:

```text
IVF
-> cluster-based candidate selection

HNSW
-> graph-based navigation
```

This introduces the fundamental:

```text
speed <-> recall
```

trade-off.

## D. Quantization reduces memory

```text
float32 vectors
-> lower precision / PQ
-> smaller representations
```

Product Quantization can achieve dramatic compression by replacing sub-vectors with codeword IDs, at the cost of lossy approximation.

Finally, a practical vector database combines:

```text
ID
+
vector
+
metadata
+
distance function
+
ANN index
+
filtering
+
optional compression
```

The embedding model creates the meaningful space.

The vector database's engineering job is to:

> **store that space, index it, search it, filter it, and do all of that efficiently at scale.**

                     USER QUERY
                         │
                         ▼
                  Embedding Model
                         │
                         ▼
                    Query Vector
                         │
                         ▼
              Similarity / Distance
                         │
                         ▼
               ANN Candidate Search
                  │              │
                  ▼              ▼
                 IVF           HNSW
              Regions           Graph
                  │              │
                  └──────┬───────┘
                         ▼
                Candidate Vectors
                         │
                         ▼
                 Metadata Filters
                         │
                         ▼
                    Top-K Results


Storage side:

1536-D float32 vector
        │
        ▼
   Quantization
        │
        ▼
 Product Quantization
        │
        ▼
 compact codeword IDs