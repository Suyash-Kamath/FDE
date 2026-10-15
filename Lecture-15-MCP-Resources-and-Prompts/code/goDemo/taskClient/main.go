// Task client web app (Go port of pythonDemo/taskClient/main.py)
//
//	GET /ask?message=...  -> chat with the assistant (uses the guidelines resource + tools)
//	GET /plan?hours=...   -> plan the day (uses the plan_day prompt + tools)
//
// The Python app is started with `uvicorn main:app`, which listens on
// http://127.0.0.1:8000 by default - we use the same address.
//
// Run:  go run ./taskClient

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/openai/openai-go/v3"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// textHandler turns fn(query value) into a plain-text endpoint.
// Like FastAPI, a missing required query parameter is a 422.
func textHandler(queryParam string, fn func(context.Context, string) (string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !r.URL.Query().Has(queryParam) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintf(w, `{"detail":"Missing query parameter: %s"}`, queryParam)
			return
		}

		answer, err := fn(r.Context(), r.URL.Query().Get(queryParam))
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, answer)
	}
}

func main() {
	_ = godotenv.Load() // .env in the goDemo folder (run from there)

	ai = openai.NewClient() // reads OPENAI_API_KEY from the environment
	model = getenv("OPENAI_MODEL", "gpt-5.4-mini")
	mcpURL = getenv("MCP_SERVER_URL", "http://127.0.0.1:8081/mcp")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ask", textHandler("message", chat))
	mux.HandleFunc("GET /plan", textHandler("hours", planDay))

	addr := "127.0.0.1:8000"
	log.Printf("Task client running on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
