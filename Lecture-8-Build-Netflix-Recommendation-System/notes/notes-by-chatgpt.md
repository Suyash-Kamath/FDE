Yes. I went through the transcript, including the implementation half. The lecture is much more important than “how to make a movie recommendation app.” It is actually teaching you the foundation of **semantic search, embeddings, vector retrieval, nearest-neighbor search, and eventually RAG/vector databases**.

I’ll explain it from the perspective of someone building the backend in **Python + FastAPI**, and I’ll separate what the lecturer demonstrates from what I would change in a real backend.

---

# 1. First understand what the project actually is

Imagine your backend owns this catalog:

```text
Movie 1:
Title: Interstellar
Description:
"A group of astronauts travels through a wormhole
in search of a new home for humanity."

Movie 2:
Title: Titanic
Description:
"A tragic romance aboard the Titanic..."

Movie 3:
Title: Gravity
Description:
"Two astronauts struggle to survive after an accident in space."

Movie 4:
Title: The Hangover
Description:
"A group of friends gets into chaotic situations in Las Vegas."
```

A user searches:

```text
"something emotional and romantic"
```

or:

```text
"space survival movie"
```

or:

```text
"funny movie with friends"
```

The problem is:

> How does your backend understand what the user **means**, rather than merely matching exact words?

That distinction is the soul of this lecture.

Traditional string search might look for words:

```python
if "space" in movie.description:
    ...
```

But suppose the movie description says:

```text
"astronauts travel through a wormhole"
```

There may be no literal word `"space"`.

Yet semantically:

```text
"space movie"
```

and

```text
"astronauts travel through a wormhole"
```

are very closely related.

That is where **embeddings** enter.

The transcript's demo supports two operations: natural-language searching and clicking a movie to retrieve similar movies. 

---

# 2. The core mental model

The entire system can be understood as:

```text
TEXT
 ↓
Embedding Model
 ↓
VECTOR
 ↓
Compare against other vectors
 ↓
Similarity scores
 ↓
Top K closest vectors
 ↓
Movies
```

For example:

```text
"space survival movie"
        ↓
Embedding model
        ↓
[0.017, -0.082, 0.125, ..., 0.034]
        ↓
compare with every movie embedding
        ↓
Interstellar  → 0.91
Gravity       → 0.88
The Martian   → 0.86
Titanic       → 0.32
The Hangover  → 0.09
        ↓
Top 3
        ↓
Interstellar
Gravity
The Martian
```

Notice something very important:

**No LLM generated those movie names.**

The backend searched its **existing catalog**.

That's the architectural lesson.

---

# 3. Why not simply ask an LLM?

Suppose the user writes:

```text
Recommend me 3 space movies.
```

You could send this to an LLM:

```text
You are a movie recommendation expert.

User wants:
space movies

Recommend three movies.
```

The LLM might return:

```text
Interstellar
The Martian
Gravity
```

Technically, that works.

But there is a serious problem.

Suppose your streaming platform only contains:

```text
Interstellar
Titanic
Toy Story
The Dark Knight
La La Land
```

The LLM doesn't automatically know your catalog.

It might recommend:

```text
The Martian
Gravity
Arrival
```

even though none of those are available.

The lecturer emphasizes exactly this difference: the goal isn't generic movie knowledge; recommendations have to come **from the platform's own catalog**. 

You could solve this by sending the entire catalog to the LLM:

```text
Here are my 100,000 movies:

Movie 1...
Movie 2...
Movie 3...
...
Movie 100000...

User wants:
space sci-fi movie

Choose three from the above.
```

But now you've created another problem.

Huge context.

Every request becomes something like:

```text
System Prompt
+
100,000 movie descriptions
+
User query
```

That means more tokens, larger latency, more cost, and eventually context-window limitations. The lecture explicitly calls this out as the context problem. 

Even more importantly:

**you don't need generation.**

The application doesn't need:

```text
"Interstellar is an epic Christopher Nolan masterpiece..."
```

It only needs:

```json
[
    {"title": "Interstellar"},
    {"title": "Gravity"},
    {"title": "The Martian"}
]
```

The transcript describes using an LLM here as unnecessary generation/overengineering. 

This is a very important AI-backend principle:

> Don't use generation when retrieval solves the problem.

---

# 4. So what is an embedding?

An embedding is essentially:

```text
meaning → numbers
```

Suppose, purely for visualization, our imaginary embedding model understood only three dimensions:

```text
[action, romance, comedy]
```

Batman might become:

```text
Batman = [9, 1, 2]
```

Titanic:

```text
Titanic = [2, 10, 1]
```

The Hangover:

```text
Hangover = [3, 1, 10]
```

Then similar meanings occupy similar regions of the vector space.

Real embeddings don't have interpretable dimensions like:

```text
dimension 1 = romance
dimension 2 = comedy
dimension 3 = darkness
```

Instead you get something like:

```python
[
    0.0157123,
    -0.028943,
    0.008291,
    0.041562,
    ...
]
```

Hundreds or thousands of dimensions.

The lecture's configuration produced a 1536-dimensional vector. 

So mathematically you can imagine:

