package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"goDemo/internal/embedding"
	"goDemo/internal/model"
)

const topKMatches = 3

// ErrMovieNotFound is returned by SimilarMovies when the title does not exist in the catalog.
var ErrMovieNotFound = errors.New("movie not found")

type MovieService struct {
	embeddingClient embedding.Client
	moviesEmbedding []model.Movie
}

func NewMovieService(embeddingClient embedding.Client) *MovieService {
	return &MovieService{embeddingClient: embeddingClient}
}

// InitializeMovies loads the catalog from resourcePath and precomputes an embedding for each movie.
func (s *MovieService) InitializeMovies(ctx context.Context, resourcePath string) error {
	data, err := os.ReadFile(resourcePath)
	if err != nil {
		return fmt.Errorf("reading movies resource: %w", err)
	}

	var movieDataList []model.MovieData
	if err := json.Unmarshal(data, &movieDataList); err != nil {
		return fmt.Errorf("parsing movies resource: %w", err)
	}

	for _, movieData := range movieDataList {
		vector, err := s.embeddingClient.Embed(ctx, movieData.Description)
		if err != nil {
			return fmt.Errorf("embedding movie %q: %w", movieData.Title, err)
		}

		s.moviesEmbedding = append(s.moviesEmbedding, model.Movie{
			Title:       movieData.Title,
			Description: movieData.Description,
			Embedding:   vector,
		})
	}

	fmt.Printf("%d movies loaded with embeddings.\n", len(s.moviesEmbedding))
	return nil
}

// Search ranks the catalog against a free-text query embedding.
func (s *MovieService) Search(ctx context.Context, query string) ([]model.MovieMatch, error) {
	userQueryEmbedding, err := s.embeddingClient.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	matches := make([]model.MovieMatch, 0, len(s.moviesEmbedding))
	for _, movie := range s.moviesEmbedding {
		matches = append(matches, model.MovieMatch{
			Title:       movie.Title,
			Description: movie.Description,
			Match:       cosineSimilarity(userQueryEmbedding, movie.Embedding),
		})
	}

	return topMatches(sortBySimilarity(matches)), nil
}

// SimilarMovies ranks the catalog against the embedding of an existing movie.
func (s *MovieService) SimilarMovies(title string) ([]model.MovieMatch, error) {
	selectedMovie, err := s.findMovie(title)
	if err != nil {
		return nil, err
	}

	matches := make([]model.MovieMatch, 0, len(s.moviesEmbedding))
	for _, movie := range s.moviesEmbedding {
		if strings.EqualFold(movie.Title, title) {
			continue
		}

		matches = append(matches, model.MovieMatch{
			Title:       movie.Title,
			Description: movie.Description,
			Match:       cosineSimilarity(selectedMovie.Embedding, movie.Embedding),
		})
	}

	return topMatches(sortBySimilarity(matches)), nil
}

func (s *MovieService) findMovie(title string) (model.Movie, error) {
	for _, movie := range s.moviesEmbedding {
		if strings.EqualFold(movie.Title, title) {
			return movie, nil
		}
	}

	return model.Movie{}, fmt.Errorf("%w: %s", ErrMovieNotFound, title)
}

func cosineSimilarity(a, b []float64) float64 {
	var dotProduct, normA, normB float64

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func sortBySimilarity(matches []model.MovieMatch) []model.MovieMatch {
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Match > matches[j].Match
	})
	return matches
}

func topMatches(matches []model.MovieMatch) []model.MovieMatch {
	if len(matches) > topKMatches {
		return matches[:topKMatches]
	}
	return matches
}
