// Task client (Go port of pythonDemo/taskClient/app.py)
//
// An HTTP server on http://127.0.0.1:8080 with one route:
//
//	GET /ask?message=...  -> runs a "Task Assistant" agent that can use the MCP task tools
//
// The Python version uses the OpenAI Agents SDK (Agent + Runner +
// MCPServerStreamableHttp). There is no official Agents SDK for Go, so this
// file does by hand what the Runner does for us:
//
//  1. connect to the MCP server once at startup and cache its tool list
//  2. send the user's message + the tools to the model (Responses API)
//  3. if the model asks for tools, call them on the MCP server and send the
//     results back; repeat until the model answers with plain text
//
// Run:  go run ./taskClient

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

const (
	instructions = "Use the MCP task tools to handle every task-management request."
	maxTurns     = 10 // the Agents SDK Runner's default max_turns
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// TaskAssistant is our hand-rolled "Agent": a model, instructions and the
// MCP session whose tools it may call.
type TaskAssistant struct {
	ai      openai.Client
	model   string
	session *mcp.ClientSession
	tools   []responses.ToolUnionParam // cached once, like cache_tools_list=True
}

// toOpenAITools converts MCP tool definitions into OpenAI function tools.
func toOpenAITools(mcpTools []*mcp.Tool) ([]responses.ToolUnionParam, error) {
	var tools []responses.ToolUnionParam
	for _, tool := range mcpTools {
		// InputSchema arrives as generic JSON; OpenAI wants a map.
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, err
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			return nil, err
		}
		tools = append(tools, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  schema,
			},
		})
	}
	return tools, nil
}

// toolResultText flattens an MCP tool result into a string for the model.
func toolResultText(result *mcp.CallToolResult) string {
	var values []string
	for _, block := range result.Content {
		if text, ok := block.(*mcp.TextContent); ok {
			values = append(values, text.Text)
		} else {
			raw, _ := json.Marshal(block)
			values = append(values, string(raw))
		}
	}
	return strings.Join(values, "\n")
}

// Run is our version of Runner.run(agent, message).
func (a *TaskAssistant) Run(ctx context.Context, message string) (string, error) {
	inputs := responses.ResponseInputParam{
		responses.ResponseInputItemParamOfMessage(message, responses.EasyInputMessageRoleUser),
	}

	for range maxTurns {
		response, err := a.ai.Responses.New(ctx, responses.ResponseNewParams{
			Model:        a.model,
			Instructions: openai.String(instructions),
			Tools:        a.tools,
			Input:        responses.ResponseNewParamsInputUnion{OfInputItemList: inputs},
			// The Agents SDK sends reasoning effort "none" by default for its
			// default model, so we do the same.
			Reasoning: shared.ReasoningParam{Effort: shared.ReasoningEffortNone},
		})
		if err != nil {
			return "", err
		}

		// Keep everything the model produced (messages, tool calls, ...) in
		// the conversation, so the next turn has the full context.
		var toolCalls []responses.ResponseFunctionToolCall
		for _, item := range response.Output {
			var input responses.ResponseInputItemUnionParam
			param.SetJSON([]byte(item.RawJSON()), &input)
			inputs = append(inputs, input)

			if item.Type == "function_call" {
				toolCalls = append(toolCalls, item.AsFunctionCall())
			}
		}

		if len(toolCalls) == 0 {
			return response.OutputText(), nil
		}

		for _, call := range toolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
				return "", fmt.Errorf("bad tool arguments: %w", err)
			}
			result, err := a.session.CallTool(ctx, &mcp.CallToolParams{Name: call.Name, Arguments: args})
			if err != nil {
				return "", err
			}
			inputs = append(inputs, responses.ResponseInputItemUnionParam{
				OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
					CallID: openai.String(call.CallID),
					Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
						OfString: openai.String(toolResultText(result)),
					},
				},
			})
		}
	}

	return "", errors.New("max turns exceeded")
}

func main() {
	_ = godotenv.Load() // .env in the goDemo folder (run from there)

	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("Set OPENAI_API_KEY in the .env file")
	}

	ctx := context.Background()

	// "lifespan": connect to the MCP server before serving requests.
	client := mcp.NewClient(&mcp.Implementation{Name: "Task MCP server", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: getenv("MCP_SERVER_URL", "http://localhost:8081/mcp"),
	}, nil)
	if err != nil {
		log.Fatalf("connect to MCP server: %v", err)
	}
	defer session.Close()

	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("list MCP tools: %v", err)
	}
	tools, err := toOpenAITools(listed.Tools)
	if err != nil {
		log.Fatalf("convert MCP tools: %v", err)
	}

	agent := &TaskAssistant{
		ai: openai.NewClient(), // reads OPENAI_API_KEY from the environment
		// Same default model as the Agents SDK (override with OPENAI_DEFAULT_MODEL).
		model:   getenv("OPENAI_DEFAULT_MODEL", "gpt-5.6-luna"),
		session: session,
		tools:   tools,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ask", func(w http.ResponseWriter, r *http.Request) {
		message := r.URL.Query().Get("message")

		// Same validation as FastAPI's Query(min_length=1).
		if message == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, `{"detail":"message must be a non-empty string"}`)
			return
		}

		answer, err := agent.Run(r.Context(), message)
		if err != nil {
			log.Println(err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"detail":"Unable to process the request"}`)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, answer)
	})

	addr := "127.0.0.1:8080"
	log.Printf("Task client running on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
