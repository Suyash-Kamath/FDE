package main

import (
	"bytes"
	"embed"
	"errors"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// The frontend is embedded into the binary, so `go run .` works from anywhere.
//
//go:embed static
var staticFiles embed.FS

const maxContentLength = 10 * 1024 * 1024

func writeText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	io.WriteString(w, text)
}

// createApp wires up the routes. Pass nil to build a real ChatService from .env,
// or pass your own ChatService (e.g. one with a fake client) in tests.
func createApp(chatService *ChatService) (http.Handler, error) {
	if chatService == nil {
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, errors.New("set OPENAI_API_KEY in .env before starting the demo")
		}
		model := os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = "gpt-4o-mini"
		}
		client := openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithRequestTimeout(60*time.Second),
			option.WithMaxRetries(0),
		)
		chatService = NewChatService(&client.Chat.Completions, model)
	}

	staticFolder, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	indexHTML, err := fs.ReadFile(staticFolder, "index.html")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	index := func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(indexHTML))
	}
	mux.HandleFunc("GET /{$}", index)
	mux.HandleFunc("GET /index.html", index)
	// Like Flask's static_url_path="/static": serve files from the static folder.
	// (http.FileServer would redirect /static/index.html to /static/, so we serve files ourselves.)
	mux.HandleFunc("GET /static/{path...}", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(staticFolder, r.PathValue("path"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, r.PathValue("path"), time.Time{}, bytes.NewReader(data))
	})

	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		// Same idea as Flask's MAX_CONTENT_LENGTH: never read more than 10 MB.
		r.Body = http.MaxBytesReader(w, r.Body, maxContentLength)
		// Parses both multipart (the browser's FormData) and url-encoded forms.
		if err := r.ParseMultipartForm(maxContentLength); err != nil && !errors.Is(err, http.ErrNotMultipart) {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeText(w, http.StatusRequestEntityTooLarge, "Maximum request size is 10 MB.")
				return
			}
			writeText(w, http.StatusBadRequest, "Invalid form data.")
			return
		}

		messageValues, ok := r.PostForm["message"]
		if !ok || len(messageValues) == 0 {
			writeText(w, http.StatusBadRequest, "The message field is required.")
			return
		}
		message := messageValues[0]

		var image *Image
		file, header, err := r.FormFile("image")
		if err == nil {
			defer file.Close()
			mimeType, _, _ := mime.ParseMediaType(header.Header.Get("Content-Type"))
			if !strings.HasPrefix(mimeType, "image/") {
				writeText(w, http.StatusBadRequest, "Please upload an image file.")
				return
			}
			data, err := io.ReadAll(file)
			if err != nil {
				writeText(w, http.StatusBadRequest, "Could not read the uploaded image.")
				return
			}
			image = &Image{Data: data, MimeType: mimeType}
		}

		reply, err := chatService.Chat(r.Context(), message, image)
		if err != nil {
			// Avoid exposing credentials or provider internals to the frontend.
			writeText(w, http.StatusBadGateway, "Unable to get an AI response. Check your API key and try again.")
			return
		}
		writeText(w, http.StatusOK, reply)
	})

	mux.HandleFunc("DELETE /api/chat", func(w http.ResponseWriter, r *http.Request) {
		chatService.ClearHistory()
		w.WriteHeader(http.StatusOK)
	})

	return mux, nil
}

func main() {
	// Load .env from the current folder (run this from goDemo).
	_ = godotenv.Load()

	app, err := createApp(nil)
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "127.0.0.1:" + port
	log.Printf("Running on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, app))
}
