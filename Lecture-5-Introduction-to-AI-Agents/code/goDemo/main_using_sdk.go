package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

const model = "gpt-4o-mini"

var client = openai.NewClient()

type ToolHandler func(
	ctx context.Context,
	arguments json.RawMessage,
) (string, error)

func functionTool(
	name string,
	description string,
	properties map[string]any,
	required []string,
) responses.ToolUnionParam {
	return responses.ToolUnionParam{
		OfFunction: &responses.FunctionToolParam{
			Name:        name,
			Description: openai.String(description),
			Parameters: map[string]any{
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		},
	}
}

func calculate(
	operation string,
	a float64,
	b float64,
) (float64, error) {

	fmt.Println("Calculator tool called")

	switch operation {
	case "add":
		return a + b, nil

	case "subtract":
		return a - b, nil

	case "multiply":
		return a * b, nil

	case "divide":
		if b == 0 {
			return 0, fmt.Errorf("cannot divide by 0")
		}
		return a / b, nil

	case "mod":
		if b == 0 {
			return 0, fmt.Errorf("cannot calculate mod by 0")
		}
		return math.Mod(a, b), nil

	case "power":
		return math.Pow(a, b), nil

	default:
		return 0, fmt.Errorf(
			"unsupported operation: %s",
			operation,
		)
	}
}

var chatTools = []responses.ToolUnionParam{
	functionTool(
		"calculate",
		"Performs arithmetic calculations. Supported operations: add, subtract, multiply, divide, mod, power.",
		map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"description": "Operation: add, subtract, multiply, divide, mod, power",
			},
			"a": map[string]any{
				"type":        "number",
				"description": "First number",
			},
			"b": map[string]any{
				"type":        "number",
				"description": "Second number",
			},
		},
		[]string{"operation", "a", "b"},
	),

	functionTool(
		"currentWeather",
		"Get the current weather of a city.",
		map[string]any{
			"city": map[string]any{
				"type":        "string",
				"description": "Name of the city",
			},
		},
		[]string{"city"},
	),

	functionTool(
		"getExchangeRate",
		"Gets the latest exchange rate between two currencies.",
		map[string]any{
			"from": map[string]any{
				"type":        "string",
				"description": "Source currency code, for example USD",
			},
			"to": map[string]any{
				"type":        "string",
				"description": "Target currency code, for example INR",
			},
		},
		[]string{"from", "to"},
	),
}

const chatSystemPrompt = `
You are a helpful AI assistant with access to external tools.

Follow these rules:
1. For arithmetic calculations, ALWAYS use the calculator tool.
2. Always use calculator tool for even trivial calculation.
3. For current weather, ALWAYS use the currentWeather tool.
4. For currency conversion or exchange rates, ALWAYS use the getExchangeRate tool.
5. You may call multiple tools when solving a multi-step request.
6. After receiving tool results, explain the answer naturally.
7. Never invent current weather or exchange-rate information.
`

func chat(
	ctx context.Context,
	message string,
	handlers map[string]ToolHandler,
) (string, error) {

	response, err := client.Responses.New(
		ctx,
		responses.ResponseNewParams{
			Model: openai.ChatModel(model),

			Input: responses.ResponseNewParamsInputUnion{
				OfString: openai.String(
					chatSystemPrompt + "\n\nUser: " + message,
				),
			},

			Tools: chatTools,
		},
	)

	if err != nil {
		return "", err
	}

	for {

		var toolOutputs []responses.ResponseInputItemUnionParam
		hasToolCall := false

		for _, item := range response.Output {

			if item.Type != "function_call" {
				continue
			}

			hasToolCall = true

			call := item.AsFunctionCall()

			handler, exists := handlers[call.Name]

			if !exists {
				return "", fmt.Errorf(
					"unknown tool: %s",
					call.Name,
				)
			}

			result, err := handler(
				ctx,
				json.RawMessage(call.Arguments),
			)

			if err != nil {
				result = fmt.Sprintf(
					"Tool execution failed: %v",
					err,
				)
			}

			toolOutputs = append(
				toolOutputs,
				responses.ResponseInputItemUnionParam{
					OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
						CallID: call.CallID,

						Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
							OfString: openai.String(result),
						},
					},
				},
			)
		}

		if !hasToolCall {
			return response.OutputText(), nil
		}

		response, err = client.Responses.New(
			ctx,
			responses.ResponseNewParams{
				Model:              openai.ChatModel(model),
				PreviousResponseID: openai.String(response.ID),

				Input: responses.ResponseNewParamsInputUnion{
					OfInputItemList: toolOutputs,
				},
			},
		)

		if err != nil {
			return "", err
		}
	}
}
