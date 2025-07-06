package model

// Post represents a post from a Discord forum.
type Post struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	AuthorID      string `json:"author_id"`
	Content       string `json:"content"`
	Tags          string `json:"tags"`
	MessageCount  int    `json:"message_count"`
	Timestamp     int64  `json:"timestamp"`
	CoverImageURL string `json:"cover_image_url"`
}
