// https://certain-mechanic-42c.notion.site/RAG-System-23c3a78e0e22801caa04d16f95df1825
//
// Go RAG system using OpenAI + Pinecone.
//
//   go run . index   -> PDF -> chunks -> OpenAI embeddings (1536-dim) -> Pinecone   (index.go)
//   go run . chat    -> question -> rewrite -> embed -> Pinecone top-K -> answer   (query.go)
//
// This file holds what both commands share: config, clients, and the
// embedding call. Keeping embeddings in ONE place guarantees the chunks and
// the queries are always embedded with the same model and dimensions.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/joho/godotenv"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/pinecone-io/go-pinecone/v6/pinecone"
)

// ==========================================================
// Shared config
// ==========================================================

const (
	embeddingModel = openai.EmbeddingModelTextEmbedding3Small
	embeddingDim   = 1536 // native size of text-embedding-3-small; Pinecone index must be 1536-dim
	maxRetries     = 5    // SDK retries 429/5xx with exponential backoff
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing environment variable %s", key)
	}
	return v
}

// ==========================================================
// Shared clients
// ==========================================================

func newOpenAIClient() openai.Client {
	return openai.NewClient(
		option.WithAPIKey(mustEnv("OPENAI_API_KEY")),
		option.WithMaxRetries(maxRetries),
	)
}

// connectIndex looks up the index host by name, checks its dimension matches
// our embeddings, and opens a connection to PINECONE_NAMESPACE ("" = default).
func connectIndex(ctx context.Context) (*pinecone.IndexConnection, error) {
	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: mustEnv("PINECONE_API_KEY"),
	})
	if err != nil {
		return nil, fmt.Errorf("pinecone client: %w", err)
	}

	indexName := mustEnv("PINECONE_INDEX_NAME")
	idxModel, err := pc.DescribeIndex(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("describe index %q: %w", indexName, err)
	}
	if idxModel.Dimension != nil && int(*idxModel.Dimension) != embeddingDim {
		return nil, fmt.Errorf("index %q has dimension %d but embeddings are %d — create a %d-dim index",
			indexName, *idxModel.Dimension, embeddingDim, embeddingDim)
	}

	idx, err := pc.Index(pinecone.NewIndexConnParams{
		Host:      idxModel.Host,
		Namespace: os.Getenv("PINECONE_NAMESPACE"),
	})
	if err != nil {
		return nil, fmt.Errorf("index connection: %w", err)
	}
	return idx, nil
}

// ==========================================================
// Shared embedding call (used by both index and chat)
// ==========================================================
//
// - One request embeds all texts (Input = array of strings).
// - OpenAI returns L2-normalized vectors, so no manual normalize step.
// - The SDK returns float64; Pinecone wants float32.

func embedTexts(ctx context.Context, client openai.Client, texts []string) ([][]float32, error) {
	resp, err := client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model:      embeddingModel,
		Input:      openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
		Dimensions: openai.Int(embeddingDim),
	})
	if err != nil {
		var apiErr *openai.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 429 {
			return nil, fmt.Errorf("still rate limited after %d retries (check quota / lower batch size): %w", maxRetries, err)
		}
		return nil, err
	}
	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(resp.Data))
	}

	// Results carry an Index; sort by it so vectors line up with inputs.
	sort.Slice(resp.Data, func(i, j int) bool { return resp.Data[i].Index < resp.Data[j].Index })

	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		v := make([]float32, len(d.Embedding))
		for j, x := range d.Embedding {
			v[j] = float32(x)
		}
		out[i] = v
	}
	return out, nil
}

// ==========================================================
// Entry point
// ==========================================================

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  go run . index   Index the PDF into Pinecone
  go run . chat    Start the RAG chat loop`)
}

func main() {
	_ = godotenv.Load() // .env is optional; real env vars also work

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx := context.Background()
	var err error

	switch os.Args[1] {
	case "index":
		err = runIndex(ctx)
	case "chat":
		err = runChat(ctx)
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		log.Fatal(err)
	}
}