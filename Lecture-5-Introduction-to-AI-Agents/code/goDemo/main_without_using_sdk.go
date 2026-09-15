package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultModel = "gpt-4o-mini"

	chatSystemPrompt = `You are a helpful AI assistant with access to external tools.

Follow these rules:
1. For arithmetic calculations, ALWAYS use the calculator tool.
2. Always use calculator tool for even trivial calculation
3. For current weather, ALWAYS use the currentWeather tool.
4. For currency conversion or exchange rates, ALWAYS use the convertCurrency tool.
5. You may call multiple tools when solving a multi-step request.
6. After receiving tool results, explain the answer naturally.
7. Never invent current weather or exchange-rate information.`

	websiteSystemPrompt = `You are an expert frontend website developer.

Your job is to create complete static websites using the available tools.

Follow these rules:
1. Create a separate directory for every website.
2. Create index.html.
3. Create style.css.
4. Create script.js when JavaScript is useful.
5. Build modern, beautiful and responsive websites.
6. Use only HTML, CSS and vanilla JavaScript.
7. Do not just return website code in your response. Actually create the files using tools.
8. After creating the website, list the project files.
9. Read important files again if needed and fix obvious problems.
10. Finish only when the complete website has been created.`
)

var (
	chatHistory    []Message
	websiteHistory []Message
)

// ------------------------------------------------------------
// OpenAI message / tool structures
// ------------------------------------------------------------

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function FunctionTool `json:"function"`
}

type FunctionTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// ------------------------------------------------------------
// OpenAI client
// ------------------------------------------------------------

type OpenAIClient struct {
	APIKey string
	Model  string
	Client *http.Client
}

func NewOpenAIClient() *OpenAIClient {
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = defaultModel
	}

	return &OpenAIClient{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  model,
		Client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *OpenAIClient) ChatCompletion(
	ctx context.Context,
	messages []Message,
	tools []Tool,
) (*ChatCompletionResponse, error) {

	if c.APIKey == "" {
		return nil, errors.New("OPENAI_API_KEY is not set")
	}

	requestBody := ChatCompletionRequest{
		Model:    c.Model,
		Messages: messages,
		Tools:    tools,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAI request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.openai.com/v1/chat/completions",
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenAI request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read OpenAI response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf(
			"OpenAI API returned %d: %s",
			resp.StatusCode,
			string(responseBody),
		)
	}

	var result ChatCompletionResponse

	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("decode OpenAI response: %w", err)
	}

	return &result, nil
}

// ------------------------------------------------------------
// Tool helpers
// ------------------------------------------------------------

func functionTool(
	name string,
	description string,
	properties map[string]any,
) Tool {
	required := make([]string, 0, len(properties))

	for property := range properties {
		required = append(required, property)
	}

	return Tool{
		Type: "function",
		Function: FunctionTool{
			Name:        name,
			Description: description,
			Parameters: map[string]any{
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		},
	}
}

// ------------------------------------------------------------
// Calculator
// ------------------------------------------------------------

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
			return 0, errors.New("cannot divide by 0")
		}
		return a / b, nil

	case "mod":
		if b == 0 {
			return 0, errors.New("cannot calculate mod by 0")
		}
		return math.Mod(a, b), nil

	case "power":
		return math.Pow(a, b), nil

	default:
		return 0, fmt.Errorf(
			"unsupported operation %q",
			operation,
		)
	}
}

// ------------------------------------------------------------
// Weather
// ------------------------------------------------------------

