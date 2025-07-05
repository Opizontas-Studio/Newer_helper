package model

import (
	"encoding/json"
	"os"
)

// PresetMessage 定义了预设消息的结构
type PresetMessage struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
}

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

// SaveConfig saves the configuration to a file.
func SaveConfig(config *Config) error {
	file, err := os.Create("data/task_config.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}
