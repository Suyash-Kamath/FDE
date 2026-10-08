// Chat pipeline (advanced RAG):
//
//   user picks a category (or "others" = no filter)
//   question -> rewrite into standalone query (chat history aware)
//            -> embed query
//            -> [semantic] Pinecone top-N, category filter, similarity threshold
//            -> [keyword]  BM25 top-N over chunks.json, same category filter
//            -> nothing from either?  -> canned fallback (LLM never called)
//            -> Reciprocal Rank Fusion (k = 60)
//            -> rerank the fused pool (LLM as reranker, query+chunk read together)
//            -> keep chunks above the rerank cut-off, top finalTopK
//            -> answer from that context only

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/pinecone-io/go-pinecone/v6/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	chatModel = "gpt-5.4-mini"

	// Retrieval
	candidatePoolK      = 20   // how many chunks EACH search fetches (wide net)
	similarityThreshold = 0.75 // min cosine score for a semantic hit (tune for your embedding model)
	rrfK                = 60   // standard RRF constant

	// Reranking
	enableRerank   = true // turn off for small/simple corpora to save a model call
	rerankMinScore = 0.5  // chunks the reranker scores below this are dropped
	rerankDocChars = 1500 // chunk text sent to the reranker is truncated to this

	// What finally reaches the answering LLM
	finalTopK = 4

	otherCategory = "others" // "others" = search across every category
)

const fallbackAnswer = "I could not find the relevant information about this query."

const rewritePrompt = `You are a query rewriting expert.
Based on the provided chat history, rephrase the follow-up user question
into a complete, standalone question that can be understood without chat history.
Only output the rewritten question and nothing else.`

const rerankPrompt = `You are a reranking model.
You will get a user query and a numbered list of documents.
For EACH document independently, judge how useful it is for answering the query
and give it a relevance score between 0 and 1:
  1.0 = directly answers the query
  0.5 = partially relevant / useful background
  0.0 = unrelated
Return ONLY a JSON object, no markdown and no other text, in exactly this form:
{"scores":[{"id":1,"score":0.92},{"id":2,"score":0.10}]}
Include every document id exactly once.`

const answerPromptTemplate = `You have to behave like a Data Structure and Algorithm Expert.
You will be given a context of relevant information and a user question.
Your task is to answer the user's question based ONLY on the provided context.
If the answer is not in the context, you must say:
"I could not find the relevant information about this query."
Keep your answers clear, concise, and educational.

Context:
%s`

// ==========================================================
// RAGChat
// ==========================================================

type RAGChat struct {
	ai         openai.Client
	index      *pinecone.IndexConnection
	bm25       *BM25Index
	categories []string
	category   string // currently selected category, or otherCategory
	history    []openai.ChatCompletionMessageParamUnion
}

func NewRAGChat(ctx context.Context) (*RAGChat, error) {
	records, err := loadChunkStore(chunkStorePath)
	if err != nil {
		return nil, err
	}

	idx, err := connectIndex(ctx)
	if err != nil {
		return nil, err
	}

	return &RAGChat{
		ai:         newOpenAIClient(),
		index:      idx,
		bm25:       NewBM25Index(records),
		categories: categoriesOf(records),
		category:   otherCategory,
	}, nil
}

func (c *RAGChat) Close() {
	c.index.Close()
}

func (c *RAGChat) SetCategory(category string) {
	c.category = category
}

// filterCategory returns "" when no filter should be applied.
func (c *RAGChat) filterCategory() string {
	if c.category == otherCategory {
		return ""
	}
	return c.category
}

func (c *RAGChat) remember(question, answer string) {
	c.history = append(
		c.history,
		openai.UserMessage(question),
		openai.AssistantMessage(answer),
	)
}

// complete calls the chat model. withHistory=false is used for the reranker,
// which must judge relevance without being swayed by the conversation.
func (c *RAGChat) complete(
	ctx context.Context,
	system string,
	user string,
	withHistory bool,
) (string, error) {

	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(c.history)+2)
	msgs = append(msgs, openai.SystemMessage(system))
	if withHistory {
		msgs = append(msgs, c.history...)
	}
	msgs = append(msgs, openai.UserMessage(user))

	resp, err := c.ai.Chat.Completions.New(
		ctx,
		openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(chatModel),
			Messages: msgs,
		},
	)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("empty response from model")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func (c *RAGChat) transformQuery(ctx context.Context, question string) (string, error) {
	rewritten, err := c.complete(ctx, rewritePrompt, question, true)
	if err != nil {
		return "", fmt.Errorf("rewrite query: %w", err)
	}
	if rewritten == "" {
		return question, nil
	}
	return rewritten, nil
}

// ==========================================================
// Semantic search (Pinecone + metadata filter + threshold)
// ==========================================================