func currentWeather(
	ctx context.Context,
	city string,
) (string, error) {

	fmt.Println("Weather tool called")

	apiKey := os.Getenv("WEATHER_API_KEY")

	if apiKey == "" {
		return "", errors.New("WEATHER_API_KEY is not set")
	}

	params := url.Values{}
	params.Set("key", apiKey)
	params.Set("q", city)

	requestURL :=
		"https://api.weatherapi.com/v1/current.json?" +
			params.Encode()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("create weather request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("weather request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read weather response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf(
			"weather API returned %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	return string(body), nil
}

// ------------------------------------------------------------
// Currency Exchange
// ------------------------------------------------------------

func getExchangeRate(
	ctx context.Context,
	fromCurrency string,
	toCurrency string,
) (string, error) {

	fmt.Println("Currency Exchange tool called")

	requestURL := fmt.Sprintf(
		"https://api.frankfurter.dev/v2/rate/%s/%s",
		url.PathEscape(fromCurrency),
		url.PathEscape(toCurrency),
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf(
			"create exchange request: %w",
			err,
		)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf(
			"exchange request failed: %w",
			err,
		)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf(
			"read exchange response: %w",
			err,
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf(
			"exchange API returned %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	return string(body), nil
}

// ------------------------------------------------------------
// Safe filesystem workspace
// ------------------------------------------------------------

type WebsiteWorkspace struct {
	Root string
}

func NewWebsiteWorkspace() (*WebsiteWorkspace, error) {
	root, err := filepath.Abs("generated-sites")
	if err != nil {
		return nil, fmt.Errorf(
			"resolve generated-sites: %w",
			err,
		)
	}

	return &WebsiteWorkspace{
		Root: root,
	}, nil
}

func (w *WebsiteWorkspace) safePath(relativePath string) (string, error) {

	// filepath.Clean removes things like ./foo/../bar
	cleanPath := filepath.Clean(relativePath)

	// Make path absolute relative to workspace.
	resolved := filepath.Join(w.Root, cleanPath)

	resolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf(
			"resolve path: %w",
			err,
		)
	}

	relative, err := filepath.Rel(w.Root, resolved)
	if err != nil {
		return "", fmt.Errorf(
			"calculate relative path: %w",
			err,
		)
	}

	// Prevent:
	// ../../something
	// ../secret
	// etc.
	if relative == ".." ||
		strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {

		return "", errors.New(
			"access outside generated-sites is not allowed",
		)
	}

	return resolved, nil
}

// ------------------------------------------------------------
// createDirectory
// ------------------------------------------------------------

func (w *WebsiteWorkspace) createDirectory(
	relativePath string,
) (string, error) {

	path, err := w.safePath(relativePath)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return "", fmt.Errorf(
			"failed to create directory: %w",
			err,
		)
	}

	return fmt.Sprintf(
		"Directory created successfully: %s",
		relativePath,
	), nil
}

// ------------------------------------------------------------
// writeFile
// ------------------------------------------------------------

func (w *WebsiteWorkspace) writeFile(
	relativePath string,
	content string,
) (string, error) {

	path, err := w.safePath(relativePath)
	if err != nil {
		return "", err
	}

	parent := filepath.Dir(path)

	if err := os.MkdirAll(parent, 0755); err != nil {
		return "", fmt.Errorf(
			"failed to create parent directory: %w",
			err,
		)
	}

	if err := os.WriteFile(
		path,
		[]byte(content),
		0644,
	); err != nil {
		return "", fmt.Errorf(
			"failed to write file: %w",
			err,
		)
	}

	return fmt.Sprintf(
		"File written successfully: %s",
		relativePath,
	), nil
}

// ------------------------------------------------------------
// readFile
// ------------------------------------------------------------

func (w *WebsiteWorkspace) readFile(
	relativePath string,
) (string, error) {

	path, err := w.safePath(relativePath)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf(
			"failed to read file: %w",
			err,
		)
	}

	return string(data), nil
}

// ------------------------------------------------------------
// listFiles
// ------------------------------------------------------------

func (w *WebsiteWorkspace) listFiles(
	relativePath string,
) (string, error) {

	directory, err := w.safePath(relativePath)
	if err != nil {
		return "", err
	}

	_, err = os.Stat(directory)

	if os.IsNotExist(err) {
		return fmt.Sprintf(
			"Directory does not exist: %s",
			relativePath,
		), nil
	}

	if err != nil {
		return "", fmt.Errorf(
			"failed to stat directory: %w",
			err,
		)
	}

	var files []string

	err = filepath.Walk(
		directory,
		func(
			path string,
			info os.FileInfo,
			err error,
		) error {

			if err != nil {
				return err
			}

			if path == directory {
				return nil
			}

			relative, err := filepath.Rel(
				w.Root,
				path,
			)

			if err != nil {
				return err
			}

			files = append(files, relative)

			return nil
		},
	)

	if err != nil {
		return "", fmt.Errorf(
			"failed to list files: %w",
			err,
		)
	}

	return strings.Join(files, "\n"), nil
}

// ------------------------------------------------------------
// Tool definitions
// ------------------------------------------------------------

var chatTools = []Tool{

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
	),
}

var websiteTools = []Tool{

	functionTool(
		"createDirectory",
		"Creates a new directory inside the website workspace.",
		map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative directory path, for example brewlab",
			},
		},
	),

	functionTool(
		"writeFile",
		"Creates or overwrites a text file inside the website workspace. Use this to create HTML, CSS and JavaScript files.",
		map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative file path, for example brewlab/index.html",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Complete content that should be written into the file",
			},
		},
	),

	functionTool(
		"readFile",
		"Reads the contents of an existing file from the website workspace.",
		map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative file path",
			},
		},
	),

	functionTool(
		"listFiles",
		"Lists all files and directories inside a website project.",
		map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative directory path, for example brewlab",
			},
		},
	),
}

