// Chat logic (Go port of pythonDemo/taskClient/chat_service.py)
//
//	chat(message)  -> reads the task://guidelines RESOURCE, puts it in the
//	                  instructions, then lets the model use the MCP TOOLS
//	planDay(hours) -> gets the plan_day PROMPT from the server and sends it to
//	                  the model, which again can use the MCP TOOLS

package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

// Set up in main() after .env is loaded.
var (
	ai     openai.Client
	model  string
	mcpURL string
)

// withMCPClient is Go's version of Python's `async with Client(mcp_url) as mcp_client:`
// - connect, run fn, close.
func withMCPClient[T any](ctx context.Context, fn func(*mcp.ClientSession) (T, error)) (T, error) {
	var zero T
	client := mcp.NewClient(&mcp.Implementation{Name: "task-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: mcpURL}, nil)
	if err != nil {
		return zero, err
	}
	defer session.Close()
	return fn(session)
}

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

func askModel(ctx context.Context, session *mcp.ClientSession, message, instructions string) (string, error) {
	listedTools, err := session.ListTools(ctx, nil)
	if err != nil {
		return "", err
	}
	tools, err := toOpenAITools(listedTools.Tools)
	if err != nil {
		return "", err
	}
	inputs := responses.ResponseInputParam{
		responses.ResponseInputItemParamOfMessage(message, responses.EasyInputMessageRoleUser),
	}

	for range 10 {
		params := responses.ResponseNewParams{
			Model: model,
			Tools: tools,
			Input: responses.ResponseNewParamsInputUnion{OfInputItemList: inputs},
		}
		if instructions != "" {
			params.Instructions = openai.String(instructions)
		}

		response, err := ai.Responses.New(ctx, params)
		if err != nil {
			return "", err
		}

		// inputs.extend(response.output): send the model's own output items
		// back as-is on the next round.
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

		for _, toolCall := range toolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
				return "", err
			}
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: toolCall.Name, Arguments: args})
			if err != nil {
				return "", err
			}
			inputs = append(inputs, responses.ResponseInputItemUnionParam{
				OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
					CallID: openai.String(toolCall.CallID),
					Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
						OfString: openai.String(toolResultText(result)),
					},
				},
			})
		}
	}

	return "", errors.New("the model used too many tool-calling rounds")
}

func chat(ctx context.Context, message string) (string, error) {
	return withMCPClient(ctx, func(session *mcp.ClientSession) (string, error) {
		result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "task://guidelines"})
		if err != nil {
			return "", err
		}
		if len(result.Contents) == 0 || result.Contents[0].Text == "" {
			return "", errors.New("the task guidelines resource must contain text")
		}

		instructions := `You are a task management assistant.

Use the following task management guidelines whenever they are relevant
to the user's question.

TASK GUIDELINES:

` + result.Contents[0].Text

		return askModel(ctx, session, message, instructions)
	})
}

func planDay(ctx context.Context, hours string) (string, error) {
	return withMCPClient(ctx, func(session *mcp.ClientSession) (string, error) {
		result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
			Name:      "plan_day",
			Arguments: map[string]string{"availableHours": hours},
		})
		if err != nil {
			return "", err
		}
		content, ok := result.Messages[0].Content.(*mcp.TextContent)
		if !ok {
			return "", errors.New("the plan_day prompt must contain text")
		}

		return askModel(ctx, session, content.Text, "")
	})
}
