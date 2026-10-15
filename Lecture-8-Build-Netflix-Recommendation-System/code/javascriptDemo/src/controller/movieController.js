import { Router } from "express";

import { MovieNotFoundError } from "../service/movieService.js";

/**
 * Builds the /movies router (the Python version returns a FastAPI APIRouter).
 * @param {import("../service/movieService.js").MovieService} movieService
 */
export function createMovieRouter(movieService) {
    const router = Router();

    // GET /movies/search?query=...  -> MovieMatch[]
    router.get("/search", async (req, res) => {
        const { query } = req.query;
        if (typeof query !== "string") {
            // FastAPI answers 422 when a required query parameter is missing.
            return res.status(422).json({ detail: "Missing required query parameter: query" });
        }

        res.json(await movieService.search(query));
    });

    // GET /movies/:title/similar  -> MovieMatch[]
    router.get("/:title/similar", (req, res) => {
        try {
            res.json(movieService.similarMovies(req.params.title));
        } catch (error) {
            if (error instanceof MovieNotFoundError) {
                return res.status(404).json({ detail: error.message });
            }
            throw error;
        }
    });

    return router;
}
