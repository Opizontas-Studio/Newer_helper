package model

// ServerConfig 定义了每个服务器的配置
type ServerConfig struct {
	Name           string          `json:"name"`
	GuildID        string          `json:"guilds_id"`
	AdminRoleIDs   []string        `json:"admin_role_ids"`
	UserRoleIDs    []string        `json:"user_role_ids"`
	PresetMessages []PresetMessage `json:"preset_messages"`
}

// Config 存储应用程序的配置
type Config struct {
	BotToken      string
	LogWebhookURL string
	ServerConfigs map[string]ServerConfig
}
