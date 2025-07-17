//go:build !migrate

package main

import (
	"discord-bot/bot"
	"discord-bot/config"
	"discord-bot/handlers"
	"discord-bot/utils/database"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
)

func startPprofServer() {
	if os.Getenv("ENABLE_PPROF") == "true" {
		log.Println("Starting pprof server on :6060")
		go func() {
			if err := http.ListenAndServe(":6060", nil); err != nil {
				log.Printf("Failed to start pprof server: %v", err)
			}
		}()
	}
}

func main() {
	startPprofServer()

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
