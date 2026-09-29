// Chat: user question -> rewrite into standalone query (OpenAI chat model)
//                     -> embed query -> Pinecone top-K
//                     -> answer from retrieved context only (OpenAI chat model)

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/pinecone-io/go-pinecone/v6/pinecone"
)

const (
	chatModel = "gpt-5.4-mini" // cheap + fast; swap for a bigger model if answers need it
	topK      = 10
)

const rewritePrompt = `You are a query rewriting expert. Based on the provided chat history, rephrase the "Follow Up user Question" into a complete, standalone question that can be understood without the chat history.
Only output the rewritten question and nothing else.`

const answerPromptTemplate = `You have to behave like a Data Structure and Algorithm Expert.
You will be given a context of relevant information and a user question.
Your task is to answer the user's question based ONLY on the provided context.
If the answer is not in the context, you must say "I could not find the answer in the provided document."
Keep your answers clear, concise, and educational.

Context: %s`

// RAGChat holds the clients and the conversation history.
// History holds only user/assistant turns; the system prompt is added fresh
// on every call because it differs per call (rewrite vs. answer, and the
// answer prompt contains this turn's retrieved context).
type RAGChat struct {
	ai      openai.Client
	index   *pinecone.IndexConnection
	history []openai.ChatCompletionMessageParamUnion
}

func NewRAGChat(ctx context.Context) (*RAGChat, error) {
	idx, err := connectIndex(ctx)
	if err != nil {
		return nil, err
	}
	return &RAGChat{ai: newOpenAIClient(), index: idx}, nil
}

func (c *RAGChat) Close() { c.index.Close() }

// complete runs one chat completion: system prompt + history + one new user
// message. It never modifies c.history.
func (c *RAGChat) complete(ctx context.Context, system, user string) (string, error) {
	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(c.history)+2)
	msgs = append(msgs, openai.SystemMessage(system))
	msgs = append(msgs, c.history...)
	msgs = append(msgs, openai.UserMessage(user))

	resp, err := c.ai.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(chatModel),
		Messages: msgs,
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("empty response from model")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// transformQuery rewrites a follow-up question into a standalone one.
func (c *RAGChat) transformQuery(ctx context.Context, question string) (string, error) {
	rewritten, err := c.complete(ctx, rewritePrompt, question)
	if err != nil {
		return "", fmt.Errorf("rewrite query: %w", err)
	}
	if rewritten == "" {
		return question, nil // fall back to the original question
	}
	return rewritten, nil
}

func (c *RAGChat) Chat(ctx context.Context, question string) (string, error) {
	query, err := c.transformQuery(ctx, question)
	if err != nil {
		return "", err
	}

	// Same embedTexts as the indexer -> same model and dimensions.
	vecs, err := embedTexts(ctx, c.ai, []string{query})
	if err != nil {
		return "", fmt.Errorf("embed query: %w", err)
	}

	results, err := c.index.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:          vecs[0],
		TopK:            topK,
		IncludeMetadata: true,
	})
	if err != nil {
		return "", fmt.Errorf("pinecone query: %w", err)
	}

	fmt.Println("\n--- Raw Pinecone matches (JSON) ---")
	if raw, err := json.MarshalIndent(results, "", "  "); err == nil {
		fmt.Println(string(raw))
	}

	// Nil-safe proto getters: missing metadata or "text" just yields "".
	var parts []string
	for _, m := range results.Matches {
		if m == nil || m.Vector == nil {
			continue
		}
		if text := m.Vector.Metadata.GetFields()["text"].GetStringValue(); text != "" {
			parts = append(parts, text)
		}
	}
	contextText := strings.Join(parts, "\n\n---\n\n")

	fmt.Println("\n--- Retrieved context (readable) ---")
	if contextText == "" {
		fmt.Println("(no matches found)")
	} else {
		fmt.Println(contextText)
	}
	fmt.Println("\n------------------------------------")

	// Generate the answer; commit to history only on success.
	answer, err := c.complete(ctx, fmt.Sprintf(answerPromptTemplate, contextText), query)
	if err != nil {
		return "", fmt.Errorf("generate answer: %w", err)
	}

	c.history = append(c.history,
		openai.UserMessage(query),
		openai.AssistantMessage(answer),
	)

	fmt.Printf("\n\n%s\n", answer)
	return contextText, nil
}

// ==========================================================
// chat command
// ==========================================================

func runChat(ctx context.Context) error {
	chat, err := NewRAGChat(ctx)
	if err != nil {
		return err
	}
	defer chat.Close()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Ask me anything--> ")
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) && line == "" {
			fmt.Println()
			return nil // Ctrl+D exits cleanly
		}
		question := strings.TrimSpace(line)
		if question == "" {
			continue
		}

		if _, err := chat.Chat(ctx, question); err != nil {
			fmt.Println("⚠️  Something went wrong:", err)
		}
	}
}