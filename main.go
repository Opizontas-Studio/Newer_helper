package main

import (
	"discord-bot/bot"
	"discord-bot/config"
	"discord-bot/handlers"
	"discord-bot/utils"
	"log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	db, err := utils.SetupDatabase()
	if err != nil {
		log.Fatalf("Error setting up database: %v", err)
	}

	if err := utils.LoadConfigFromDB(db, cfg); err != nil {
		log.Fatalf("Error loading config from database: %v", err)
	}

	b, err := bot.New(cfg, db)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	handlers.Register(b)

	b.Run()

	defer b.Close()
}