```text
Interstellar = point in 1536-dimensional space
Gravity      = point in 1536-dimensional space
Titanic      = point in 1536-dimensional space
```

Obviously humans cannot visualize 1536 dimensions.

But computers don't need to visualize them.

They just calculate mathematical similarity.

---

# 5. Embedding model versus LLM

This is another major part of the transcript.

Both receive text.

For example:

```text
"Interstellar is an amazing movie"
```

A generative LLM conceptually does:

```text
Text
 ↓
tokens
 ↓
neural network / transformer representation
 ↓
predict next token
 ↓
next token
 ↓
next token
 ↓
...
```

Its purpose is:

```text
text → more text
```

For example:

```text
Input:
"Interstellar is an amazing"

Output:
"movie because..."
```

An embedding model instead does:

```text
Text
 ↓
tokens
 ↓
neural representation
 ↓
fixed-size vector
```

Its purpose is:

```text
text → vector
```

The transcript explicitly distinguishes generative next-token output from embedding-vector output. 

Think of it like this:

| Model           | Input | Output      | Purpose               |
| --------------- | ----- | ----------- | --------------------- |
| LLM             | Text  | Text/tokens | Generation/reasoning  |
| Embedding model | Text  | Vector      | Representation/search |

And this distinction will matter enormously when you study RAG.

---

# 6. Embeddings are reusable

This is possibly the most important backend idea in the lecture.

Movie descriptions usually don't change on every request.

For example:

```text
Interstellar description
```

today is probably the same description tomorrow.

So why do this every time?

```text
request
 ↓
embed Interstellar
embed Gravity
embed Titanic
embed Batman
embed Toy Story
...
```

That's wasteful.

Instead:

```text
                    OFFLINE / PRECOMPUTE

Interstellar description ──→ embedding model ──→ vector I
Gravity description      ──→ embedding model ──→ vector G
Titanic description      ──→ embedding model ──→ vector T
Batman description       ──→ embedding model ──→ vector B

                           ↓

                     store vectors
```

Then:

```text
                     ONLINE REQUEST

User query
"space survival"
      ↓
embedding model
      ↓
query vector Q
      ↓
Q vs I
Q vs G
Q vs T
Q vs B
      ↓
Top K
```

The transcript calls this **precomputation**: catalog embeddings are calculated ahead of time and reused. 

This idea appears everywhere in backend engineering:

```text
do expensive work once
        ↓
store result
        ↓
reuse cheap result many times
```

That's an optimization pattern far beyond AI.

---

# 7. Why embed the description instead of only the movie title?

Suppose you embed:

```text
Interstellar
```

There isn't much information in that string.

The embedding model must rely heavily on what it learned previously about the word/name "Interstellar."

Compare it with:

```text
Interstellar.
A science-fiction film about astronauts traveling
through a wormhole searching for another habitable
planet while humanity faces extinction on Earth.
```

Much richer semantic information.

Therefore the transcript uses movie **descriptions** for embeddings. 

A production system could go even further.

Instead of:

```python
text_to_embed = movie.description
```

I would often construct:

```python
text_to_embed = f"""
Title: {movie.title}
Genres: {", ".join(movie.genres)}
Description: {movie.description}
Director: {movie.director}
Keywords: {", ".join(movie.tags)}
"""
```

Now the vector captures much more useful information.

So:

```text
input quality
       ↓
embedding quality
       ↓
retrieval quality
```

This is extremely important.

Embeddings aren't magic.

Bad representation in → bad retrieval out.

---

# 8. Your first FastAPI flow: semantic search

Suppose the client sends:

```http
GET /movies/search?query=emotional space adventure
```

Your FastAPI backend performs:

```text
HTTP Request
      ↓
FastAPI router
      ↓
MovieService.search()
      ↓
Embedding API
      ↓
query embedding
      ↓
compare against stored movie embeddings
      ↓
similarity scores
      ↓
sort
      ↓
top 3
      ↓
JSON response
```

In Python conceptually:

```python
async def search_movies(query: str):
    query_embedding = await embed(query)

    matches = []

    for movie in movies:
        score = cosine_similarity(
            query_embedding,
            movie.embedding,
        )

        matches.append({
            "title": movie.title,
            "description": movie.description,
            "similarity": score,
        })

    matches.sort(
        key=lambda movie: movie["similarity"],
        reverse=True,
    )

    return matches[:3]
```

That tiny piece of code contains almost the whole lecture.

---

# 9. Understand the vector comparison very deeply

Suppose:

```python
query_embedding = [1, 2, 3]
```

and:

```python
movie_embedding = [4, 5, 6]
```

Obviously real embeddings contain far more dimensions, but the mathematics is identical.

The lecturer chooses **cosine similarity**. 

The formula is:

$$
\text{cosineSimilarity}(A,B)
=
\frac{A \cdot B}
{||A||\,||B||}
$$

Three pieces exist.

### Dot product

For:

```text
A = [1, 2, 3]
B = [4, 5, 6]
```

calculate:

$$
A \cdot B
=
(1 \times 4)
+
(2 \times 5)
+
(3 \times 6)
$$

The transcript walks through this exact computation. 

### Magnitude of A

