package main

import (
	"discord-bot/bot"
	"discord-bot/handlers"
	"discord-bot/utils"
	"log"
)

func main() {
	cfg := LoadConfig()
	db, err := utils.InitGuildDB("./data/guilds.db")
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer db.Close()

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
