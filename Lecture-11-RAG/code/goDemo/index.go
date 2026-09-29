// Indexing: PDF -> pages -> recursive chunks -> OpenAI embeddings -> Pinecone
//
// Vectors are stored in the same shape LangChain's PineconeVectorStore uses
// (chunk text under the "text" metadata key, plus source/page), so a
// LangChain retriever on the same index keeps working.

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
	"github.com/pinecone-io/go-pinecone/v6/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	pdfPath      = "./Dsa.pdf"
	chunkSize    = 1000
	chunkOverlap = 200
	batchSize    = 100 // 100 chunks ≈ 25k tokens, well under OpenAI's per-request limit
)

// Document mirrors LangChain's Document: text plus metadata.
type Document struct {
	PageContent string
	Metadata    map[string]any
}

// ==========================================================
// Step 1 : Read PDF (one Document per page, like PyPDFLoader)
// ==========================================================

func loadPDF(path string) ([]Document, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	var docs []Document
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return nil, fmt.Errorf("read page %d: %w", i, err)
		}
		docs = append(docs, Document{
			PageContent: text,
			Metadata: map[string]any{
				"source": path,
				"page":   i - 1, // PyPDFLoader uses 0-indexed pages
			},
		})
	}
	return docs, nil
}

// ==========================================================
// Step 2 : Recursive Character Text Splitter
// ==========================================================
//
// Port of LangChain's RecursiveCharacterTextSplitter:
//   - try separators in order ("\n\n", "\n", " ", "")
//   - split on the first one present, keeping the separator at the start
//     of each following piece
//   - pieces still larger than chunkSize are split recursively
//   - small pieces are merged back into chunks of <= chunkSize with
//     chunkOverlap characters of overlap
// Lengths are counted in characters (runes), like Python's len().

type RecursiveSplitter struct {
	ChunkSize    int
	ChunkOverlap int
	Separators   []string
}