$$
||A||
=
\sqrt{1^2 + 2^2 + 3^2}
$$

### Magnitude of B

$$
||B||
=
\sqrt{4^2 + 5^2 + 6^2}
$$

Then:

$$
\cos(\theta)
=
\frac{A \cdot B}
{||A|| ||B||}
$$

The transcript's implementation computes the dot product and both norms in a single loop before applying this formula. 

---

# 10. What cosine similarity actually means

Imagine two arrows:

```text
          B
         /
        /
       /
      /
-----/-------->
    A
```

Cosine similarity is primarily asking:

> Are these two vectors pointing in similar directions?

Not:

> Are their lengths identical?

That is why cosine similarity is useful for semantic vectors.

For example:

```text
A = [1, 2]
B = [10, 20]
```

B is much longer.

But both point in exactly the same direction.

Semantically you could interpret this as:

```text
same pattern / same meaning direction
```

Therefore cosine similarity treats them as highly similar.

This is what the transcript means when it says cosine emphasizes direction and largely ignores absolute magnitude. 

---

# 11. One correction to the transcript

At one point the lecture suggests that the individual embedding numbers are between `-1` and `+1` because cosine similarity is being used. 

Keep these two things separate.

An embedding is:

```python
vector = [x1, x2, x3, ..., xd]
```

The individual coordinates are floating-point values whose exact range depends on the model. You should **not conceptually assume every embedding coordinate must lie between -1 and +1 merely because you're using cosine similarity**.

What has the mathematical bound

```text
-1 ≤ score ≤ 1
```

is the **cosine similarity result**.

So remember:

```text
embedding components:
model-dependent floats

cosine similarity:
-1 to +1 mathematically
```

Very important distinction.

---

# 12. What do similarity values mean?

Conceptually:

```text
1
↑
very similar direction

0
↑
roughly orthogonal/unrelated direction

-1
↑
opposite direction
```

But don't build business logic around simplistic rules such as:

```python
if score > 0.8:
    definitely same meaning
```

The useful score distribution depends on the embedding model, dataset, text style and task.

Usually you're doing **ranking**.

Meaning:

```text
Movie A score = ...
Movie B score = ...
Movie C score = ...
Movie D score = ...

sort descending

return top K
```

The relative ordering is often more important than interpreting one score in isolation.

---

# 13. This is actually K-nearest-neighbor search

The lecturer doesn't need to dress it up in complicated terminology.

You have:

```text
query vector Q
```

and vectors:

```text
V1
V2
V3
...
VN
```

You calculate:

```text
similarity(Q, V1)
similarity(Q, V2)
similarity(Q, V3)
...
similarity(Q, VN)
```

Then select:

```text
K most similar vectors
```

That is a **K-nearest-neighbor-style vector search**.

If:

```python
k = 3
```

you're asking:

```text
Give me the 3 nearest vectors.
```

The transcript makes `k` configurable so top 3, top 5, etc. can be returned. 

This also connects directly to the vector-indexing concepts you've been learning.

---

# 14. What the lecture implemented is brute-force exact search

This point matters a lot.

Imagine:

```text
N = number of movies
D = embedding dimensions
```

For every query:

```python
for movie in all_movies:
    cosine_similarity(query_embedding, movie.embedding)
```

Each cosine comparison touches approximately `D` numbers.

And you're doing it for `N` movies.

Therefore the fundamental comparison cost is roughly:

$$
O(N \times D)
$$

If:

```text
N = 30
D = 1536
```

nothing to worry about.

If:

```text
N = 100,000
```

much larger.

If:

```text
N = 10,000,000
```

now brute force becomes an architectural problem.

This is why the transcript eventually says the in-memory linear search is just the demo and later a vector database/index is needed. 

And this connects directly to what you were asking me about earlier:

```text
Exact nearest-neighbor search
        ↓
compare with everything
        ↓
O(N × D)

Approximate nearest-neighbor index
        ↓
avoid comparing everything
        ↓
HNSW / IVF / etc.
```

So this lecture is the bridge between **embeddings** and **vector database indexing**.

---

# 15. There is another cost hidden in the lecture: sorting

Suppose you've computed similarities for:

```text
N movies
```

The lecture then sorts all matches:

```python
matches.sort(
    key=lambda x: x.similarity,
    reverse=True,
)
```

Sorting costs approximately:

$$
O(N\log N)
$$

Then:

```python
return matches[:k]
```

For the demo, perfect.

For huge datasets you wouldn't want to:

```text
calculate every score
+
sort every score
```

just to get 3 values.

You could use a top-K heap:

```python
heapq.nlargest(
    k,
    matches,
    key=lambda m: m.similarity,
)
```

That can make the selection stage closer to:

$$
O(N\log K)
$$

although your brute-force vector comparisons are still:

$$
O(ND)
$$

A vector index solves the larger search problem.

---

# 16. Now understand the second endpoint — similar movies

This flow is subtly different.

Suppose the user clicks:

```text
Interstellar
```

You already computed Interstellar's embedding:

```text
Interstellar → VI
```

Therefore don't do:

```text
Interstellar
 ↓
embedding API
 ↓
VI
```

again.

Simply retrieve `VI`.

