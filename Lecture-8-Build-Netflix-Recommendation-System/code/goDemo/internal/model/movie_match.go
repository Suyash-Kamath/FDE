package model

// MovieMatch is a search/similarity result returned to the client.
type MovieMatch struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Match       float64 `json:"match"`
}
