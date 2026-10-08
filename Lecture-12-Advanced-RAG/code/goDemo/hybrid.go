// Hybrid retrieval building blocks:
//   - Candidate : one chunk moving through the pipeline, carrying every score
//   - BM25      : in-memory keyword search over chunks.json
//   - RRF       : Reciprocal Rank Fusion, merges semantic + keyword rankings
//
// Why both searches? Semantic search matches meaning but can miss exact
// terms (a product code like "AX204", or "Kadane" vs "maximum subarray").
// Keyword search nails exact terms but knows nothing about meaning.
// Their scores live on different scales (cosine vs BM25), so we don't add
// scores; RRF only looks at each document's RANK in each list:
//
//     RRF(d) = Σ  1 / (k + rank_i(d))      k = 60
//
// A chunk ranked well by both searches beats one ranked #1 by only one.

package main

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Candidate struct {
	ID       string
	Text     string
	Source   string
	Page     int
	Category string

	SemanticRank  int     // 1-based rank in semantic results, 0 = not retrieved
	SemanticScore float32 // cosine similarity from Pinecone
	KeywordRank   int     // 1-based rank in BM25 results, 0 = not retrieved
	KeywordScore  float64 // raw BM25 score
	RRFScore      float64 // fused score
	RerankScore   float64 // 0..1 relevance from the reranker, -1 = not reranked
}

// ==========================================================
// Tokenizer
// ==========================================================

var stopwords = func() map[string]struct{} {
	words := strings.Fields(`a an the and or but if then else of to in on at by for with
		from into over under about as is are was were be been being am do does did
		doing have has had having i me my we our you your he she it its they them
		their this that these those what which who whom whose when where why how
		can could should would will shall may might must not no yes so than too
		very just also there here all any some each such only own same other more
		most up down out off again further once`)
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		m[w] = struct{}{}
	}
	return m
}()

func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if _, stop := stopwords[f]; stop {
			continue
		}
		// drop single letters ("a", "n" from "O(n)") but keep single digits
		if r, _ := utf8.DecodeRuneInString(f); utf8.RuneCountInString(f) < 2 && !unicode.IsDigit(r) {
			continue
		}
		out = append(out, stem(f))
	}
	return out
}

// stem is a deliberately tiny plural stemmer so "headphones" matches
// "headphone" and "supports" matches "support". Exact codes like "ax204"
// and words ending in "ss"/"us"/"is" ("class", "radius", "analysis") are untouched.
func stem(t string) string {
	if len(t) > 3 && strings.HasSuffix(t, "s") &&
		!strings.HasSuffix(t, "ss") && !strings.HasSuffix(t, "us") && !strings.HasSuffix(t, "is") {
		return t[:len(t)-1]
	}
	return t
}

func uniqueTokens(tokens []string) []string {
	seen := make(map[string]bool, len(tokens))
	out := tokens[:0:0]
	for _, t := range tokens {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// ==========================================================
// BM25 keyword search
// ==========================================================

const (
	bm25K1 = 1.5  // term-frequency saturation
	bm25B  = 0.75 // document-length normalisation
)

type BM25Index struct {
	docs   []ChunkRecord
	tf     []map[string]int // term counts per chunk
	docLen []int
	df     map[string]int // number of chunks containing each term
	avgDL  float64
}

func NewBM25Index(docs []ChunkRecord) *BM25Index {
	idx := &BM25Index{
		docs:   docs,
		tf:     make([]map[string]int, len(docs)),
		docLen: make([]int, len(docs)),
		df:     map[string]int{},
	}
	total := 0
	for i, d := range docs {
		toks := tokenize(d.Text)
		counts := make(map[string]int, len(toks))
		for _, t := range toks {
			counts[t]++
		}
		for t := range counts {
			idx.df[t]++
		}
		idx.tf[i] = counts
		idx.docLen[i] = len(toks)
		total += len(toks)
	}
	idx.avgDL = 1
	if len(docs) > 0 && total > 0 {
		idx.avgDL = float64(total) / float64(len(docs))
	}
	return idx
}

// Search returns up to k chunks ranked by BM25. An empty category means
// "search every category" (the "others" option).
func (idx *BM25Index) Search(query string, k int, category string) []Candidate {
	terms := uniqueTokens(tokenize(query))
	if len(terms) == 0 {
		return nil
	}

	n := float64(len(idx.docs))
	type hit struct {
		i     int
		score float64
	}
	var hits []hit

	for i, d := range idx.docs {
		if category != "" && d.Category != category {
			continue // metadata filter, same rule as the Pinecone filter
		}
		dl := float64(idx.docLen[i])
		var score float64
		for _, t := range terms {
			f := float64(idx.tf[i][t])
			if f == 0 {
				continue
			}
			df := float64(idx.df[t])
			idf := math.Log(1 + (n-df+0.5)/(df+0.5))
			score += idf * (f * (bm25K1 + 1)) / (f + bm25K1*(1-bm25B+bm25B*dl/idx.avgDL))
		}
		if score > 0 {
			hits = append(hits, hit{i, score})
		}
	}

	sort.Slice(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	if len(hits) > k {
		hits = hits[:k]
	}

	out := make([]Candidate, len(hits))
	for r, h := range hits {
		d := idx.docs[h.i]
		out[r] = Candidate{
			ID:           d.ID,
			Text:         d.Text,
			Source:       d.Source,
			Page:         d.Page,
			Category:     d.Category,
			KeywordRank:  r + 1,
			KeywordScore: h.score,
			RerankScore:  -1,
		}
	}
	return out
}

// ==========================================================
// Reciprocal Rank Fusion
// ==========================================================

func fuseRRF(k int, semantic, keyword []Candidate) []Candidate {
	byID := map[string]*Candidate{}
	var order []string

	get := func(c Candidate) *Candidate {
		if existing, ok := byID[c.ID]; ok {
			if existing.Text == "" {
				existing.Text = c.Text
			}
			return existing
		}
		cp := c
		byID[c.ID] = &cp
		order = append(order, c.ID)
		return &cp
	}

	for _, c := range semantic {
		m := get(c)
		m.SemanticRank, m.SemanticScore = c.SemanticRank, c.SemanticScore
		m.RRFScore += 1.0 / float64(k+c.SemanticRank)
	}
	for _, c := range keyword {
		m := get(c)
		m.KeywordRank, m.KeywordScore = c.KeywordRank, c.KeywordScore
		m.RRFScore += 1.0 / float64(k+c.KeywordRank)
	}

	out := make([]Candidate, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].RRFScore > out[b].RRFScore })
	return out
}