func (c *RAGChat) semanticSearch(ctx context.Context, vec []float32) ([]Candidate, error) {
	req := &pinecone.QueryByVectorValuesRequest{
		Vector:          vec,
		TopK:            candidatePoolK,
		IncludeMetadata: true,
	}

	if cat := c.filterCategory(); cat != "" {
		// Equivalent to: category == "<selected>"
		filter, err := structpb.NewStruct(map[string]any{
			"category": map[string]any{"$eq": cat},
		})
		if err != nil {
			return nil, fmt.Errorf("create metadata filter: %w", err)
		}
		req.MetadataFilter = filter
	}

	results, err := c.index.QueryByVectorValues(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("pinecone query: %w", err)
	}

	var out []Candidate
	rejected := 0
	for _, m := range results.Matches {
		if m == nil || m.Vector == nil {
			continue
		}
		fields := m.Vector.Metadata.GetFields()
		text := fields["text"].GetStringValue()
		if strings.TrimSpace(text) == "" {
			continue
		}
		if m.Score < similarityThreshold {
			rejected++
			continue
		}
		out = append(out, Candidate{
			ID:            m.Vector.Id,
			Text:          text,
			Source:        fields["source"].GetStringValue(),
			Page:          int(fields["page"].GetNumberValue()),
			Category:      fields["category"].GetStringValue(),
			SemanticRank:  len(out) + 1,
			SemanticScore: m.Score,
			RerankScore:   -1,
		})
	}

	fmt.Printf("Semantic : %d accepted, %d below threshold %.2f\n", len(out), rejected, similarityThreshold)
	return out, nil
}

// ==========================================================
// Reranking (LLM as a cross-encoder style reranker)
// ==========================================================
//
// The embedding model embeds the query and each chunk SEPARATELY, then
// compares vectors. A reranker reads the query and the chunk TOGETHER and
// scores "can this chunk answer this query?", which is far more precise.
// It is slower, so it only runs on the small fused pool, never the whole index.

func (c *RAGChat) rerank(ctx context.Context, query string, cands []Candidate) ([]Candidate, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Query: %s\n\n", query)
	for i, cand := range cands {
		fmt.Fprintf(&b, "Document %d:\n%s\n\n", i+1, truncateRunes(cand.Text, rerankDocChars))
	}

	raw, err := c.complete(ctx, rerankPrompt, b.String(), false)
	if err != nil {
		return nil, fmt.Errorf("rerank call: %w", err)
	}

	// Be forgiving about stray text / markdown fences around the JSON.
	start, end := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("reranker returned no JSON: %q", raw)
	}

	var parsed struct {
		Scores []struct {
			ID    int     `json:"id"`
			Score float64 `json:"score"`
		} `json:"scores"`
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("parse rerank JSON: %w", err)
	}

	out := make([]Candidate, len(cands))
	copy(out, cands)
	for i := range out {
		out[i].RerankScore = 0 // anything the model skipped counts as irrelevant
	}
	for _, s := range parsed.Scores {
		if s.ID >= 1 && s.ID <= len(out) {
			out[s.ID-1].RerankScore = min(max(s.Score, 0), 1)
		}
	}

	sort.SliceStable(out, func(a, b int) bool { return out[a].RerankScore > out[b].RerankScore })
	return out, nil
}

// ==========================================================
// Chat
// ==========================================================

func (c *RAGChat) Chat(ctx context.Context, question string) (string, error) {

	// STEP 1: Rewrite question into a standalone query.
	query, err := c.transformQuery(ctx, question)
	if err != nil {
		return "", err
	}
	fmt.Println("\n--- Rewritten Query ---")
	fmt.Println(query)
	fmt.Printf("\n--- Retrieval (category: %s) ---\n", c.category)

	// STEP 2: Generate query embedding.
	vecs, err := embedTexts(ctx, c.ai, []string{query})
	if err != nil {
		return "", fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) == 0 {
		return "", errors.New("embedding returned no vectors")
	}

	// STEP 3: Semantic search (metadata filter + similarity threshold).
	semantic, err := c.semanticSearch(ctx, vecs[0])
	if err != nil {
		return "", err
	}

	// STEP 4: Keyword search (BM25, same category filter).
	keyword := c.bm25.Search(query, candidatePoolK, c.filterCategory())
	fmt.Printf("Keyword  : %d BM25 hits\n", len(keyword))

	// STEP 5: Nothing relevant anywhere -> canned answer, LLM is never called.
	if len(semantic) == 0 && len(keyword) == 0 {
		fmt.Println("\n⚠️ No relevant chunks found (threshold / category / keywords).")
		c.remember(question, fallbackAnswer)
		return fallbackAnswer, nil
	}

	// STEP 6: Merge both rankings with Reciprocal Rank Fusion.
	fused := fuseRRF(rrfK, semantic, keyword)
	if len(fused) > candidatePoolK {
		fused = fused[:candidatePoolK]
	}
	printCandidates("Fused with RRF (k=60)", fused, false)

	// STEP 7: Rerank the fused pool.
	final := fused
	if enableRerank {
		reranked, err := c.rerank(ctx, query, fused)
		if err != nil {
			// Reranking is an improvement, not a requirement: keep RRF order.
			fmt.Println("⚠️ Rerank failed, falling back to RRF order:", err)
		} else {
			printCandidates("Reranked", reranked, true)
			final = final[:0:0]
			for _, cand := range reranked {
				if cand.RerankScore >= rerankMinScore {
					final = append(final, cand)
				}
			}
		}
	}

	// STEP 8: Keep the best few for the LLM.
	if len(final) > finalTopK {
		final = final[:finalTopK]
	}
	if len(final) == 0 {
		fmt.Printf("\n⚠️ Reranker judged every chunk irrelevant (< %.2f).\n", rerankMinScore)
		c.remember(question, fallbackAnswer)
		return fallbackAnswer, nil
	}

	parts := make([]string, len(final))
	for i, cand := range final {
		parts[i] = fmt.Sprintf("[Source: %s, page %d, category: %s]\n%s",
			cand.Source, cand.Page+1, cand.Category, cand.Text)
	}
	contextText := strings.Join(parts, "\n\n---\n\n")

	fmt.Printf("\n--- Context sent to LLM (%d chunks) ---\n", len(final))
	fmt.Println(contextText)
	fmt.Println("\n------------------------------------")

	// STEP 9: Generate answer from retrieved context.
	answer, err := c.complete(ctx, fmt.Sprintf(answerPromptTemplate, contextText), query, true)
	if err != nil {
		return "", fmt.Errorf("generate answer: %w", err)
	}

	// STEP 10: Save conversation history.
	c.remember(question, answer)
	return answer, nil
}

