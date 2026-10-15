import path from "node:path";

import express from "express";
import OpenAI from "openai";

import { PROJECT_ROOT, settings } from "./src/config.js";
import { OpenAIEmbeddingClient } from "./src/embedding.js";
import { createMovieRouter } from "./src/controller/movieController.js";
import { MovieService } from "./src/service/movieService.js";

const STATIC_DIR = path.join(PROJECT_ROOT, "static");
const HOST = "0.0.0.0";
const PORT = 8000;

const client = new OpenAI({ apiKey: settings.openaiApiKey });
const embeddingClient = new OpenAIEmbeddingClient(client, settings.embeddingModel);
const movieService = new MovieService(embeddingClient);

// Startup step (FastAPI's lifespan): embed every movie before accepting requests.
await movieService.initializeMovies();

const app = express();
app.use("/movies", createMovieRouter(movieService));
// Serve the UI at "/" (index.html) - mounted after the API routes, like the Python app.
app.use("/", express.static(STATIC_DIR));

app.listen(PORT, HOST, () => {
    console.log(`MovieMatch server listening on http://${HOST}:${PORT}`);
});
