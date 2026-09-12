package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/openai/openai-go/v3"
)

var client openai.Client

func summarize(ticket string) (string, error) {
	completion, err := client.Chat.Completions.New(
		context.Background(),
		openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(
					"Summarize this support ticket in 2 lines:\n\n" + ticket,
				),
			},
			Model: openai.ChatModelGPT4oMini,
		},
	)

	if err != nil {
		return "", err
	}

	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from model")
	}

	return completion.Choices[0].Message.Content, nil
}

func summarizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	summary, err := summarize(string(body))
	if err != nil {
		log.Println("OpenAI error:", err)
		http.Error(w, "failed to summarize ticket", http.StatusInternalServerError)
		return
	}

	w.Write([]byte(summary))
}

func main() {
	// 1. Load .env FIRST
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found")
	}

	// 2. Check whether key exists
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("OPENAI_API_KEY is missing")
	}

	log.Println("OPENAI_API_KEY found")

	// 3. NOW create the client
	client = openai.NewClient()

	http.HandleFunc("/api/summarize", summarizeHandler)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}