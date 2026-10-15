package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const systemPrompt = "You are a helpful AI assistant. Answer questions clearly and simply."

// ChatCompleter is the one method we need from the OpenAI client.
// The real client.Chat.Completions satisfies it, and tests can pass a fake.
type ChatCompleter interface {
	New(ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption) (*openai.ChatCompletion, error)
}

// Image is an uploaded image file: its raw bytes and its MIME type (e.g. image/png).
type Image struct {
	Data     []byte
	MimeType string
}

// ChatService works like the Spring singleton service: history is shared and kept in memory.
type ChatService struct {
	client  ChatCompleter
	model   string
	history []openai.ChatCompletionMessageParamUnion
	lock    sync.Mutex
}

func NewChatService(client ChatCompleter, model string) *ChatService {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &ChatService{client: client, model: model}
}

// Chat sends the message (and optional image) to the model along with the history.
func (s *ChatService) Chat(ctx context.Context, message string, image *Image) (string, error) {
	// Keep simultaneous requests and clearing in a consistent turn order.
	s.lock.Lock()
	defer s.lock.Unlock()

	userMessage := openai.UserMessage(message)
	if image != nil && len(image.Data) > 0 {
		encodedImage := base64.StdEncoding.EncodeToString(image.Data)
		userMessage = openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
			openai.TextContentPart(message),
			openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
				URL: fmt.Sprintf("data:%s;base64,%s", image.MimeType, encodedImage),
			}),
		})
	}

	messages := []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(systemPrompt)}
	messages = append(messages, s.history...)
	messages = append(messages, userMessage)

	response, err := s.client.New(ctx, openai.ChatCompletionNewParams{
		Model:       s.model,
		Temperature: openai.Float(0.4),
		Messages:    messages,
	})
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
		return "", errors.New("the model returned no text response")
	}
	reply := response.Choices[0].Message.Content

	// Only commit a complete turn after the API call succeeds.
	s.history = append(s.history, userMessage, openai.AssistantMessage(reply))
	return reply, nil
}

// ClearHistory forgets the whole conversation.
func (s *ChatService) ClearHistory() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.history = nil
}