// ==========================================================
// Helpers
// ==========================================================

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func printCandidates(title string, cands []Candidate, showRerank bool) {
	fmt.Printf("\n--- %s ---\n", title)
	fmt.Printf("%-3s %-28s %-10s %-10s %-8s", "#", "chunk id", "semantic", "keyword", "rrf")
	if showRerank {
		fmt.Printf(" %-6s", "rerank")
	}
	fmt.Println()

	for i, cand := range cands {
		sem, kw := "-", "-"
		if cand.SemanticRank > 0 {
			sem = fmt.Sprintf("#%d %.2f", cand.SemanticRank, cand.SemanticScore)
		}
		if cand.KeywordRank > 0 {
			kw = fmt.Sprintf("#%d %.1f", cand.KeywordRank, cand.KeywordScore)
		}
		fmt.Printf("%-3d %-28s %-10s %-10s %.4f", i+1, truncateRunes(cand.ID, 27), sem, kw, cand.RRFScore)
		if showRerank {
			mark := "✅"
			if cand.RerankScore < rerankMinScore {
				mark = "❌"
			}
			fmt.Printf("  %.2f %s", cand.RerankScore, mark)
		}
		fmt.Println()
	}
}

// chooseCategory shows the category menu. Picking a category narrows both
// searches to that category; "others" searches everything.
func chooseCategory(reader *bufio.Reader, categories []string) (string, error) {
	fmt.Println("\nWhat is your question about?")
	for i, cat := range categories {
		fmt.Printf("  %d) %s\n", i+1, cat)
	}
	fmt.Printf("  %d) %s (search everything)\n", len(categories)+1, otherCategory)

	for {
		fmt.Print("Choose a number [Enter = others]--> ")
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		choice := strings.TrimSpace(line)

		if choice == "" || strings.EqualFold(choice, otherCategory) {
			return otherCategory, nil
		}
		if n, convErr := strconv.Atoi(choice); convErr == nil {
			if n >= 1 && n <= len(categories) {
				return categories[n-1], nil
			}
			if n == len(categories)+1 {
				return otherCategory, nil
			}
		}
		for _, cat := range categories {
			if strings.EqualFold(choice, cat) {
				return cat, nil
			}
		}
		if errors.Is(err, io.EOF) {
			return otherCategory, nil
		}
		fmt.Println("Invalid choice, try again.")
	}
}

// ==========================================================
// Chat Command
// ==========================================================

func runChat(ctx context.Context) error {

	chat, err := NewRAGChat(ctx)
	if err != nil {
		return err
	}
	defer chat.Close()

	reader := bufio.NewReader(os.Stdin)

	category, err := chooseCategory(reader, chat.categories)
	if err != nil {
		return err
	}
	chat.SetCategory(category)
	fmt.Printf("Category set to: %s\n", category)
	fmt.Println("Commands: /category to switch category, /exit to quit.")

	for {
		fmt.Printf("\n[%s] Ask me anything--> ", chat.category)

		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if errors.Is(err, io.EOF) && line == "" {
			fmt.Println()
			return nil
		}

		question := strings.TrimSpace(line)

		switch strings.ToLower(question) {
		case "":
			// nothing typed
		case "/exit", "/quit":
			return nil
		case "/category":
			category, catErr := chooseCategory(reader, chat.categories)
			if catErr != nil {
				return catErr
			}
			chat.SetCategory(category)
			fmt.Printf("Category set to: %s\n", category)
		default:
			answer, chatErr := chat.Chat(ctx, question)
			if chatErr != nil {
				fmt.Println("⚠️ Something went wrong:", chatErr)
			} else {
				fmt.Printf("\nAnswer:\n%s\n", answer)
			}
		}

		if errors.Is(err, io.EOF) {
			return nil
		}
	}
}