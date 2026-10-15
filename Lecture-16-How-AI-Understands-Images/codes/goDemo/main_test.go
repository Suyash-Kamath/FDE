package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// fakeCompleter records every request (as plain JSON, like a deep copy) and returns a canned reply.
type fakeCompleter struct {
	requests []map[string]any
	fail     bool
}

func (f *fakeCompleter) New(ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption) (*openai.ChatCompletion, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var request map[string]any
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, err
	}
	f.requests = append(f.requests, request)
	if f.fail {
		return nil, errors.New("simulated provider failure")
	}
	return &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "Test reply"}}},
	}, nil
}

// browser sends requests straight into the handler (like Flask's test_client).
type browser struct {
	t       *testing.T
	handler http.Handler
}

func (b browser) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	b.handler.ServeHTTP(rec, req)
	return rec
}

func (b browser) get(path string) *httptest.ResponseRecorder {
	return b.do(httptest.NewRequest(http.MethodGet, path, nil))
}

func (b browser) delete(path string) *httptest.ResponseRecorder {
	return b.do(httptest.NewRequest(http.MethodDelete, path, nil))
}

// postForm sends a url-encoded form.
func (b browser) postForm(path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return b.do(req)
}

// postFile sends a multipart form with a message and one file in the "image" field.
func (b browser) postFile(path, message string, data []byte, filename, contentType string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("message", message); err != nil {
		b.t.Fatal(err)
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="image"; filename="`+filename+`"`)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		b.t.Fatal(err)
	}
	part.Write(data)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return b.do(req)
}

func TestFrontendChatImageHistoryClearValidationAndFailure(t *testing.T) {
	client := &fakeCompleter{}
	service := NewChatService(client, "")
	app, err := createApp(service)
	if err != nil {
		t.Fatal(err)
	}
	b := browser{t: t, handler: app}

	response := b.get("/")
	if !strings.Contains(response.Body.String(), "Vision Chat") {
		t.Fatal("index page should contain 'Vision Chat'")
	}

	response = b.postForm("/api/chat", url.Values{"message": {"Hello"}})
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if mediaType, _, _ := mime.ParseMediaType(response.Header().Get("Content-Type")); mediaType != "text/plain" {
		t.Fatalf("mimetype = %q, want text/plain", mediaType)
	}
	if response.Body.String() != "Test reply" {
		t.Fatalf("body = %q, want %q", response.Body.String(), "Test reply")
	}
	first := client.requests[0]
	if first["model"] != "gpt-4o-mini" {
		t.Fatalf("model = %v", first["model"])
	}
	if first["temperature"] != 0.4 {
		t.Fatalf("temperature = %v", first["temperature"])
	}
	if role := first["messages"].([]any)[0].(map[string]any)["role"]; role != "system" {
		t.Fatalf("first message role = %v", role)
	}

	response = b.postFile("/api/chat", "Describe it", []byte("image bytes"), "image.png", "image/png")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	messages := client.requests[1]["messages"].([]any)
	if len(messages) != 4 {
		t.Fatalf("len(messages) = %d, want 4", len(messages))
	}
	content := messages[3].(map[string]any)["content"].([]any)
	if text := content[0].(map[string]any)["text"]; text != "Describe it" {
		t.Fatalf("text = %v", text)
	}
	imageURL := content[1].(map[string]any)["image_url"].(map[string]any)["url"]
	if imageURL != "data:image/png;base64,aW1hZ2UgYnl0ZXM=" {
		t.Fatalf("image url = %v", imageURL)
	}

	if code := b.delete("/api/chat").Code; code != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200", code)
	}
	b.postForm("/api/chat", url.Values{"message": {"New conversation"}})
	if n := len(client.requests[2]["messages"].([]any)); n != 2 {
		t.Fatalf("len(messages) after clear = %d, want 2", n)
	}

	if code := b.postForm("/api/chat", url.Values{}).Code; code != http.StatusBadRequest {
		t.Fatalf("missing message status = %d, want 400", code)
	}
	if code := b.postFile("/api/chat", "Bad file", []byte("text"), "file.txt", "text/plain").Code; code != http.StatusBadRequest {
		t.Fatalf("non-image status = %d, want 400", code)
	}
	large := bytes.Repeat([]byte("x"), 10*1024*1024+1)
	if code := b.postFile("/api/chat", "Too large", large, "large.png", "").Code; code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large upload status = %d, want 413", code)
	}

	client.fail = true
	if code := b.postForm("/api/chat", url.Values{"message": {"Failure"}}).Code; code != http.StatusBadGateway {
		t.Fatalf("provider failure status = %d, want 502", code)
	}
	if len(service.history) != 2 {
		t.Fatalf("len(history) = %d, want 2", len(service.history))
	}
}
