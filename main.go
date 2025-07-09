package main

import (
	"discord-bot/bot"
	"discord-bot/config"
	"discord-bot/handlers"
	"discord-bot/utils/database"
	"log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	db, err := database.InitDB("./data/guilds.db")
	if err != nil {
		log.Fatalf("Error setting up database: %v", err)
	}
	if err := database.CreateGuildTables(db); err != nil {
		log.Fatalf("Error creating guild tables: %v", err)
	}

	if _, err := database.InitUserDB(); err != nil {
		log.Fatalf("Error setting up user database: %v", err)
	}

	if err := database.LoadConfigFromDB(db, cfg); err != nil {
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
