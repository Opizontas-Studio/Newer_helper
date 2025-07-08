package model

// LeaderboardState holds the necessary information to find and update a leaderboard message.
type LeaderboardState struct {
	GuildID   string `json:"guild_id"`
	ChannelID string `json:"channel_id"`
	MessageID string `json:"message_id"`
}
