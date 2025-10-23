package model

import "fmt"

// Post represents a post from a Discord forum.
type Post struct {
	ID            string `json:"id"`
	ChannelID     string `json:"channel_id"`
	TableName     string `json:"-"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	AuthorID      string `json:"author_id"`
	Content       string `json:"content"`
	Tags          string `json:"tags"`
	MessageCount  int    `json:"message_count,omitempty"`
	Timestamp     int64  `json:"timestamp"`
	CoverImageURL string `json:"cover_image_url"`
}

// URL generates a Discord message link for the post.
func (p *Post) URL(guildID string) string {
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, p.ChannelID, p.ID)
}
