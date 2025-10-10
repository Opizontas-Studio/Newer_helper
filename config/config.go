package config

import (
	"encoding/json"
	"log"
	"newer_helper/model"
	"os"
	"strconv"
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

	evidencePath := os.Getenv("EVIDENCE_PATH")
	if evidencePath == "" {
		evidencePath = "data/evidence"
	}

	evidenceMaxAgeDaysStr := os.Getenv("EVIDENCE_MAX_AGE_DAYS")
	if evidenceMaxAgeDaysStr == "" {
		evidenceMaxAgeDaysStr = "30"
	}
	evidenceMaxAgeDays, err := strconv.Atoi(evidenceMaxAgeDaysStr)
	if err != nil {
		log.Printf("Warning: Invalid EVIDENCE_MAX_AGE_DAYS value, using default of 30. Error: %v", err)
		evidenceMaxAgeDays = 30
	}

	cfg := &model.Config{
		BotToken:                 token,
		AppID:                    appID,
		LogChannelID:             logChannelID,
		DeveloperUserIDs:         strings.Split(os.Getenv("DEVELOPER_USER_IDS"), ","),
		SuperAdminRoleIDs:        strings.Split(os.Getenv("SUPER_ADMIN_ROLE_IDS"), ","),
		DisableInitialScan:       disableInitialScan,
		DisableCommandUnregister: disableCommandUnregister,
		ServerConfigs:            make(map[string]model.ServerConfig),
		EvidenceCleaner: model.EvidenceCleanerConfig{
			Path:       evidencePath,
			MaxAgeDays: evidenceMaxAgeDays,
		},
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

	if err := generateDatabaseMapping(cfg.TaskConfig); err != nil {
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

// generateDatabaseMapping generates the databaseMapping.json file from the task config.
func generateDatabaseMapping(taskConfig model.TaskConfig) error {
	databaseMapping := make(map[string]interface{})

	for guildID, guildConfig := range taskConfig {
		dataBaseTableNameMapping := make(map[string]string)
		for name, channelTask := range guildConfig.Data {
			if len(channelTask.ChannelID) >= 4 {
				key := name + "_" + channelTask.ChannelID[len(channelTask.ChannelID)-4:]
				dataBaseTableNameMapping[key] = name
			}
		}

		databaseMapping[guildID] = struct {
			GuildsID                 string            `json:"guilds_id"`
			Database                 string            `json:"database"`
			DataBaseTableNameMapping map[string]string `json:"dataBaseTableNameMapping"`
		}{
			GuildsID:                 guildID,
			Database:                 "data/" + guildID + ".db",
			DataBaseTableNameMapping: dataBaseTableNameMapping,
		}
	}

	file, err := json.MarshalIndent(databaseMapping, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile("data/databaseMapping.json", file, 0644)
}