Then:

```text
VI
 ↓
compare against every other movie
 ↓
scores
 ↓
top K
```

The transcript stresses this optimization: clicked-movie recommendations can reuse the already-stored vector, meaning that flow may require no model call whatsoever. 

That gives us two fundamentally different request paths.

---

# 17. Flow A versus Flow B

| Semantic query search                   | Similar-movie search                    |
| --------------------------------------- | --------------------------------------- |
| User sends new text                     | User selects existing item              |
| Query has no stored vector              | Movie already has vector                |
| Need embedding-model call               | No embedding call necessary             |
| Compare query vector with movie vectors | Compare movie vector with movie vectors |
| Return top K                            | Return top K excluding itself           |

That's a very useful architecture distinction.

---

# 18. Why skip the selected movie itself?

Suppose you calculate:

```text
similarity(
    interstellar_embedding,
    interstellar_embedding
)
```

Obviously this will be maximally similar.

So your result could become:

```text
1. Interstellar
2. Gravity
3. The Martian
```

But the user already selected Interstellar.

That's useless.

Therefore:

```python
if candidate.id == selected_movie.id:
    continue
```

The lecture explicitly handles this edge case. 

This seems tiny, but these details are exactly what backend implementation work is made of.

---

# 19. Now map the lecture to FastAPI

The Spring structure in the video is approximately:

```text
Controller
   ↓
Service
   ↓
Embedding model
   ↓
Stored movie data
```

Your FastAPI equivalent should conceptually be:

```text
Router
   ↓
MovieService
   ├── EmbeddingService
   └── Repository / Vector Store
```

So don't put everything inside:

```python
@app.get(...)
async def whatever():
    # 200 lines
```

You want responsibilities separated.

Conceptually:

```text
API layer
    HTTP concerns

Service layer
    recommendation logic

Embedding provider
    text → vector

Repository/vector store
    persistence + vector retrieval

Models/schemas
    structured data
```

The transcript itself makes a controller/service distinction before implementing the search logic. 

---

# 20. A clean FastAPI version

Here is a teaching implementation that matches the lecture closely.

```python
from contextlib import asynccontextmanager
from math import sqrt
from pathlib import Path
import json
import os

from fastapi import FastAPI, HTTPException, Query
from openai import AsyncOpenAI
from pydantic import BaseModel


EMBEDDING_MODEL = os.getenv(
    "OPENAI_EMBEDDING_MODEL",
    "text-embedding-3-small",
)


client = AsyncOpenAI()


class MovieMatch(BaseModel):
    title: str
    description: str
    similarity: float


class Movie:
    def __init__(
        self,
        title: str,
        description: str,
        embedding: list[float],
    ):
        self.title = title
        self.description = description
        self.embedding = embedding


def cosine_similarity(
    a: list[float],
    b: list[float],
) -> float:

    if len(a) != len(b):
        raise ValueError(
            "Vectors must have the same dimensions"
        )

    dot_product = 0.0
    norm_a = 0.0
    norm_b = 0.0

    for x, y in zip(a, b):
        dot_product += x * y
        norm_a += x * x
        norm_b += y * y

    if norm_a == 0 or norm_b == 0:
        return 0.0

    return dot_product / (
        sqrt(norm_a) * sqrt(norm_b)
    )


async def embed_many(
    texts: list[str],
) -> list[list[float]]:

    response = await client.embeddings.create(
        model=EMBEDDING_MODEL,
        input=texts,
        encoding_format="float",
    )

    ordered = sorted(
        response.data,
        key=lambda item: item.index,
    )

    return [
        item.embedding
        for item in ordered
    ]


class MovieService:

    def __init__(self):
        self.movies: list[Movie] = []

    async def initialize(self):

        path = Path("movies.json")

        raw_movies = json.loads(
            path.read_text()
        )

        descriptions = [
            movie["description"]
            for movie in raw_movies
        ]

        embeddings = await embed_many(
            descriptions
        )

        self.movies = [
            Movie(
                title=movie["title"],
                description=movie["description"],
                embedding=embedding,
            )
            for movie, embedding
            in zip(raw_movies, embeddings)
        ]

    def top_matches(
        self,
        query_embedding: list[float],
        k: int,
        exclude_title: str | None = None,
    ) -> list[MovieMatch]:

        matches = []

        for movie in self.movies:

            if (
                exclude_title is not None
                and movie.title == exclude_title
            ):
                continue

            similarity = cosine_similarity(
                query_embedding,
                movie.embedding,
            )

            matches.append(
                MovieMatch(
                    title=movie.title,
                    description=movie.description,
                    similarity=similarity,
                )
            )

        matches.sort(
            key=lambda match: match.similarity,
            reverse=True,
        )

        return matches[:k]

    async def search(
        self,
        query: str,
        k: int,
    ) -> list[MovieMatch]:

        [query_embedding] = await embed_many(
            [query]
        )

        return self.top_matches(
            query_embedding,
            k,
        )

    def similar_movies(
        self,
        title: str,
        k: int,
    ) -> list[MovieMatch]:

        selected_movie = next(
            (
                movie
                for movie in self.movies
                if movie.title.lower()
                == title.lower()
            ),
            None,
        )

        if selected_movie is None:
            raise HTTPException(
                status_code=404,
                detail="Movie not found",
            )

        return self.top_matches(
            selected_movie.embedding,
            k,
            exclude_title=selected_movie.title,
        )


movie_service = MovieService()


@asynccontextmanager
async def lifespan(app: FastAPI):

    await movie_service.initialize()

    yield

    movie_service.movies.clear()


app = FastAPI(
    lifespan=lifespan,
)


@app.get(
    "/movies/search",
    response_model=list[MovieMatch],
)
async def search_movies(
    query: str = Query(min_length=1),
    k: int = Query(default=3, ge=1, le=20),
):

    return await movie_service.search(
        query=query,
        k=k,
    )


@app.get(
    "/movies/{title}/similar",
    response_model=list[MovieMatch],
)
async def similar_movies(
    title: str,
    k: int = Query(default=3, ge=1, le=20),
):

    return movie_service.similar_movies(
        title=title,
        k=k,
    )
```

