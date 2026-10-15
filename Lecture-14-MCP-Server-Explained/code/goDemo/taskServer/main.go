// Task MCP server (Go port of pythonDemo/taskServer/server.py)
//
// Exposes three tools over Streamable HTTP at http://127.0.0.1:8081/mcp:
//
//	create_task   -> add a task
//	list_tasks    -> list pending tasks
//	complete_task -> remove a task by its exact title
//
// Like the Python version it runs in *stateless* mode with plain JSON
// responses (stateless_http=True, json_response=True): every POST is handled
// on its own, no MCP session id is issued, and no SSE stream is opened.
//
// Run:  go run ./taskServer

package main

import (
	"context"
	"log"
	"net/http"
	"slices"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// In-memory task list. HTTP requests are served concurrently in Go, so a
// mutex guards it (Python's single event loop doesn't need one).
var (
	mu    sync.Mutex
	tasks []string
)

// ==========================================================
// Tool inputs / outputs
// ==========================================================
//
// The SDK builds each tool's JSON schema from these structs. The `jsonschema`
// tag becomes the property description, like pydantic's Field(description=...).
//
// Python's MCP SDK wraps a plain return value as {"result": ...} in
// structuredContent (and advertises a matching outputSchema). The Out structs
// below do the same, so Python, JS and Go servers are interchangeable.

type CreateTaskInput struct {
	Title string `json:"title" jsonschema:"Title of the task"`
}

type CompleteTaskInput struct {
	Title string `json:"title" jsonschema:"Exact title of the task"`
}

type TextOutput struct {
	Result string `json:"result"`
}

type ListOutput struct {
	Result []string `json:"result"`
}

// textResult returns a string as one text block + structuredContent.
func textResult(text string) (*mcp.CallToolResult, TextOutput, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, TextOutput{Result: text}, nil
}

// ==========================================================
// Tools
// ==========================================================

func createTask(_ context.Context, _ *mcp.CallToolRequest, in CreateTaskInput) (*mcp.CallToolResult, TextOutput, error) {
	mu.Lock()
	defer mu.Unlock()

	tasks = append(tasks, in.Title)
	return textResult("Task created: " + in.Title)
}

func listTasks(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListOutput, error) {
	mu.Lock()
	defer mu.Unlock()

	// Python turns a list[str] into one text block per task.
	content := []mcp.Content{}
	for _, task := range tasks {
		content = append(content, &mcp.TextContent{Text: task})
	}
	result := append([]string{}, tasks...) // copy; never nil, so JSON is [] not null
	return &mcp.CallToolResult{Content: content}, ListOutput{Result: result}, nil
}

func completeTask(_ context.Context, _ *mcp.CallToolRequest, in CompleteTaskInput) (*mcp.CallToolResult, TextOutput, error) {
	mu.Lock()
	defer mu.Unlock()

	i := slices.Index(tasks, in.Title)
	if i == -1 {
		return textResult("Task not found: " + in.Title)
	}
	tasks = slices.Delete(tasks, i, i+1)
	return textResult("Task completed: " + in.Title)
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "task-mcp-server", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "create_task", Description: "Create a new task"}, createTask)
	mcp.AddTool(server, &mcp.Tool{Name: "list_tasks", Description: "List all pending tasks"}, listTasks)
	mcp.AddTool(server, &mcp.Tool{Name: "complete_task", Description: "Complete a task using its exact title"}, completeTask)

	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:    true, // stateless_http=True
			JSONResponse: true, // json_response=True
		},
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	addr := "127.0.0.1:8081"
	log.Printf("Task MCP server running on http://%s/mcp", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
