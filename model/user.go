package model

import "time"

// User 用户信息
type User struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	Discriminator string    `json:"discriminator"`
	GlobalName    string    `json:"global_name"`
	AvatarURL     string    `json:"avatar_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// UserStats 用户统计信息
type UserStats struct {
	UserID       string    `json:"user_id"`
	GuildID      string    `json:"guild_id"`
	TotalPosts   int       `json:"total_posts"`
	TotalRolls   int       `json:"total_rolls"`
	LastActivity time.Time `json:"last_activity"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserRanking 用户排名信息
type UserRanking struct {
	UserID     string `json:"user_id"`
	GuildID    string `json:"guild_id"`
	PostRank   int    `json:"post_rank"`
	PostCount  int    `json:"post_count"`
	RollRank   int    `json:"roll_rank"`
	RollCount  int    `json:"roll_count"`
	TotalRank  int    `json:"total_rank"`
	TotalScore int    `json:"total_score"`
}

// LeaderboardEntry 排行榜条目
type LeaderboardEntry struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	GuildID   string    `json:"guild_id"`
	Count     int       `json:"count"`
	Rank      int       `json:"rank"`
	Score     int       `json:"score"`
	UpdatedAt time.Time `json:"updated_at"`
}
