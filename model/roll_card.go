package model

type RollCardConfig map[string]RollCardGuildConfig

// RollCardGuildConfig holds the configuration for a single guild.
type RollCardGuildConfig struct {
	Name                     string            `json:"name"`
	GuildID                  string            `json:"guilds_id"`
	Database                 string            `json:"database"`
	DataBaseTableNameMapping map[string]string `json:"dataBaseTableNameMapping"`
	TagMappingFile           string            `json:"tagMappingFile"`
}
