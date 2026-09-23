package embedding

import (
	"context"

	"github.com/openai/openai-go/v2"
)

// Client abstracts away the embedding provider, so MovieService does not
// depend on OpenAI directly.
type Client interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

// OpenAIClient is a Client backed by the OpenAI embeddings API.
type OpenAIClient struct {
	client openai.Client
	model  string
}

func NewOpenAIClient(client openai.Client, model string) *OpenAIClient {
	return &OpenAIClient{client: client, model: model}
}

func (c *OpenAIClient) Embed(ctx context.Context, text string) ([]float64, error) {
	response, err := c.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{OfString: openai.String(text)},
		Model: openai.EmbeddingModel(c.model),
	})
	if err != nil {
		return nil, err
	}

	return response.Data[0].Embedding, nil
}
