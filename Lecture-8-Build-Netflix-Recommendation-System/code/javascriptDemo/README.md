# MovieMatch – JavaScript port

Netflix-style movie recommendations using OpenAI embeddings and cosine similarity.
This is a Node.js (Express) port of `../pythonDemo` (FastAPI). It keeps the same layering, routes, port and output.

## Files

| JavaScript                          | Python (`pythonDemo/src/pythondemo/`) | What it does |
|-------------------------------------|---------------------------------------|--------------|
| `main.js`                           | `main.py` + `__init__.py`             | Wires everything together, embeds all movies at startup, serves the API + UI on port 8000 |
| `src/config.js`                     | `config.py`                           | Loads `OPENAI_API_KEY` (required) and `EMBEDDING_MODEL` (default `text-embedding-3-small`) from `.env` |
| `src/embedding.js`                  | `embedding.py`                        | `OpenAIEmbeddingClient` – turns text into an embedding vector |
| `src/model/movie.js`                | `model/movie.py`                      | Movie + its embedding |
| `src/model/movieData.js`            | `model/movie_data.py`                 | One entry of `movies.json` |
| `src/model/movieMatch.js`           | `model/movie_match.py`                | A result: title, description, match score |
| `src/service/movieService.js`       | `movie_service.py`                    | Loads movies, cosine similarity, top-3 search / similar movies |
| `src/controller/movieController.js` | `movie_controller.py`                 | `GET /movies/search?query=...` and `GET /movies/:title/similar` (404 if not found) |
| `static/index.html`                 | `static/index.html`                   | The UI (copy; only "FastAPI" → "Express" in the labels) |
| `resources/movies.json`             | `resources/movies.json`               | The movie catalog (copy) |

## Setup

Needs Node.js 20 or newer.

```bash
npm install
cp .env.example .env   # then put your OpenAI key in .env
```

## Run

```bash
npm start              # same as: node main.js
```

Open http://localhost:8000. At startup each movie description is embedded once, so it costs one embedding call per movie.
