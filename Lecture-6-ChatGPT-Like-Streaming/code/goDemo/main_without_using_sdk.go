package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	openAIResponsesURL = "https://api.openai.com/v1/responses"
	rawModel           = "gpt-4o-mini"
)

const rawSystemPrompt = `
You are a funny AI chatbot. You reply everything sarcastically.
`

// rawMessage mirrors the {"role": ..., "content": ...} shape the Python
// history list used, sent straight to the /v1/responses endpoint.
type rawMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// rawStreamEvent is only the subset of the SSE event payload we care about.
type rawStreamEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
}

// ChatService talks to OpenAI over raw HTTP (no SDK) and keeps track of the
// conversation history, the same way chat_service.py does.
type ChatService struct {
	apiKey  string
	history []rawMessage
}

func NewChatService(apiKey string) *ChatService {
	return &ChatService{apiKey: apiKey}
}

// Stream sends the user's message to OpenAI's Responses API with
// "stream": true and calls onDelta for every chunk of the streamed reply
// as it arrives.
func (s *ChatService) Stream(
	ctx context.Context,
	message string,
	onDelta func(delta string),
) error {

	// USER role
	s.history = append(s.history, rawMessage{Role: "user", Content: message})

	body, err := json.Marshal(map[string]any{
		"model":        rawModel,
		"instructions": rawSystemPrompt,
		"input":        s.history,
		"stream":       true,
	})

	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		openAIResponsesURL,
		bytes.NewReader(body),
	)

	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openai request failed: %s: %s", resp.Status, errBody)
	}

	var fullResponse string

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {

		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")

		if payload == "[DONE]" {
			break
		}

		var event rawStreamEvent

		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}

		if event.Type == "response.output_text.delta" {
			fullResponse += event.Delta
			onDelta(event.Delta)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// ASSISTANT role
	s.history = append(s.history, rawMessage{Role: "assistant", Content: fullResponse})

	return nil
}

// ChatController is the thin HTTP layer on top of ChatService, the Go
// equivalent of chat_controller.py.
type ChatController struct {
	chatService *ChatService
}

func NewChatController(chatService *ChatService) *ChatController {
	return &ChatController{chatService: chatService}
}

type chatRequest struct {
	Message string `json:"message"`
}

func (c *ChatController) Chat(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req chatRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)

	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	err := c.chatService.Stream(r.Context(), req.Message, func(delta string) {
		w.Write([]byte(delta))
		flusher.Flush()
	})

	if err != nil {
		log.Printf("chat error: %v", err)
	}
}

func main() {

	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file could not be loaded")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")

	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is not present in your .env file.")
	}

	chatService := NewChatService(apiKey)
	chatController := NewChatController(chatService)

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", chatController.Chat)

	log.Println("Server listening on http://localhost:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
