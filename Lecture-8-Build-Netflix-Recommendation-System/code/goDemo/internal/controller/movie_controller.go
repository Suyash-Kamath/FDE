package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"goDemo/internal/service"
)

type MovieController struct {
	movieService *service.MovieService
}

func NewMovieController(movieService *service.MovieService) *MovieController {
	return &MovieController{movieService: movieService}
}

// RegisterRoutes wires the /movies endpoints onto mux.
func (c *MovieController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /movies/search", c.search)
	mux.HandleFunc("GET /movies/{title}/similar", c.similarMovies)
}

func (c *MovieController) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")

	matches, err := c.movieService.Search(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, matches)
}

func (c *MovieController) similarMovies(w http.ResponseWriter, r *http.Request) {
	title := r.PathValue("title")

	matches, err := c.movieService.SimilarMovies(title)
	if err != nil {
		if errors.Is(err, service.ErrMovieNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, matches)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"detail": message})
}
