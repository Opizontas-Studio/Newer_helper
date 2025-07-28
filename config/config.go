package config

import (
	"discord-bot/model"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Load loads the configuration from environment variables and JSON files.
func Load() (*model.Config, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Info: .env file not found, relying on environment variables")
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("Error: BOT_TOKEN environment variable not set")
	}

	appID := os.Getenv("APP_ID")
	if appID == "" {
		log.Fatal("Error: APP_ID environment variable not set")
	}

	logChannelID := os.Getenv("LOG_CHANNEL_ID")
	if logChannelID == "" {
		log.Println("Warning: LOG_CHANNEL_ID not set, logging will be disabled")
	}

	disableInitialScan := os.Getenv("DISABLE_INITIAL_SCAN") == "true"
	disableCommandUnregister := os.Getenv("DISABLE_COMMAND_UNREGISTER") == "true"

	cfg := &model.Config{
		BotToken:                 token,
		AppID:                    appID,
		LogChannelID:             logChannelID,
		DeveloperUserIDs:         strings.Split(os.Getenv("DEVELOPER_USER_IDS"), ","),
		SuperAdminRoleIDs:        strings.Split(os.Getenv("SUPER_ADMIN_ROLE_IDS"), ","),
		DisableInitialScan:       disableInitialScan,
		DisableCommandUnregister: disableCommandUnregister,
		ServerConfigs:            make(map[string]model.ServerConfig),
	}

	// Load task config
	if err := loadJSON("data/task_config.json", &cfg.TaskConfig); err != nil {
		return nil, err
	}

	// Load roll card config
	if err := loadJSON("data/roll_cardConfig.json", &cfg.RollCardConfigs); err != nil {
		return nil, err
	}

	// Load thread config
	if err := loadJSON("data/thread_config.json", &cfg.ThreadConfig); err != nil {
		return nil, err
	}

	// Load kick config
	if err := loadJSON("data/kick_config.json", &cfg.KickConfig); err != nil {
		return nil, err
	}

	return cfg, nil
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