// ------------------------------------------------------------
// Generic agent completion loop
// ------------------------------------------------------------

type ToolHandler func(
	ctx context.Context,
	arguments json.RawMessage,
) (string, error)

func complete(
	ctx context.Context,
	client *OpenAIClient,

	systemPrompt string,
	history *[]Message,
	tools []Tool,
	handlers map[string]ToolHandler,
	userMessage string,
) (string, error) {

	// Same behavior as Python:
	//
	// history.append({
	//     "role": "user",
	//     "content": message
	// })

	*history = append(
		*history,
		Message{
			Role:    "user",
			Content: userMessage,
		},
	)

	// Build local request history.
	//
	// Important:
	// Tool messages are NOT persisted into chatHistory/websiteHistory.
	// This matches your Python implementation.

	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	messages = append(
		messages,
		(*history)...,
	)

	for {

		response, err := client.ChatCompletion(
			ctx,
			messages,
			tools,
		)

		if err != nil {
			return "", err
		}

		if len(response.Choices) == 0 {
			return "", errors.New(
				"OpenAI returned no choices",
			)
		}

		reply := response.Choices[0].Message

		// No tool calls -> final response.
		if len(reply.ToolCalls) == 0 {

			*history = append(
				*history,
				Message{
					Role:    "assistant",
					Content: reply.Content,
				},
			)

			return reply.Content, nil
		}

		// Add assistant message containing tool calls.
		messages = append(
			messages,
			reply,
		)

		// Execute every tool call returned by the model.
		for _, call := range reply.ToolCalls {

			handler, exists := handlers[call.Function.Name]

			if !exists {
				return "", fmt.Errorf(
					"unknown tool: %s",
					call.Function.Name,
				)
			}

			result, err := handler(
				ctx,
				json.RawMessage(call.Function.Arguments),
			)

			if err != nil {

				result = fmt.Sprintf(
					"Tool execution failed: %s",
					err.Error(),
				)
			}

			// Add tool result to current agent turn.
			messages = append(
				messages,
				Message{
					Role:       "tool",
					ToolCallID: call.ID,
					Content:    result,
				},
			)
		}
	}
}

// ------------------------------------------------------------
// Chat agent
// ------------------------------------------------------------

