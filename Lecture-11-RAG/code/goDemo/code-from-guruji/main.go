package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openai/openai-go"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"github.com/ledongthuc/pdf"
)

const (
	EmbeddingModel      = "text-embedding-3-small"
	EmbeddingDimensions = 1024
	ChatModel           = "gpt-5-mini"
	ChunkSize           = 300
	TopK                = 4
	BatchSize            = 100
)

var (
	KnowledgeDir = filepath.Join(
		"spring-code",
		"src",
		"main",
		"resources",
		"knowledge",
	)

	IndexName = getEnv(
		"PINECONE_INDEX_NAME",
		"shop-support",
	)

	Namespace = getEnv(
		"PINECONE_NAMESPACE",
		"policies",
	)
)

type Chunk struct {
	Text     string
	Source   string
	ChunkNum int
}

func getEnv(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	return value
}

func requireKey(name string, fallbackName string) string {
	value := os.Getenv(name)

	if value == "" {
		value = os.Getenv(fallbackName)
	}

	if value == "" {
		log.Fatalf(
			"Set %s (or %s) before running the program.",
			name,
			fallbackName,
		)
	}

	return value
}

// --------------------------------------------------
// PDF READING
// --------------------------------------------------

func readPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)

	if err != nil {
		return "", err
	}

	defer f.Close()

	var text strings.Builder

	totalPages := r.NumPage()

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := r.Page(pageNum)

		if page.V.IsNull() {
			continue
		}

		pageText, err := page.GetPlainText(nil)

		if err != nil {
			return "", err
		}

		text.WriteString(pageText)
		text.WriteString("\n")
	}

	return text.String(), nil
}

// --------------------------------------------------
// TOKEN CHUNKING
// --------------------------------------------------

