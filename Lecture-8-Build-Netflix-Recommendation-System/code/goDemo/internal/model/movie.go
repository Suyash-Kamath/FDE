package model

// Movie is a catalog entry together with its precomputed embedding vector.
type Movie struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Embedding   []float64 `json:"embedding"`
}
