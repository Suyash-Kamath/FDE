package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// --- request/response shapes for the OpenAI chat completions API ---

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// summarize mirrors SummarizeService.summarize(ticket)
func summarize(ticket string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	reqBody := chatRequest{
		Model: "gpt-4o-mini",
		Messages: []chatMessage{
			{Role: "user", Content: "Summarize this support ticket in 2 lines : \n\n" + ticket},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai request failed: %s", string(body))
	}

	var result chatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from model")
	}

	return result.Choices[0].Message.Content, nil
}

// summarizeHandler mirrors SummarizeController.summarize(ticket)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(summary))
}

// main mirrors DemoApplication (bootstraps the server instead of a Spring context)
func main() {
	http.HandleFunc("/api/summarize", summarizeHandler)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}