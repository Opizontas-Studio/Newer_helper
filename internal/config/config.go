package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// AppConfig 应用程序基础配置
type AppConfig struct {
	Name               string `mapstructure:"name"`
	Version            string `mapstructure:"version"`
	DisableInitialScan bool   `mapstructure:"disable_initial_scan"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	GuildsDBPath     string `mapstructure:"guilds_db_path"`
	UserDBPath       string `mapstructure:"user_db_path"`
	KickUserDBPath   string `mapstructure:"kick_user_db_path"`
	TimedTasksDBPath string `mapstructure:"timed_tasks_db_path"`
}

// ChannelConfig 频道配置
type ChannelConfig struct {
	ChannelID string   `mapstructure:"channel_id"`
	ThreadIDs []string `mapstructure:"thread_ids"`
}

// TaskConfig 任务配置
type TaskConfig struct {
	Name     string                    `mapstructure:"name"`
	GuildID  string                    `mapstructure:"guild_id"`
	Channels map[string]*ChannelConfig `mapstructure:"channels"`
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	Frequency          int      `mapstructure:"frequency"`
	Time               string   `mapstructure:"time"`
	TimeoutTime        string   `mapstructure:"timeout_time"`
	AddRoles           []string `mapstructure:"add_roles"`
	AddRoleTimeoutTime string   `mapstructure:"add_role_timeout_time"`
}

// PunishGuildConfig 惩罚系统服务器配置
type PunishGuildConfig struct {
	Name             string        `mapstructure:"name"`
	BaseRoleID       string        `mapstructure:"base_role_id"`
	RemoveRoleIDs    []string      `mapstructure:"remove_role_ids"`
	WhitelistRoleIDs []string      `mapstructure:"whitelist_role_ids"`
	Timeout          TimeoutConfig `mapstructure:"timeout"`
}

// PunishConfig 惩罚系统配置
type PunishConfig struct {
	InitConfig struct {
		DBPath string `mapstructure:"db_path"`
	} `mapstructure:"init_config"`
	Guilds map[string]*PunishGuildConfig `mapstructure:"guilds"`
}

// RollCardGuildConfig 抽卡系统服务器配置
type RollCardGuildConfig struct {
	Name             string            `mapstructure:"name"`
	GuildID          string            `mapstructure:"guild_id"`
	Database         string            `mapstructure:"database"`
	TagMappingFile   string            `mapstructure:"tag_mapping_file"`
	TableNameMapping map[string]string `mapstructure:"table_name_mapping"`
}

// RollCardConfig 抽卡系统配置
type RollCardConfig map[string]*RollCardGuildConfig

// ThreadGuildConfig 线程配置
type ThreadGuildConfig struct {
	Database  string `mapstructure:"database"`
	TableName string `mapstructure:"table_name"`
}

// ThreadConfig 线程配置
type ThreadConfig map[string]*ThreadGuildConfig

// Config 统一配置结构
type Config struct {
	// 从环境变量获取
	BotToken          string   `mapstructure:"bot_token"`
	LogChannelID      string   `mapstructure:"log_channel_id"`
	DeveloperUserIDs  []string `mapstructure:"developer_user_ids"`
	SuperAdminRoleIDs []string `mapstructure:"super_admin_role_ids"`

	// 从配置文件获取
	App      AppConfig              `mapstructure:"app"`
	Database DatabaseConfig         `mapstructure:"database"`
	Tasks    map[string]*TaskConfig `mapstructure:"tasks"`
	Punish   PunishConfig           `mapstructure:"punish"`
	RollCard RollCardConfig         `mapstructure:"rollcard"`
	Thread   ThreadConfig           `mapstructure:"thread"`

	// 从数据库动态加载的服务器配置
	ServerConfigs map[string]ServerConfig `mapstructure:"-"`
}

// ServerConfig 服务器配置结构 (保持与原来兼容)
type ServerConfig struct {
	Name           string                       `json:"name"`
	GuildID        string                       `json:"guilds_id"`
	AdminRoleIDs   []string                     `json:"admin_role_ids"`
	UserRoleIDs    []string                     `json:"user_role_ids"`
	PresetMessages []PresetMessage              `json:"preset_messages"`
	TopChannels    map[string]*TopChannelConfig `json:"top_channels,omitempty"`
}

// PresetMessage 预设消息结构
type PresetMessage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
}

// TopChannelConfig 回顶频道配置
type TopChannelConfig struct {
	ChannelID          string   `json:"channel_id"`
	MessageLimit       int      `json:"message_limit"`
	ExcludedMessageIDs []string `json:"excluded_message_ids"`
}

// Load 加载配置
func Load() (*Config, error) {
	// 设置Viper配置
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// 设置环境变量
	viper.SetEnvPrefix("DISCORD_BOT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 创建配置实例
	config := &Config{
		ServerConfigs: make(map[string]ServerConfig),
	}

	// 解析配置文件
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 从环境变量获取敏感信息
	config.BotToken = os.Getenv("BOT_TOKEN")
	if config.BotToken == "" {
		return nil, fmt.Errorf("BOT_TOKEN environment variable is required")
	}

	config.LogChannelID = os.Getenv("LOG_CHANNEL_ID")
	config.DeveloperUserIDs = getEnvAsSlice("DEVELOPER_USER_IDS")
	config.SuperAdminRoleIDs = getEnvAsSlice("SUPER_ADMIN_ROLE_IDS")

	// 应用环境变量覆盖
	if disableInitialScan := os.Getenv("DISABLE_INITIAL_SCAN"); disableInitialScan == "true" {
		config.App.DisableInitialScan = true
	}

	return config, nil
}

// getEnvAsSlice 从环境变量获取切片
func getEnvAsSlice(name string) []string {
	valStr := os.Getenv(name)
	if valStr == "" {
		return []string{}
	}
	return strings.Split(valStr, ",")
}

// GetPunishConfig 获取惩罚系统配置（兼容原有接口）
func (c *Config) GetPunishConfig() *PunishConfig {
	return &c.Punish
}

// GetTaskConfig 获取任务配置（兼容原有接口）
func (c *Config) GetTaskConfig() map[string]*TaskConfig {
	return c.Tasks
}

// GetRollCardConfig 获取抽卡配置（兼容原有接口）
func (c *Config) GetRollCardConfig() RollCardConfig {
	return c.RollCard
}

// GetThreadConfig 获取线程配置（兼容原有接口）
func (c *Config) GetThreadConfig() ThreadConfig {
	return c.Thread
}