I've used the current asynchronous Python SDK shape here: the SDK supports `AsyncOpenAI`, and the embeddings resource supports `client.embeddings.create(...)`, including arrays of input strings, which is useful for batch precomputation. ([GitHub][1]) The transcript itself uses `text-embedding-3-small`, which remains an available embedding model. ([OpenAI Platform][2])

---

# 21. Notice why `lifespan` fits perfectly here

You've asked me about FastAPI `lifespan` before. This project is actually one of the easiest ways to understand why it exists.

You need something to happen:

```text
BEFORE
the first HTTP request
```

Namely:

```text
load movies
 ↓
generate/load embeddings
 ↓
put them somewhere accessible
 ↓
start serving requests
```

That maps perfectly to:

```python
@asynccontextmanager
async def lifespan(app: FastAPI):

    await movie_service.initialize()

    yield

    # shutdown cleanup
```

Everything before:

```python
yield
```

is startup.

Everything after:

```python
yield
```

is shutdown.

So:

```text
FastAPI starts

        ↓

initialize movies

        ↓

load/create embeddings

        ↓

yield
============================
SERVER IS NOW RUNNING
============================
        ↓

requests
requests
requests

        ↓

server shutting down

        ↓

code after yield
```

The Spring implementation similarly initializes the movie vectors at application startup so they're ready before search requests arrive. 

---

# 22. But I would change one major thing in production

The lecture computes all movie embeddings when the application starts.

Fine for:

```text
30 movies
```

Not something I'd recommend for:

```text
5,000,000 products
```

Imagine Kubernetes restarts your FastAPI container.

If startup means:

```text
read five million rows
+
call embedding API
+
recalculate five million embeddings
```

that's terrible architecture.

Embeddings should normally be created when content is created or changed.

For example:

```text
POST /movies

new movie
 ↓
save movie
 ↓
generate embedding
 ↓
save embedding
```

Or via background ingestion:

```text
catalog updated
      ↓
event/queue
      ↓
embedding worker
      ↓
embedding model
      ↓
vector DB
```

Then application startup becomes:

```text
connect database
connect vector database
start serving
```

not:

```text
re-embed the universe
```

The transcript does acknowledge that the JSON/in-memory approach is for learning and that production data/vectors belong in persistent storage. 

---

# 23. There is another improvement: batch embeddings

The lecturer conceptually does:

```python
for movie in movies:
    embedding = embed(movie.description)
```

Meaning potentially:

```text
Movie 1 → network call
Movie 2 → network call
Movie 3 → network call
...
```

For learning, easy to understand.

But embedding APIs can accept multiple inputs together, so you can do:

```python
descriptions = [
    movie["description"]
    for movie in movies
]

response = await client.embeddings.create(
    model=EMBEDDING_MODEL,
    input=descriptions,
)
```

That's why my FastAPI example uses `embed_many()`.

The current Python embedding API accepts either a single input or multiple inputs in the request. ([GitHub][1])

Backend lesson:

```text
N network calls
```

and:

```text
1 batched network call
```

are very different even if the mathematical output is equivalent.

---

# 24. Follow one request through the entire FastAPI system

Suppose:

```http
GET /movies/search?query=sad%20romantic%20movie&k=3
```

FastAPI receives:

```python
query = "sad romantic movie"
k = 3
```

Then:

```python
await movie_service.search(
    query,
    3,
)
```

Inside:

```python
query_embedding = await embed(...)
```

Suppose conceptually:

```text
"sad romantic movie"

↓

Q = [
  0.12,
  -0.04,
  0.71,
  ...
]
```

Your backend now has:

```text
Q
```

and already stored:

```text
Titanic        → VT
La La Land     → VL
DDLJ           → VD
Interstellar   → VI
Batman         → VB
...
```

Then:

```text
cos(Q, VT)
cos(Q, VL)
cos(Q, VD)
cos(Q, VI)
cos(Q, VB)
...
```

Suppose:

```text
Titanic      0.91
La La Land   0.89
DDLJ         0.87
Batman       0.21
Interstellar 0.19
```

Sort:

