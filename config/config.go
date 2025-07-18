package config

import (
	"discord-bot/internal/config"
	"discord-bot/model"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func getEnvAsSlice(name string) []string {
	valStr := os.Getenv(name)
	if valStr == "" {
		return []string{}
	}
	return strings.Split(valStr, ",")
}

// 全局配置服务实例
var (
	configService *config.Service
	legacyAdapter *config.LegacyConfigAdapter
)

// Initialize 初始化配置系统
func Initialize() error {
	// 确保加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Println("Info: .env file not found, relying on environment variables")
	}

	configService = config.NewService()
	legacyAdapter = config.NewLegacyConfigAdapter(configService)

	// 尝试加载新的配置格式
	if err := configService.Load(); err != nil {
		log.Printf("Warning: Failed to load new config format: %v", err)
		log.Println("Falling back to legacy configuration loading...")
		return loadLegacyConfig()
	}

	log.Println("New configuration system initialized successfully")
	return nil
}

// Load loads the configuration (兼容性函数)
func Load() (*model.Config, error) {
	if configService == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}

	// 使用兼容性适配器返回旧格式
	return legacyAdapter.ToLegacyConfig(), nil
}

// loadLegacyConfig 加载旧配置格式（后备方案）
func loadLegacyConfig() error {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("Error: BOT_TOKEN environment variable not set")
	}

	logChannelID := os.Getenv("LOG_CHANNEL_ID")
	if logChannelID == "" {
		log.Println("Warning: LOG_CHANNEL_ID not set, logging will be disabled")
	}

	disableInitialScan := os.Getenv("DISABLE_INITIAL_SCAN") == "true"

	cfg := &model.Config{
		BotToken:           token,
		LogChannelID:       logChannelID,
		DeveloperUserIDs:   getEnvAsSlice("DEVELOPER_USER_IDS"),
		SuperAdminRoleIDs:  getEnvAsSlice("SUPER_ADMIN_ROLE_IDS"),
		DisableInitialScan: disableInitialScan,
		ServerConfigs:      make(map[string]model.ServerConfig),
	}

	// Load task config
	if err := loadJSON("data/task_config.json", &cfg.TaskConfig); err != nil {
		return err
	}

	// Load roll card config
	if err := loadJSON("data/roll_cardConfig.json", &cfg.RollCardConfigs); err != nil {
		return err
	}

	// Load thread config
	if err := loadJSON("data/thread_config.json", &cfg.ThreadConfig); err != nil {
		return err
	}

	log.Println("Legacy configuration loaded successfully")
	return nil
}

// GetService 获取配置服务实例
func GetService() *config.Service {
	return configService
}

// GetLegacyAdapter 获取兼容性适配器
func GetLegacyAdapter() *config.LegacyConfigAdapter {
	return legacyAdapter
}

// Reload 重新加载配置
func Reload() error {
	if configService != nil {
		return configService.Reload()
	}
	return Initialize()
}

func loadJSON(path string, v interface{}) error {
	configFile, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Warning: Config file not found at %s, skipping.", path)
			return nil
		}
		return err
	}
	return json.Unmarshal(configFile, v)
}
