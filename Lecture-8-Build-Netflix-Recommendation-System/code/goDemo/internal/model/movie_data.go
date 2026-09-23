package model

// MovieData mirrors one entry of resources/movies.json.
type MovieData struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}