```text
0.91 Titanic
0.89 La La Land
0.87 DDLJ
0.21 Batman
0.19 Interstellar
```

Take:

```python
[:3]
```

Response:

```json
[
  {
    "title": "Titanic",
    "similarity": 0.91
  },
  {
    "title": "La La Land",
    "similarity": 0.89
  },
  {
    "title": "DDLJ",
    "similarity": 0.87
  }
]
```

Exactly this search → top-three behavior is demonstrated in the lecture. 

---

# 25. Follow the second request

Now:

```http
GET /movies/Interstellar/similar?k=3
```

This time we **do not call the embedding model**.

Find:

```python
selected = Interstellar
```

Get:

```python
query_embedding = selected.embedding
```

Then:

```text
Interstellar vector
        ↓
Gravity vector      → similarity
Martian vector      → similarity
Matrix vector       → similarity
Titanic vector      → similarity
...
        ↓
sort
        ↓
top 3
```

The transcript's example returns Gravity, The Martian and The Matrix for an Interstellar-style query. 

This means the endpoint contains:

```text
zero generative AI
zero new embedding generation
```

after initialization.

It's just mathematics and data retrieval.

That's a powerful realization.

---

# 26. One thing I would improve over the lecture's movie lookup

The transcript notes that finding the clicked movie in the list itself can take:

$$
O(N)
$$

because it loops through titles. 

But don't confuse this problem with vector search.

If you simply need:

```text
movie_id → movie
```

you can keep:

```python
movies_by_id: dict[int, Movie]
```

Then:

```python
movie = movies_by_id[movie_id]
```

is average-case approximately:

$$
O(1)
$$

So I would use:

```http
GET /movies/{movie_id}/similar
```

instead of using title as the primary identifier.

Then:

```text
exact record lookup
      ↓
O(1)-ish hash lookup / indexed DB lookup

nearest-neighbor vector search
      ↓
separate problem
```

Very important backend distinction.

---

# 27. What exactly is stored?

For the demo:

```python
Movie(
    title="Interstellar",
    description="...",
    embedding=[...]
)
```

Conceptually:

```text
Movie
├── title
├── description
└── embedding
```

A database representation might eventually become:

```text
movies
────────────────────────────────
id
title
description
year
rating
genre
...

movie_embeddings
────────────────────────────────
movie_id
embedding
embedding_model
embedding_version
created_at
```

Or with PostgreSQL + pgvector, depending on the architecture, it can live in the same table.

Why store the embedding model/version?

Because vectors from different embedding spaces generally should not simply be mixed.

If movie A was embedded using:

```text
Model X
```

and the query using:

```text
Model Y
```

their coordinate spaces may not mean the same thing.

You generally need:

```text
documents → same embedding model
queries   → compatible same embedding space
```

This is a fundamental vector-search invariant.

---

# 28. This lecture isn't really building “Netflix”

The transcript itself eventually acknowledges this.

Netflix-style recommendations may use:

user history, ratings, viewing behavior, other users' behavior and more. 

This demo is primarily:

# Content-based semantic similarity

You're asking:

```text
What content resembles this query/movie?
```

It isn't really doing personalization.

A personalized system asks things like:

```text
What does Suyash tend to watch?

Does he finish science-fiction movies?

What did users similar to him enjoy?

What has he already watched?

What genres does he skip?

Which recommendations did he click?

What's trending?

What is geographically available?

What should be explored versus exploited?
```

That is a significantly larger recommendation-system problem.

The lecture intentionally simplifies it.

That's appropriate.

---

# 29. The beautiful connection to RAG

This is where this lecture becomes especially relevant to your AI-backend path.

What you've built is:

```text
Query
 ↓
Embedding
 ↓
Vector Search
 ↓
Relevant Documents
```

That is the **retrieval** portion of RAG.

RAG adds one more step:

```text
User question
      ↓
Embedding
      ↓
Vector Search
      ↓
Relevant documents
      ↓
LLM
      ↓
Generated answer
```

Your movie app stops here:

```text
Vector Search
      ↓
movies returned
```

because generation isn't required.

A RAG app continues:

```text
Vector Search
      ↓
retrieved chunks
      ↓
LLM
      ↓
answer
```

So conceptually:

```text
Movie recommendation demo

Query
 ↓
Embedding
 ↓
Retriever
 ↓
Top K
 ↓
Response
```

versus:

```text
RAG

Question
 ↓
Embedding
 ↓
Retriever
 ↓
Top K chunks
 ↓
LLM
 ↓
Generated response
```

That is why understanding this lecture properly will make RAG much easier.

---

# 30. Where the vector database eventually enters

Right now:

```python
self.movies: list[Movie]
```

contains everything.

Then:

```python
for movie in self.movies:
```

checks every movie.

Later you replace this:

```text
Python list
+
Python cosine loop
```

with something like:

```text
Vector database
+
vector index
+
nearest-neighbor query
```

Your service changes conceptually from:

```python
for movie in movies:
    score = cosine_similarity(
        query_vector,
        movie.embedding,
    )

sort(...)
return top_k(...)
```

to something like:

```python
results = vector_store.search(
    vector=query_vector,
    k=3,
)
```

But notice:

### The concept hasn't changed.

You still have:

```text
query → embedding → nearest neighbors → top K
```

Only the implementation of:

```text
nearest neighbors
```

became sophisticated.

This is exactly where:

```text
KNN
Exact NN
ANN
HNSW
IVF
vector indexes
```

enter the picture.

---

# 31. This is why vector DBs aren't magical databases

A common beginner mental model is:

```text
Vector DB = AI database.
```

No.

A better model:

```text
Vector database
=
database/storage
+
specialized vector operations
+
nearest-neighbor indexes
+
metadata filtering
+
persistence
```

The fundamental object is still:

```python
[0.02, -0.13, 0.84, ...]
```

The fundamental question is still:

```text
Which stored vectors are closest to this vector?
```

The lecture first implements this manually so you understand what's happening.

That's actually the right learning order:

```text
embedding
 ↓
manual cosine
 ↓
manual KNN
 ↓
understand complexity
 ↓
vector index
 ↓
vector DB
```

If you directly jump to:

```python
vector_db.similarity_search(...)
```

you know the API but not the engineering.

---

# 32. Think about the entire system as two phases

This is perhaps the cleanest mental model.

### Indexing / ingestion phase

```text
                    MOVIE CATALOG

                     movies.json
                          │
                          ▼
                    read movies
                          │
                          ▼
                     descriptions
                          │
                          ▼
                  embedding model
                          │
                          ▼
                movie embeddings
                          │
                          ▼
              memory / database /
                  vector database
```

This is primarily concerned with:

```text
preparation
indexing
storage
```

### Query phase

```text
USER
 │
 │ "space survival film"
 ▼
FastAPI
 │
 ▼
Embedding model
 │
 ▼
Query vector
 │
 ▼
Vector search
 │
 ▼
Top K movie IDs
 │
 ▼
Fetch movie metadata
 │
 ▼
JSON response
 │
 ▼
Frontend
```

This separation becomes hugely important in real AI systems.

---

# 33. FastAPI itself is not doing the AI

This is also important for your learning.

FastAPI's role is:

```text
HTTP server / application framework
```

It handles:

```text
GET /movies/search
```

parses:

```python
query: str
k: int
```

calls:

```python
MovieService.search()
```

serializes:

```python
list[MovieMatch]
```

and sends HTTP JSON.

FastAPI doesn't care whether inside your service you use:

```text
SQL
Redis
Kafka
LLM
embedding model
vector DB
machine learning
```

That's business logic/infrastructure.

So your architecture is:

```text
                        AI layer
                     ┌────────────┐
                     │ Embeddings │
                     └─────┬──────┘
                           │
HTTP                       │
Client → FastAPI → Service ├──→ Vector Search
                           │
                           └──→ Database
```

This is why Python + FastAPI is a natural environment for the AI-backend work you're studying.

---

# 34. Also notice the async boundary

Here's an important backend engineering distinction.

This:

```python
await client.embeddings.create(...)
```

involves a remote HTTP call.

That is **I/O-bound**.

Using an async client fits FastAPI well. The current OpenAI Python SDK provides `AsyncOpenAI`, where requests are awaited rather than blocking the request handler. ([GitHub][3])

But this:

```python
cosine_similarity(a, b)
```

is local CPU computation.

There is nothing to `await`.

So:

```python
query_embedding = await embed(query)
```

but:

```python
score = cosine_similarity(
    query_embedding,
    movie.embedding
)
```

This distinction is very important:

```text
network operation
→ async I/O

pure mathematics
→ CPU work
```

For 30 movies, CPU work is irrelevant.

For millions of 1536-dimensional vectors, you absolutely should not run a giant Python loop inside every FastAPI request.

At that point:

```text
FastAPI
 ↓
vector-search engine
```

should handle it.

---

# 35. One other huge production distinction: exact vs approximate search

The lecture's algorithm gives the true results according to the similarity metric because it compares the query with everything.

That's:

# Exact nearest-neighbor search

```text
Q vs V1
Q vs V2
Q vs V3
...
Q vs VN
```

The benefit:

```text
exact answer
```

The cost:

$$
O(ND)
$$

A large vector system usually uses:

# Approximate nearest-neighbor search

Instead of asking:

```text
check every single vector
```

it builds an index that helps answer:

```text
where in the vector space are likely good candidates?
```

Then it examines a much smaller subset.

You trade a small amount of perfect-recall certainty for much faster queries.

That is where the **tree/index/HNSW discussion from your other vector-search lecture** connects to this project.

---

# 36. Another important correction: “vector database solves everything”

Not quite.

Suppose you have:

```text
movie ID = 5832
```

and need:

```text
get movie 5832
```

that's normal database indexing.

Something like:

```sql
SELECT *
FROM movies
WHERE id = 5832;
```

B-tree/hash/primary-key indexing.

But:

```text
Find 10 movies whose semantic vectors are
closest to this 1536-dimensional vector
```

is a fundamentally different query.

That requires vector distance/similarity search.

This is why ordinary database indexes and vector indexes solve different search problems.

---

# 37. Think of semantic search as “search by meaning”

Traditional search:

```text
query:
"space movie"

description:
"an astronaut travels through a wormhole"
```