// IMPORTANT:
// Python uses tiktoken's cl100k_base tokenizer.
// Go does not have a direct standard-library equivalent.
//
// This implementation uses whitespace-based chunking as a
// simple replacement.
//
// For exact token parity with Python, use a Go BPE/tiktoken
// implementation.
func splitIntoChunks(text string, chunkSize int) []string {
	words := strings.Fields(text)

	var chunks []string

	for start := 0; start < len(words); start += chunkSize {
		end := start + chunkSize

		if end > len(words) {
			end = len(words)
		}

		chunk := strings.TrimSpace(
			strings.Join(words[start:end], " "),
		)

		if chunk != "" {
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// --------------------------------------------------
// KNOWLEDGE BASE
// --------------------------------------------------

func readKnowledgeBase() ([]Chunk, error) {
	files, err := filepath.Glob(
		filepath.Join(KnowledgeDir, "*.pdf"),
	)

	if err != nil {
		return nil, err
	}

	sort.Strings(files)

	var chunks []Chunk

	for _, pdfPath := range files {

		text, err := readPDF(pdfPath)

		if err != nil {
			return nil, fmt.Errorf(
				"failed reading %s: %w",
				pdfPath,
				err,
			)
		}

		fileName := filepath.Base(pdfPath)

		pdfChunks := splitIntoChunks(
			text,
			ChunkSize,
		)

		for chunkNumber, chunkText := range pdfChunks {
			chunks = append(chunks, Chunk{
				Text:     chunkText,
				Source:   fileName,
				ChunkNum: chunkNumber,
			})
		}
	}

	return chunks, nil
}

// --------------------------------------------------
// STABLE ID
// --------------------------------------------------

func stableID(chunk Chunk) string {
	rawID := fmt.Sprintf(
		"%s:%d:%s",
		chunk.Source,
		chunk.ChunkNum,
		chunk.Text,
	)

	hash := sha256.Sum256(
		[]byte(rawID),
	)

	return hex.EncodeToString(hash[:])
}

// --------------------------------------------------
// OPENAI EMBEDDING
// --------------------------------------------------

func createEmbeddings(
	ctx context.Context,
	client *openai.Client,
	texts []string,
) ([][]float64, error) {

	response, err := client.Embeddings.New(
		ctx,
		openai.EmbeddingNewParams{
			Model: openai.EmbeddingModel(EmbeddingModel),
			Input: openai.EmbeddingNewParamsInputUnion{
				OfArrayOfStrings: texts,
			},
			Dimensions: openai.Int(int64(EmbeddingDimensions)),
		},
	)

	if err != nil {
		return nil, err
	}

	embeddings := make(
		[][]float64,
		len(response.Data),
	)

	for i, embedding := range response.Data {
		embeddings[i] = embedding.Embedding
	}

	return embeddings, nil
}

// --------------------------------------------------
// LOAD KNOWLEDGE BASE
// --------------------------------------------------

func loadKnowledgeBase(
	ctx context.Context,
	openaiClient *openai.Client,
	pineconeIndex *pinecone.IndexConnection,
) (int, error) {

	chunks, err := readKnowledgeBase()

	if err != nil {
		return 0, err
	}

	for start := 0; start < len(chunks); start += BatchSize {

		end := start + BatchSize

		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[start:end]

		texts := make([]string, len(batch))

		for i, chunk := range batch {
			texts[i] = chunk.Text
		}

		embeddings, err := createEmbeddings(
			ctx,
			openaiClient,
			texts,
		)

		if err != nil {
			return 0, fmt.Errorf(
				"embedding failed: %w",
				err,
			)
		}

		vectors := make(
			[]pinecone.Vector,
			len(batch),
		)

		for i, chunk := range batch {

			metadata := map[string]interface{}{
				"source": chunk.Source,
				"chunk":  chunk.ChunkNum,
				"text":   chunk.Text,
			}

			vectors[i] = pinecone.Vector{
				Id:       stableID(chunk),
				Values:   embeddings[i],
				Metadata: metadata,
			}
		}

		_, err = pineconeIndex.Upsert(
			ctx,
			&pinecone.UpsertRequest{
				Vectors:  vectors,
				Namespace: Namespace,
			},
		)

		if err != nil {
			return 0, fmt.Errorf(
				"pinecone upsert failed: %w",
				err,
			)
		}

		fmt.Printf(
			"Uploaded %d/%d chunks\n",
			end,
			len(chunks),
		)
	}

	return len(chunks), nil
}

// --------------------------------------------------
// QUERY PINECONE
// --------------------------------------------------

func searchKnowledgeBase(
	ctx context.Context,
	openaiClient *openai.Client,
	pineconeIndex *pinecone.IndexConnection,
	question string,
) (string, error) {

	embeddings, err := createEmbeddings(
		ctx,
		openaiClient,
		[]string{question},
	)

	if err != nil {
		return "", err
	}

	queryVector := embeddings[0]

	result, err := pineconeIndex.Query(
		ctx,
		&pinecone.QueryRequest{
			Vector:          queryVector,
			TopK:            TopK,
			IncludeMetadata: true,
			Namespace:       Namespace,
		},
	)

	if err != nil {
		return "", err
	}

	var contextParts []string

	for _, match := range result.Matches {

		if match.Metadata == nil {
			continue
		}

		textValue, ok := match.Metadata["text"]

		if !ok {
			continue
		}

		text, ok := textValue.(string)

		if !ok || text == "" {
			continue
		}

		contextParts = append(
			contextParts,
			text,
		)
	}

	return strings.Join(
		contextParts,
		"\n\n",
	), nil
}

// --------------------------------------------------
// ANSWER USER QUERY
// --------------------------------------------------

func answerUserQuery(
	ctx context.Context,
	openaiClient *openai.Client,
	pineconeIndex *pinecone.IndexConnection,
	question string,
) (string, error) {

	contextText, err := searchKnowledgeBase(
		ctx,
		openaiClient,
		pineconeIndex,
		question,
	)

	if err != nil {
		return "", err
	}

	instructions := fmt.Sprintf(`
You are an AI customer support assistant for our e-commerce company.

Answer the customer using ONLY the company information provided below.

If the answer is not available in the provided information, say:

"I don't have that information in the company documents."

COMPANY INFORMATION

%s
`, contextText)

	response, err := openaiClient.Responses.New(
		ctx,
		openai.ResponseNewParams{
			Model:       openai.ChatModel(ChatModel),
			Instructions: openai.String(instructions),
			Input: openai.ResponseNewParamsInputUnion{
				OfString: openai.String(question),
			},
		},
	)

	if err != nil {
		return "", err
	}

	return response.OutputText(), nil
}

// --------------------------------------------------
// MAIN
// --------------------------------------------------

func main() {

	ctx := context.Background()

	openAIKey := requireKey(
		"OPENAI_API_KEY",
		"OWN_KEY",
	)

	pineconeKey := requireKey(
		"PINECONE_API_KEY",
		"OWN_KEY",
	)

	// OpenAI
	openAIClient := openai.NewClient(
		openai.WithAPIKey(openAIKey),
	)

	// Pinecone
	pineconeClient, err := pinecone.NewClient(
		pinecone.WithApiKey(pineconeKey),
	)

	if err != nil {
		log.Fatal(err)
	}

	// Connect to index
	index, err := pineconeClient.Index(
		ctx,
		IndexName,
	)

	if err != nil {
		log.Fatal(err)
	}

	// Question from command line
	question := "What is the return policy?"

	if len(os.Args) > 1 {
		question = strings.Join(
			os.Args[1:],
			" ",
		)
	}

	// Load PDFs into Pinecone
	chunkCount, err := loadKnowledgeBase(
		ctx,
		&openAIClient,
		index,
	)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"Loaded %d knowledge chunks.\n",
		chunkCount,
	)

	// Ask question
	answer, err := answerUserQuery(
		ctx,
		&openAIClient,
		index,
		question,
	)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nAnswer:")
	fmt.Println(answer)
}