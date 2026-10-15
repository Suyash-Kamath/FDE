// Task MCP server (Go port of pythonDemo/taskServer/server.py)
//
// Lecture 14 only had tools. This server shows all three MCP building blocks:
//
//	Tools     -> create_task, list_tasks, complete_task  (the model calls these)
//	Resources -> task://guidelines                       (data the client reads)
//	Prompts   -> plan_day                                (reusable prompt templates)
//
// Served over Streamable HTTP at http://127.0.0.1:8081/mcp. Like the Python
// default, the server is stateful: each client gets an MCP session id
// (Mcp-Session-Id header) and responses are streamed as SSE.
//
// Run:  go run ./taskServer

package main

import (
	"context"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================================
// Tool inputs / outputs
// ==========================================================
//
// The SDK builds each tool's JSON schema from these structs.
// Python's MCP SDK wraps a plain return value as {"result": ...} in
// structuredContent (and advertises a matching outputSchema). The Out structs
// do the same, so Python, JS and Go servers are interchangeable.

type TitleInput struct {
	Title string `json:"title"`
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

func createTask(_ context.Context, _ *mcp.CallToolRequest, in TitleInput) (*mcp.CallToolResult, TextOutput, error) {
	return textResult(createTaskValue(in.Title))
}

func listTasks(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListOutput, error) {
	values := listTaskValues()

	// Python turns a list[str] into one text block per task.
	content := []mcp.Content{}
	for _, task := range values {
		content = append(content, &mcp.TextContent{Text: task})
	}
	return &mcp.CallToolResult{Content: content}, ListOutput{Result: values}, nil
}

func completeTask(_ context.Context, _ *mcp.CallToolRequest, in TitleInput) (*mcp.CallToolResult, TextOutput, error) {
	return textResult(completeTaskValue(in.Title))
}

// ==========================================================
// Resources
// ==========================================================

func taskGuidelines(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "text/plain",
			Text:     getGuidelines(),
		}},
	}, nil
}

// ==========================================================
// Prompts
// ==========================================================

func planDay(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	availableHours := req.Params.Arguments["availableHours"]
	return &mcp.GetPromptResult{
		Description: "Plan pending tasks for the available time",
		Messages: []*mcp.PromptMessage{{
			Role:    "user",
			Content: &mcp.TextContent{Text: planDayPrompt(availableHours)},
		}},
	}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "task-mcp-server", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "create_task", Description: "Create a new task. The title is required."}, createTask)
	mcp.AddTool(server, &mcp.Tool{Name: "list_tasks", Description: "List all pending tasks."}, listTasks)
	mcp.AddTool(server, &mcp.Tool{Name: "complete_task", Description: "Complete a task using its exact title."}, completeTask)

	server.AddResource(&mcp.Resource{
		URI:         "task://guidelines",
		Name:        "task-guidelines",
		Description: "Guidelines for managing and prioritizing tasks",
		MIMEType:    "text/plain",
	}, taskGuidelines)

	server.AddPrompt(&mcp.Prompt{
		Name:        "plan_day",
		Description: "Plan pending tasks for the available time",
		Arguments:   []*mcp.PromptArgument{{Name: "availableHours", Required: true}},
	}, planDay)

	// Default options = stateful sessions + SSE responses (the Python default).
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	addr := "127.0.0.1:8081"
	log.Printf("Task MCP server running on http://%s/mcp", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