Keyword logic may say:

```text
"space" not present
→ poor match
```

Semantic embedding search may understand:

```text
space
astronaut
wormhole
galaxy
planet
NASA
interstellar travel
```

as semantically related concepts.

Therefore:

```text
keyword search
≈ matching words

semantic search
≈ matching meaning
```

Of course modern search engines often combine both.

That's called hybrid search in many systems:

```text
lexical relevance
+
semantic/vector relevance
```

---

# 38. Why the model must embed the query too

This is easy to overlook.

Suppose movie embeddings exist in vector space:

```text
M1 = [...]
M2 = [...]
M3 = [...]
```

Your user gives:

```text
"romantic tragedy"
```

You cannot calculate:

```python
cosine_similarity(
    "romantic tragedy",
    movie_embedding
)
```

because:

```text
string ≠ vector
```

You first need:

```text
"romantic tragedy"
       ↓
same embedding model
       ↓
Q = [...]
```

Now:

```text
vector vs vector
```

is possible.

That's exactly what the lecture's first search flow does. 

---

# 39. Why compatible embedding spaces are critical

Suppose your movie embeddings were generated using model A:

```text
movie → Model A → vector
```

and query embeddings using unrelated model B:

```text
query → Model B → vector
```

Even if both happen to return 1536 numbers, those dimensions do not necessarily represent the same learned space.

It's like saying:

```text
Movie coordinates:
latitude/longitude

Query coordinates:
RGB colors
```

Both could contain numbers.

Comparing them is meaningless.

Therefore, conceptually:

```text
documents
and
queries
```

need compatible embeddings from the same embedding space.

---

# 40. What does “precompute” save?

Without precomputation, every search would require:

```text
embed query
+
embed movie 1
+
embed movie 2
+
...
+
embed movie N
+
compare
```

With precomputation:

```text
embed query
+
compare against already stored vectors
```

And clicked-movie recommendation is even cheaper:

```text
fetch existing movie vector
+
compare
```

No model needed.

The transcript makes this distinction repeatedly. 

So precomputation reduces:

```text
API calls
latency
cost
duplicate computation
```

while trading for:

```text
storage
```

Classic systems-engineering tradeoff:

> Compute versus storage.

---

# 41. Now the complete backend picture

Keep this picture in your head:

```text
                   ┌──────────────────────┐
                   │     Movie Catalog    │
                   └──────────┬───────────┘
                              │
                              ▼
                      descriptions
                              │
                              ▼
                   ┌────────────────────┐
                   │ Embedding Model    │
                   └─────────┬──────────┘
                             │
                             ▼
                    movie vectors
                             │
                             ▼
                   ┌────────────────────┐
                   │ Vector Store       │
                   │ / in-memory list   │
                   └─────────┬──────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
              │                             │
      Semantic Search                Similar Movie
              │                             │
              ▼                             ▼
        User text query              Existing movie
              │                             │
              ▼                             ▼
       Embedding Model               fetch embedding
              │                             │
              ▼                             │
        query vector                       │
              └──────────────┬──────────────┘
                             ▼
                     similarity search
                             │
                             ▼
                       nearest K
                             │
                             ▼
                       FastAPI JSON
                             │
                             ▼
                          frontend
```

If you understand that diagram, you understand nearly the entire lecture.

---

# 42. And here is the single most important thing I want you to remember

Don't memorize:

```python
client.embeddings.create(...)
```

Don't memorize:

```python
cosine_similarity(...)
```

Don't memorize:

```python
matches.sort(...)
```

Understand the system:

1. **Represent** meaning as vectors: `text → embedding`.
2. **Precompute** vectors for mostly-static catalog data.
3. For a new natural-language query, compute only the query vector.
4. Search stored vectors for the **nearest neighbors**.
5. Use a similarity metric such as cosine to rank candidates.
6. For an already-existing item, reuse its stored embedding instead of calling the model again.
7. The lecture's Python/Java-list version is **exact brute-force search**; at scale, move vector retrieval into an indexed vector-capable store.
8. FastAPI is the HTTP/application layer around this retrieval pipeline.
9. An LLM is unnecessary unless you actually need **generation/reasoning after retrieval**.
10. Add an LLM after the retrieval stage and you are now moving toward the core architecture behind **RAG**.

That is the real lesson hiding underneath the movie demo.

For your current AI-backend path, I'd study this lecture until you can implement the **manual in-memory FastAPI version yourself without LangChain or LangGraph**. Once you can do that, vector DBs and RAG will stop looking like magic abstractions and will just look like optimized versions of something you already understand.

[1]: https://github.com/openai/openai-python/blob/main/src/openai/resources/embeddings.py?utm_source=chatgpt.com "openai-python/src/openai/resources/embeddings.py at main · openai/openai-python · GitHub"
[2]: https://platform.openai.com/docs/models/default-usage-policies-by-endpoint?utm_source=chatgpt.com "Data controls in the OpenAI platform - OpenAI API"
[3]: https://github.com/openai/openai-python/blob/main/README.md?utm_source=chatgpt.com "openai-python/README.md at main · openai/openai-python · GitHub"
