package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

const model = "gpt-4o-mini"

const systemPrompt = `
You are a funny AI chatbot. You reply everything sarcastically.
`

// ChatService talks to OpenAI (using the official SDK) and keeps track of
// the conversation history, the same way chat_service.py does.
type ChatService struct {
	client  openai.Client
	history []responses.ResponseInputItemUnionParam
}

func NewChatService(client openai.Client) *ChatService {
	return &ChatService{client: client}
}

// Stream sends the user's message to OpenAI and calls onDelta for every
// chunk of the streamed reply as it arrives.
func (s *ChatService) Stream(
	ctx context.Context,
	message string,
	onDelta func(delta string),
) error {

	// USER role
	s.history = append(
		s.history,
		responses.ResponseInputItemParamOfMessage(
			message,
			responses.EasyInputMessageRoleUser,
		),
	)

	// SYSTEM + Conversation History
	stream := s.client.Responses.NewStreaming(
		ctx,
		responses.ResponseNewParams{
			Model:        model,
			Instructions: openai.String(systemPrompt),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: s.history,
			},
		},
	)

	defer stream.Close()

	var fullResponse string

	for stream.Next() {

		event := stream.Current()

		if event.Type == "response.output_text.delta" {
			fullResponse += event.Delta
			onDelta(event.Delta)
		}
	}

	if err := stream.Err(); err != nil {
		return err
	}

	s.history = append(
		s.history,
		responses.ResponseInputItemParamOfMessage(
			fullResponse,
			responses.EasyInputMessageRoleAssistant,
		),
	)

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

	// Official OpenAI Go SDK client
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	chatService := NewChatService(client)
	chatController := NewChatController(chatService)

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", chatController.Chat)

	log.Println("Server listening on http://localhost:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
