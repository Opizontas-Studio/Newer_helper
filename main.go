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
	db, err := utils.InitGuildDB("./data/guilds.db")
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
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

	b, err := bot.New(cfg, db)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	handlers.Register(b)

	b.Run()

	defer b.Close()
}