func NewRecursiveSplitter(size, overlap int) *RecursiveSplitter {
	return &RecursiveSplitter{
		ChunkSize:    size,
		ChunkOverlap: overlap,
		Separators:   []string{"\n\n", "\n", " ", ""},
	}
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func (s *RecursiveSplitter) SplitDocuments(docs []Document) []Document {
	var out []Document
	for _, d := range docs {
		for _, chunk := range s.splitText(d.PageContent, s.Separators) {
			meta := make(map[string]any, len(d.Metadata))
			for k, v := range d.Metadata {
				meta[k] = v
			}
			out = append(out, Document{PageContent: chunk, Metadata: meta})
		}
	}
	return out
}

func (s *RecursiveSplitter) splitText(text string, separators []string) []string {
	separator := separators[len(separators)-1]
	var remaining []string
	for i, sep := range separators {
		if sep == "" {
			separator = ""
			break
		}
		if strings.Contains(text, sep) {
			separator = sep
			remaining = separators[i+1:]
			break
		}
	}

	var splits []string
	if separator == "" {
		for _, r := range text {
			splits = append(splits, string(r))
		}
	} else {
		parts := strings.Split(text, separator)
		for i, p := range parts {
			if i > 0 {
				p = separator + p
			}
			if p != "" {
				splits = append(splits, p)
			}
		}
	}

	var final, good []string
	for _, piece := range splits {
		if runeLen(piece) < s.ChunkSize {
			good = append(good, piece)
			continue
		}
		if len(good) > 0 {
			final = append(final, s.mergeSplits(good, "")...)
			good = nil
		}
		if len(remaining) == 0 {
			final = append(final, piece)
		} else {
			final = append(final, s.splitText(piece, remaining)...)
		}
	}
	if len(good) > 0 {
		final = append(final, s.mergeSplits(good, "")...)
	}
	return final
}

func (s *RecursiveSplitter) mergeSplits(splits []string, separator string) []string {
	sepLen := runeLen(separator)
	var docs, current []string
	total := 0

	joinCurrent := func() {
		doc := strings.TrimSpace(strings.Join(current, separator))
		if doc != "" {
			docs = append(docs, doc)
		}
	}
	extraSep := func() int {
		if len(current) > 0 {
			return sepLen
		}
		return 0
	}

	for _, d := range splits {
		l := runeLen(d)
		if total+l+extraSep() > s.ChunkSize {
			if len(current) > 0 {
				joinCurrent()
				for total > s.ChunkOverlap ||
					(total+l+extraSep() > s.ChunkSize && total > 0) {
					drop := runeLen(current[0])
					if len(current) > 1 {
						drop += sepLen
					}
					total -= drop
					current = current[1:]
				}
			}
		}
		current = append(current, d)
		total += l
		if len(current) > 1 {
			total += sepLen
		}
	}
	joinCurrent()
	return docs
}

// ==========================================================
// Step 3 : Upsert one batch into Pinecone
// ==========================================================

func upsertBatch(ctx context.Context, idx *pinecone.IndexConnection, batch []Document, vecs [][]float32) error {
	vectors := make([]*pinecone.Vector, len(batch))
	for i, doc := range batch {
		md := map[string]any{"text": doc.PageContent} // LangChain's default text_key
		for k, v := range doc.Metadata {
			md[k] = v
		}
		meta, err := structpb.NewStruct(md)
		if err != nil {
			return fmt.Errorf("build metadata: %w", err)
		}
		values := vecs[i]
		vectors[i] = &pinecone.Vector{
			Id:       uuid.NewString(),
			Values:   &values,
			Metadata: meta,
		}
	}
	_, err := idx.UpsertVectors(ctx, vectors)
	return err
}

// ==========================================================
// index command
// ==========================================================

func runIndex(ctx context.Context) error {
	// Read PDF
	rawDocs, err := loadPDF(pdfPath)
	if err != nil {
		return err
	}
	fmt.Println("✅ PDF Loaded")
	fmt.Printf("Pages Found : %d\n", len(rawDocs))

	// Chunk
	chunkedDocs := NewRecursiveSplitter(chunkSize, chunkOverlap).SplitDocuments(rawDocs)
	if len(chunkedDocs) == 0 {
		return errors.New("no text extracted from PDF (is it scanned/image-only?)")
	}
	fmt.Println("✅ Chunking Completed")
	fmt.Printf("Chunks Created : %d\n\n", len(chunkedDocs))
	fmt.Println("First Chunk")
	fmt.Println("----------------------------------------")
	preview := []rune(chunkedDocs[0].PageContent)
	if len(preview) > 300 {
		preview = preview[:300]
	}
	fmt.Println(string(preview))
	fmt.Println("----------------------------------------")

	// Clients
	aiClient := newOpenAIClient()
	fmt.Printf("✅ Embedding Model Configured (%s, %d-dim)\n", embeddingModel, embeddingDim)

	idxConn, err := connectIndex(ctx)
	if err != nil {
		return err
	}
	defer idxConn.Close()
	fmt.Println("✅ Pinecone Connected")

	stats, err := idxConn.DescribeIndexStats(ctx)
	if err != nil {
		return fmt.Errorf("index stats: %w", err)
	}
	fmt.Println()
	fmt.Println("Index Statistics")
	fmt.Printf("%+v\n", stats)

	// Embed + store in batches
	for i := 0; i < len(chunkedDocs); i += batchSize {
		end := min(i+batchSize, len(chunkedDocs))
		batch := chunkedDocs[i:end]

		texts := make([]string, len(batch))
		for j, d := range batch {
			texts[j] = d.PageContent
		}

		vecs, err := embedTexts(ctx, aiClient, texts)
		if err != nil {
			return fmt.Errorf("embed batch starting at %d: %w", i, err)
		}
		if err := upsertBatch(ctx, idxConn, batch, vecs); err != nil {
			return fmt.Errorf("upsert batch starting at %d: %w", i, err)
		}
		fmt.Printf("Uploaded %d documents\n", end)
	}

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("🎉 Successfully Indexed Documents")
	fmt.Printf("Stored %d Chunks\n", len(chunkedDocs))
	fmt.Println("===================================")
	return nil
}