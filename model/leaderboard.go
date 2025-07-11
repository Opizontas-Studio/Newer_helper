package model

// LeaderboardState holds the necessary information to find and update a leaderboard message.
type LeaderboardState struct {
	GuildID   string `json:"guild_id"`
	ChannelID string `json:"channel_id"`
	MessageID string `json:"message_id"`
}

type GuildMapping struct {
	GuildsID                 string            `json:"guilds_id"`
	Database                 string            `json:"database"`
	DataBaseTableNameMapping map[string]string `json:"database_table_name_mapping"`
}