func chat(
	ctx context.Context,
	client *OpenAIClient,
	message string,
) (string, error) {

	return complete(
		ctx,
		client,
		chatSystemPrompt,
		&chatHistory,
		chatTools,
		map[string]ToolHandler{

			"calculate": func(
				_ context.Context,
				raw json.RawMessage,
			) (string, error) {

				var args struct {
					Operation string  `json:"operation"`
					A         float64 `json:"a"`
					B         float64 `json:"b"`
				}

				if err := json.Unmarshal(
					raw,
					&args,
				); err != nil {
					return "", err
				}

				result, err := calculate(
					args.Operation,
					args.A,
					args.B,
				)

				if err != nil {
					return "", err
				}

				return fmt.Sprintf(
					"%v",
					result,
				), nil
			},

			"currentWeather": func(
				ctx context.Context,
				raw json.RawMessage,
			) (string, error) {

				var args struct {
					City string `json:"city"`
				}

				if err := json.Unmarshal(
					raw,
					&args,
				); err != nil {
					return "", err
				}

				return currentWeather(
					ctx,
					args.City,
				)
			},

			"getExchangeRate": func(
				ctx context.Context,
				raw json.RawMessage,
			) (string, error) {

				var args struct {
					From string `json:"from"`
					To   string `json:"to"`
				}

				if err := json.Unmarshal(
					raw,
					&args,
				); err != nil {
					return "", err
				}

				return getExchangeRate(
					ctx,
					args.From,
					args.To,
				)
			},
		},
		message,
	)
}

// ------------------------------------------------------------
// Website generation agent
// ------------------------------------------------------------

func generateWebsite(
	ctx context.Context,
	client *OpenAIClient,
	workspace *WebsiteWorkspace,
	message string,
) (string, error) {

	if err := os.MkdirAll(
		workspace.Root,
		0755,
	); err != nil {
		return "", fmt.Errorf(
			"create website workspace: %w",
			err,
		)
	}

	return complete(
		ctx,
		client,
		websiteSystemPrompt,
		&websiteHistory,
		websiteTools,
		map[string]ToolHandler{

			"createDirectory": func(
				_ context.Context,
				raw json.RawMessage,
			) (string, error) {

				var args struct {
					Path string `json:"path"`
				}

				if err := json.Unmarshal(
					raw,
					&args,
				); err != nil {
					return "", err
				}

				return workspace.createDirectory(
					args.Path,
				)
			},

			"writeFile": func(
				_ context.Context,
				raw json.RawMessage,
			) (string, error) {

				var args struct {
					Path    string `json:"path"`
					Content string `json:"content"`
				}

				if err := json.Unmarshal(
					raw,
					&args,
				); err != nil {
					return "", err
				}

				return workspace.writeFile(
					args.Path,
					args.Content,
				)
			},

			"readFile": func(
				_ context.Context,
				raw json.RawMessage,
			) (string, error) {

				var args struct {
					Path string `json:"path"`
				}

				if err := json.Unmarshal(
					raw,
					&args,
				); err != nil {
					return "", err
				}

				return workspace.readFile(
					args.Path,
				)
			},

			"listFiles": func(
				_ context.Context,
				raw json.RawMessage,
			) (string, error) {

				var args struct {
					Path string `json:"path"`
				}

				if err := json.Unmarshal(
					raw,
					&args,
				); err != nil {
					return "", err
				}

				return workspace.listFiles(
					args.Path,
				)
			},
		},
		message,
	)
}

// ------------------------------------------------------------
// Example main
// ------------------------------------------------------------

func main() {

	ctx := context.Background()

	client := NewOpenAIClient()

	workspace, err := NewWebsiteWorkspace()
	if err != nil {
		panic(err)
	}

	// Example 1:
	answer, err := chat(
		ctx,
		client,
		"What is 25 * 8?",
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("AI:", answer)

	// Example 2:
	answer, err = chat(
		ctx,
		client,
		"What is the weather in Mumbai?",
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("AI:", answer)

	// Example 3:
	answer, err = chat(
		ctx,
		client,
		"Convert USD to INR.",
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("AI:", answer)

	// Example 4:
	answer, err = generateWebsite(
		ctx,
		client,
		workspace,
		"Create a modern coffee shop website called BrewLab.",
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("Website Agent:", answer)
}

