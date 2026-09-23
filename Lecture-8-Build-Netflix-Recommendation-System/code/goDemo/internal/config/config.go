package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

const defaultEmbeddingModel = "text-embedding-3-small"

// Settings holds the runtime configuration loaded from the environment / .env file.
type Settings struct {
	OpenAIAPIKey   string
	EmbeddingModel string
}

// Load reads .env (if present) and required environment variables into Settings.
func Load() (*Settings, error) {
	_ = godotenv.Load() // .env is optional; real env vars still take precedence

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	embeddingModel := os.Getenv("EMBEDDING_MODEL")
	if embeddingModel == "" {
		embeddingModel = defaultEmbeddingModel
	}

	return &Settings{
		OpenAIAPIKey:   apiKey,
		EmbeddingModel: embeddingModel,
	}, nil
}
