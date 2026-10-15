# Lecture 12 - Advanced RAG (JavaScript)

JavaScript (Node 20+, ESM) port of `../pythonDemo` and `../goDemo`: OpenAI embeddings + Pinecone,
with category filtering, query rewriting, hybrid search (semantic + BM25), Reciprocal Rank Fusion
and LLM reranking.

| JS file     | Python file              | Go file     | What it does |
|-------------|--------------------------|-------------|--------------|
| `main.js`   | `main.py`                | `main.go`   | Entry point: `index` or `chat` command |
| `common.js` | `common.py`              | `main.go`   | Shared config, OpenAI/Pinecone clients, embedding call |
| `index.js`  | `index.py`               | `index.go`  | PDFs -> pages -> recursive chunks -> embeddings -> Pinecone + `chunks.json` |
| `hybrid.js` | `hybrid.py`              | `hybrid.go` | Candidate, tokenizer, BM25 keyword search, Reciprocal Rank Fusion |
| `query.js`  | `query.go` (Python code) | `query.go`  | Chat loop: category -> rewrite -> semantic + BM25 -> RRF -> rerank -> answer |

Chunk IDs, Pinecone metadata and `chunks.json` use the same format as the Python and Go versions,
so an index built by one can be queried by the others.

## Setup

```bash
npm install
cp .env.example .env     # fill in OPENAI_API_KEY, PINECONE_API_KEY, PINECONE_INDEX_NAME (PINECONE_NAMESPACE optional)
mkdir -p knowledge       # put your PDFs here; the file name becomes the category (graphs.pdf -> "graphs")
```

The Pinecone index must be **1536-dim** with the **cosine** metric (`text-embedding-3-small`).

## Run

```bash
npm run index    # node main.js index : index ./knowledge/*.pdf into Pinecone and write ./chunks.json
npm run chat     # node main.js chat  : start the RAG chat loop (needs chunks.json from the index step)
```

In the chat: pick a category (or Enter for "others" = search everything), then ask questions.
`/category` switches category, `/exit` (or `/quit`) quits.
