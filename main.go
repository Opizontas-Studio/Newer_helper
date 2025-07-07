package main

import (
	"discord-bot/bot"
	"discord-bot/handlers"
	"discord-bot/utils"
	"encoding/json"
	"log"
	"os"
)

func main() {
	cfg := LoadConfig()
	if err := os.MkdirAll("./data", os.ModePerm); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
	db, err := utils.InitDB("./data/guilds.db")
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	if err := utils.CreateGuildTables(db); err != nil {
		log.Fatalf("Error creating guild tables: %v", err)
	}
	if err := utils.LoadConfigFromDB(db, cfg); err != nil {
		log.Fatalf("Error loading config from database: %v", err)
	}

	// Load task config
	configFile, err := os.ReadFile("data/task_config.json")
	if err != nil {
		log.Fatalf("Error reading task_config.json: %v", err)
	}
	if err := json.Unmarshal(configFile, &cfg.TaskConfig); err != nil {
		log.Fatalf("Error unmarshalling task_config.json: %v", err)
	}

	// Load roll card config
	rollCardConfigFile, err := os.ReadFile("data/roll_cardConfig.json")
	if err != nil {
		log.Fatalf("Error reading roll_cardConfig.json: %v", err)
	}
	if err := json.Unmarshal(rollCardConfigFile, &cfg.RollCardConfigs); err != nil {
		log.Fatalf("Error unmarshalling roll_cardConfig.json: %v", err)
	}

	b, err := bot.New(cfg, db)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	handlers.Register(b)

	b.Run()

	defer b.Close()
}
