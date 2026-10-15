import { readFile } from "node:fs/promises";
import path from "node:path";

import { PROJECT_ROOT } from "../config.js";
import { Movie } from "../model/movie.js";
import { MovieData } from "../model/movieData.js";
import { MovieMatch } from "../model/movieMatch.js";

const RESOURCES_DIR = path.join(PROJECT_ROOT, "resources");
const TOP_K_MATCHES = 3;

// Thrown when a title is not in the catalog (the Python version raises ValueError).
export class MovieNotFoundError extends Error {
    constructor(title) {
        super(`Movie not found: ${title}`);
        this.name = "MovieNotFoundError";
    }
}

export class MovieService {
    /**
     * @param {{ embed(text: string): Promise<number[]> }} embeddingClient
     */
    constructor(embeddingClient) {
        this.embeddingClient = embeddingClient;
        /** @type {Movie[]} */
        this.moviesEmbedding = [];
    }

    async initializeMovies() {
        const resource = path.join(RESOURCES_DIR, "movies.json");

        const raw = await readFile(resource, "utf-8");
        const movieDataList = JSON.parse(raw).map((movieData) => new MovieData(movieData));

        for (const movieData of movieDataList) {
            const embedding = await this.embeddingClient.embed(movieData.description);

            this.moviesEmbedding.push(
                new Movie({
                    title: movieData.title,
                    description: movieData.description,
                    embedding,
                }),
            );
        }

        console.log(`${this.moviesEmbedding.length} movies loaded with embeddings.`);
    }

    /**
     * @param {string} query
     * @returns {Promise<MovieMatch[]>}
     */
    async search(query) {
        const userQueryEmbedding = await this.embeddingClient.embed(query);

        const matches = this.moviesEmbedding.map(
            (movie) =>
                new MovieMatch({
                    title: movie.title,
                    description: movie.description,
                    match: MovieService._cosineSimilarity(userQueryEmbedding, movie.embedding),
                }),
        );

        return MovieService._topKMatches(MovieService._sortBySimilarity(matches));
    }

    /**
     * @param {string} title
     * @returns {MovieMatch[]}
     */
    similarMovies(title) {
        const selectedMovie = this._findMovie(title);

        const matches = this.moviesEmbedding
            .filter((movie) => movie.title.toLowerCase() !== title.toLowerCase())
            .map(
                (movie) =>
                    new MovieMatch({
                        title: movie.title,
                        description: movie.description,
                        match: MovieService._cosineSimilarity(selectedMovie.embedding, movie.embedding),
                    }),
            );

        return MovieService._topKMatches(MovieService._sortBySimilarity(matches));
    }

    _findMovie(title) {
        for (const movie of this.moviesEmbedding) {
            if (movie.title.toLowerCase() === title.toLowerCase()) {
                return movie;
            }
        }

        throw new MovieNotFoundError(title);
    }

    static _cosineSimilarity(a, b) {
        let dotProduct = 0;
        let sumA = 0;
        let sumB = 0;

        // Like Python's zip(): stop at the shorter vector.
        const length = Math.min(a.length, b.length);
        for (let i = 0; i < length; i++) {
            dotProduct += a[i] * b[i];
        }
        for (const x of a) sumA += x * x;
        for (const y of b) sumB += y * y;

        const normA = Math.sqrt(sumA);
        const normB = Math.sqrt(sumB);

        if (normA === 0 || normB === 0) {
            return 0.0;
        }

        return dotProduct / (normA * normB);
    }

    static _sortBySimilarity(matches) {
        // Copy first so we don't mutate the input (Python's sorted() returns a new list).
        return [...matches].sort((x, y) => y.match - x.match);
    }

    static _topKMatches(matches) {
        return matches.slice(0, TOP_K_MATCHES);
    }
}
