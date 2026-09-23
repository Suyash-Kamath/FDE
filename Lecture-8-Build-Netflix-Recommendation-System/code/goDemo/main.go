package main

import (
	"context"
	"log"
	"net/http"
	"path/filepath"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"

	"goDemo/internal/config"
	"goDemo/internal/controller"
	"goDemo/internal/embedding"
	"goDemo/internal/service"
)

func main() {
	settings, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	client := openai.NewClient(option.WithAPIKey(settings.OpenAIAPIKey))
	embeddingClient := embedding.NewOpenAIClient(client, settings.EmbeddingModel)
	movieService := service.NewMovieService(embeddingClient)

	ctx := context.Background()
	if err := movieService.InitializeMovies(ctx, filepath.Join("resources", "movies.json")); err != nil {
		log.Fatalf("initializing movies: %v", err)
	}

	mux := http.NewServeMux()
	controller.NewMovieController(movieService).RegisterRoutes(mux)
	mux.Handle("/", http.FileServer(http.Dir("static")))

	log.Println("MovieMatch server listening on :8000")
	if err := http.ListenAndServe(":8000", mux); err != nil {
		log.Fatal(err)
	}
}
