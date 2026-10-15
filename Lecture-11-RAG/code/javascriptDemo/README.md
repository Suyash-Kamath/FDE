# Lecture 11 – RAG (JavaScript port)

Node.js 20+ (ESM, plain JavaScript) ports of every program in `../pythonDemo` and `../goDemo`.

| JavaScript file | Ported from | What it does |
| --- | --- | --- |
| `main.js` | `pythonDemo/src/pythondemo/main.py` | `ingest`: reads the ShopSphere PDFs, makes 300-token chunks (50-token overlap), embeds them with OpenAI and upserts them into Pinecone (it creates the `shop-support` index if needed). `chat`: runs the RAG chat loop. |
| `mainWithoutVectordb.js` | `pythonDemo/src/pythondemo/main_without_vectordb.py` | The same RAG flow with no vector DB. Embeddings stay in memory and cosine similarity is computed by hand. |
| `code-from-guruji/main.js` | `pythonDemo/.../code-from-guruji/main.py` and `goDemo/code-from-guruji/main.go` | Loads the PDFs into Pinecone (1024-dim embeddings), then answers one question passed on the command line. |
| `code-from-guruji/server.js` | *(extra)* | Express server for `code-from-guruji/static/index.html` (copied from `goDemo/code-from-guruji/static`). It serves `GET /api/v1/query?question=...` and returns `{ answer }`. |
| `using-langchain/index.js` | `pythonDemo/.../using-langchain/index.py` | LangChain JS: PDF to `Document`s, `RecursiveCharacterTextSplitter` (1000/200), Gemini embeddings cut to 768 dims, then `PineconeStore`. |
| `using-langchain/query.js` | `pythonDemo/.../using-langchain/query.py` | Rewrites the query with Gemini, embeds it, takes the top 10 from Pinecone, then Gemini answers using chat history. |
| `from-go/main.js`, `index.js`, `query.js`, `shared.js` | `goDemo/main.go`, `index.go`, `query.go` | OpenAI-only version of the LangChain demo. `index` covers PDF, recursive chunks, 1536-dim OpenAI embeddings and Pinecone. `chat` covers query rewrite, top-10 retrieval and the answer. `shared.js` holds the shared part of `main.go`. |

PDFs are read from the existing folders and are not copied:

- `main.js` and `mainWithoutVectordb.js` read `../pythonDemo/src/pythondemo/knowledge/*.pdf`.
- `code-from-guruji/` reads `../goDemo/code-from-guruji/knowledge/*.pdf`. The originals point at a `spring-code/...` folder that is not in this repo.
- `using-langchain/` and `from-go/` read `../pythonDemo/src/pythondemo/using-langchain/Dsa.pdf`.

## Setup

```bash
npm install
cp .env.example .env   # then fill in the keys you need
```

| Variable | Used by |
| --- | --- |
| `OPENAI_API_KEY` | `main.js`, `mainWithoutVectordb.js`, `code-from-guruji/`, `from-go/` |
| `OPENAI_CHAT_MODEL` (optional) | `main.js` |
| `PINECONE_API_KEY` | everything except `mainWithoutVectordb.js` |
| `PINECONE_INDEX_NAME` | `using-langchain/` and `from-go/` (required); `code-from-guruji/` (default `shop-support`) |
| `PINECONE_NAMESPACE` (optional) | `code-from-guruji/` (default `policies`), `from-go/` (default namespace) |
| `GEMINI_API_KEY` | `using-langchain/` |
| `OWN_KEY` (optional) | `code-from-guruji/` fallback key |

Every program loads `.env` from this folder, whatever directory you run it from.

## Run

```bash
# main.py
npm run ingest                      # node main.js ingest
npm run chat                        # node main.js chat

# main_without_vectordb.py
npm run without-vectordb

# code-from-guruji
npm run guruji -- "Can I return a product after 20 days?"
npm run guruji:server               # http://localhost:8080 (PORT to override)

# using-langchain (Gemini + Pinecone, 768-dim index)
npm run langchain:index
npm run langchain:query

# goDemo/main.go (OpenAI + Pinecone, 1536-dim index)
npm run go-port:index               # node from-go/main.js index
npm run go-port:chat                # node from-go/main.js chat
```

## Notes on the port

- `tiktoken` is replaced by `js-tiktoken` (same `cl100k_base` encoding) and `pypdf` by `pdf-parse`.
- Python's `PyPDFLoader` (`langchain_community`) is replaced by a small helper in `using-langchain/index.js`. It builds one LangChain `Document` per page with `source` and 0-indexed `page` metadata. `@langchain/community` has optional peer dependencies that conflict with this project.
- `@pinecone-database/pinecone` is pinned to 5.x because `@langchain/pinecone` requires it as a peer dependency.
- `code-from-guruji/main.js` follows the Python version and uses real token chunking. The Go version used word chunking because Go had no tiktoken.
- `from-go/` uses LangChain JS's `RecursiveCharacterTextSplitter`, which is what `index.go` re-implements by hand.
