package model

// PresetMessage 定义了预设消息的结构
type PresetMessage struct {
	ID          string `json:"id"`
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
	BotToken           string
	LogChannelID       string
	DeveloperUserIDs   []string
	SuperAdminRoleIDs  []string
	DisableInitialScan bool
	ServerConfigs      map[string]ServerConfig
	RollCardConfigs    RollCardConfig
	TaskConfig         map[string]struct {
		Data map[string]struct {
			ChannelID string `json:"channel_id"`
			TableName string `json:"table_name"`
		} `json:"data"`
	}
	ThreadConfig    ThreadConfig
	DatabaseMapping map[string]struct {
		Data map[string]struct {
			ChannelID string `json:"channel_id"`
			TableName string `json:"table_name"`
		} `json:"data"`
	}
}

// ThreadConfig holds the configuration for thread database paths.
type ThreadConfig map[string]ThreadGuildConfig

// ThreadGuildConfig holds the database path and table name for a single guild.
type ThreadGuildConfig struct {
	Database  string `json:"database"`
	TableName string `json:"tableName"`
}

// KickConfigEntry defines the settings for a specific kick configuration.
// TimeoutConfig defines the settings for user timeout punishments.
type TimeoutConfig struct {
	Frequency          int      `json:"frequency"`
	Time               string   `json:"time"`
	TimeoutTime        string   `json:"timeout_time"`
	AddRole            []string `json:"add_role"`
	AddRoleTimeoutTime string   `json:"add_role_timeout_time"`
}

type KickConfigEntry struct {
	Name            string        `json:"name"`
	LogChannelID    string        `json:"log_channel_id,omitempty"`
	RemoveRoleID    []string      `json:"remove_role_id"`
	WhitelistRoleID []string      `json:"whitelist_role_id"`
	Timeout         TimeoutConfig `json:"timeout,omitempty"`
}

// KickConfig defines the overall structure for kick configurations.
type KickConfig struct {
	InitConfig struct {
		DBPath string                     `json:"dbpath"`
		Data   map[string]KickConfigEntry `json:"data"`
	} `json:"initConfig"`
}
