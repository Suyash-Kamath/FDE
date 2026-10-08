// Indexing: knowledge/*.pdf -> pages -> recursive chunks -> category metadata
//           -> OpenAI embeddings -> Pinecone  (+ local chunk store for BM25)
//
// Every chunk is stored twice, under the SAME ID:
//   1. Pinecone   : dense vector + metadata (text, source, page, category)
//                   -> semantic search, filterable by category
//   2. chunks.json: chunk text + the same metadata
//                   -> BM25 keyword search at query time (exact terms like
//                      "AVL", "Dijkstra", "heapify" that embeddings can miss)
//
// Category comes from the PDF file name: knowledge/graphs.pdf -> "graphs".
//
// Metadata keeps LangChain's PineconeVectorStore shape (chunk text under
// "text"), so a LangChain retriever on the same index keeps working.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	"github.com/pinecone-io/go-pinecone/v6/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	knowledgeGlob  = "./knowledge/*.pdf" // file name (without .pdf) becomes the category
	chunkStorePath = "./chunks.json"     // local copy of every chunk, used for BM25
	chunkSize      = 1000
	chunkOverlap   = 200
	batchSize      = 100 // 100 chunks ≈ 25k tokens, well under OpenAI's per-request limit
)

// Document mirrors LangChain's Document: text plus metadata, plus a stable ID.
type Document struct {
	ID          string
	PageContent string
	Metadata    map[string]any
}

// ChunkRecord is one chunk as persisted in chunks.json.
type ChunkRecord struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Source   string `json:"source"`
	Page     int    `json:"page"`
	Category string `json:"category"`
}

// ==========================================================
// Step 1 : Read PDFs (one Document per page, like PyPDFLoader)
// ==========================================================

// categoryFromPath turns "./knowledge/Dynamic Programming.pdf" into "dynamic_programming".
func categoryFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(base) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
		} else if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	cat := strings.Trim(b.String(), "_")
	if cat == "" {
		cat = "general"
	}
	return cat
}

func loadPDF(path string) ([]Document, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf %s: %w", path, err)
	}
	defer f.Close()

	category := categoryFromPath(path)

	var docs []Document
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return nil, fmt.Errorf("read %s page %d: %w", path, i, err)
		}
		docs = append(docs, Document{
			PageContent: text,
			Metadata: map[string]any{
				"source":   path,
				"page":     i - 1, // PyPDFLoader uses 0-indexed pages
				"category": category,
			},
		})
	}
	return docs, nil
}

func loadKnowledgeBase(pattern string) ([]Document, error) {
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("bad glob %q: %w", pattern, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no PDFs match %q (put your PDFs in ./knowledge; the file name becomes the category)", pattern)
	}
	sort.Strings(paths)

	var all []Document
	for _, p := range paths {
		docs, err := loadPDF(p)
		if err != nil {
			return nil, err
		}
		fmt.Printf("  📄 %-35s category=%-20s pages=%d\n", filepath.Base(p), categoryFromPath(p), len(docs))
		all = append(all, docs...)
	}
	return all, nil
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

// assignChunkIDs gives every chunk a deterministic ID like "graphs-p12-c3".
// Deterministic IDs mean re-indexing overwrites vectors instead of
// duplicating them, and Pinecone + chunks.json always agree on IDs
// (needed to fuse semantic and keyword results).
func assignChunkIDs(docs []Document) {
	counters := map[string]int{}
	for i := range docs {
		key := fmt.Sprintf("%s-p%d", docs[i].Metadata["category"], docs[i].Metadata["page"])
		docs[i].ID = fmt.Sprintf("%s-c%d", key, counters[key])
		counters[key]++
	}
}

// ==========================================================
// Step 3 : Local chunk store (for BM25 keyword search)
// ==========================================================

func saveChunkStore(path string, docs []Document) error {
	records := make([]ChunkRecord, len(docs))
	for i, d := range docs {
		source, _ := d.Metadata["source"].(string)
		page, _ := d.Metadata["page"].(int)
		category, _ := d.Metadata["category"].(string)
		records[i] = ChunkRecord{
			ID:       d.ID,
			Text:     d.PageContent,
			Source:   source,
			Page:     page,
			Category: category,
		}
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("encode chunk store: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write chunk store: %w", err)
	}
	return os.Rename(tmp, path)
}

func loadChunkStore(path string) ([]ChunkRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s not found, run the index command first", path)
		}
		return nil, fmt.Errorf("read chunk store: %w", err)
	}
	var records []ChunkRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("decode chunk store: %w", err)
	}
	return records, nil
}

// categoriesOf returns the sorted, unique categories present in the store.
func categoriesOf(records []ChunkRecord) []string {
	seen := map[string]bool{}
	var cats []string
	for _, r := range records {
		if r.Category != "" && !seen[r.Category] {
			seen[r.Category] = true
			cats = append(cats, r.Category)
		}
	}
	sort.Strings(cats)
	return cats
}

// ==========================================================
// Step 4 : Upsert one batch into Pinecone
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
			Id:       doc.ID,
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
	// Read PDFs
	fmt.Println("Loading knowledge base...")
	rawDocs, err := loadKnowledgeBase(knowledgeGlob)
	if err != nil {
		return err
	}
	fmt.Println("✅ PDFs Loaded")
	fmt.Printf("Pages Found : %d\n", len(rawDocs))

	// Chunk
	chunkedDocs := NewRecursiveSplitter(chunkSize, chunkOverlap).SplitDocuments(rawDocs)
	if len(chunkedDocs) == 0 {
		return errors.New("no text extracted from PDFs (are they scanned/image-only?)")
	}
	assignChunkIDs(chunkedDocs)

	fmt.Println("✅ Chunking Completed")
	fmt.Printf("Chunks Created : %d\n", len(chunkedDocs))

	perCategory := map[string]int{}
	for _, d := range chunkedDocs {
		cat, _ := d.Metadata["category"].(string)
		perCategory[cat]++
	}
	cats := make([]string, 0, len(perCategory))
	for c := range perCategory {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	for _, c := range cats {
		fmt.Printf("  • %-20s %d chunks\n", c, perCategory[c])
	}

	fmt.Println()
	fmt.Printf("First Chunk (id=%s)\n", chunkedDocs[0].ID)
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

	// Local copy for keyword (BM25) search
	if err := saveChunkStore(chunkStorePath, chunkedDocs); err != nil {
		return err
	}
	fmt.Printf("✅ Chunk store written to %s\n", chunkStorePath)

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("🎉 Successfully Indexed Documents")
	fmt.Printf("Stored %d Chunks across %d categories\n", len(chunkedDocs), len(cats))
	fmt.Println("===================================")
	return nil
}