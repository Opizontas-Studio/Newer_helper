package model

// PresetMessage 定义了预设消息的结构
type PresetMessage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
}

// TopChannelConfig 定义了回顶频道的配置
type TopChannelConfig struct {
	ChannelID          string   `json:"channel_id"`
	MessageLimit       int      `json:"message_limit"`
	ExcludedMessageIDs []string `json:"excluded_message_ids"`
}

// AutoTriggerConfig 定义了自动触发的配置
type AutoTriggerConfig struct {
	ID        int      `json:"id"`
	Keywords  []string `json:"keywords"`
	PresetID  string   `json:"preset_id"`
	ChannelID string   `json:"channel_id"`
}

// ServerConfig 定义了每个服务器的配置
type ServerConfig struct {
	Name           string                       `json:"name"`
	GuildID        string                       `json:"guilds_id"`
	AdminRoleIDs   []string                     `json:"admin_role_ids"`
	UserRoleIDs    []string                     `json:"user_role_ids"`
	Enable         bool                         `json:"enable"`
	PresetMessages []PresetMessage              `json:"preset_messages"`
	TopChannels    map[string]*TopChannelConfig `json:"top_channels,omitempty"`
	AutoTriggers   []AutoTriggerConfig          `json:"auto_triggers,omitempty"`
}

// PunishmentStatsChannel 定义了处罚统计频道的配置
type PunishmentStatsChannel struct {
	ChannelID     string `json:"channel_id"`
	GuildID       string `json:"guild_id"`
	MessageID     string `json:"message_id"`
	TargetGuildID string `json:"target_guild_id"`
}

// Config 存储应用程序的配置
type Config struct {
	BotToken                 string
	AppID                    string
	LogChannelID             string
	DeveloperUserIDs         []string
	SuperAdminRoleIDs        []string
	DisableInitialScan       bool
	DisableCommandUnregister bool
	ServerConfigs            map[string]ServerConfig
	PunishmentStatsChannels  map[string]PunishmentStatsChannel
	RollCardConfigs          RollCardConfig
	TaskConfig               TaskConfig
	ThreadConfig             ThreadConfig
	DatabaseMapping          map[string]struct {
		Data map[string]struct {
			ChannelID string `json:"channel_id"`
			TableName string `json:"table_name"`
		} `json:"data"`
	}
	KickConfig      KickConfig
	EvidenceCleaner EvidenceCleanerConfig
}

// ThreadConfig holds the configuration for thread database paths.
type ThreadConfig map[string]ThreadGuildConfig

// ThreadGuildConfig holds the database path and table name for a single guild.
type ThreadGuildConfig struct {
	Name      string `json:"name"`
	Database  string `json:"database"`
	TableName string `json:"tableName"`
}

// KickConfigEntry defines the settings for a specific kick configuration.
type TimeoutConfig struct {
	Frequency          int      `json:"frequency"`
	Time               string   `json:"time"`
	TimeoutTime        string   `json:"timeout_time"`
	AddRole            []string `json:"add_role"`
	AddRoleTimeoutTime string   `json:"add_role_timeout_time"`
}

// TimeoutConfig defines the settings for user timeout punishments.
type KickConfigEntry struct {
	Name            string        `json:"name"`
	BaseRoleID      string        `json:"base_role_id"`
	LogChannelID    string        `json:"log_channel_id,omitempty"`
	RemoveRoleID    []string      `json:"remove_role_id"`
	WhitelistRoleID []string      `json:"whitelist_role_id"`
	Timeout         TimeoutConfig `json:"timeout,omitempty"`
}

// KickConfig defines the overall structure for kick configurations.
type KickConfig struct {
	InitConfig struct {
		DBPath string `json:"dbpath"`
	} `json:"initConfig"`
	Data map[string]KickConfigEntry `json:"data"`
}

// PersistentPanelInfo 存储持久化面板信息
type PersistentPanelInfo struct {
	MessageID   string `json:"message_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
	LastUpdated int64  `json:"last_updated"`
}

// PersistentPanelData 存储所有持久化面板数据
type PersistentPanelData struct {
	Panels map[string]map[string]*PersistentPanelInfo `json:"panels"` // guild_id -> channel_id -> panel_info
}

// TaskConfig represents the structure of task_config.json
type TaskConfig map[string]GuildTaskConfig

// GuildTaskConfig represents the configuration for a single guild's tasks.
type GuildTaskConfig struct {
	Name     string                 `json:"name"`
	GuildsID string                 `json:"guilds_id"`
	Data     map[string]ChannelTask `json:"data"`
}

// ChannelTask represents a task for a specific channel.
type ChannelTask struct {
	ChannelID string   `json:"channel_id"`
	ThreadID  []string `json:"thread_id"`
}

// EvidenceCleanerConfig holds the configuration for the evidence cleaner.
type EvidenceCleanerConfig struct {
	Path       string
	MaxAgeDays int
}
