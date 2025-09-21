package main

import (
	"discord-bot/bot"
	"discord-bot/config"
	"discord-bot/handlers"
	"discord-bot/utils/database"
	punishment_db "discord-bot/utils/database/punish"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
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
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

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

	punishDB, err := punishment_db.InitPunishmentDB(cfg.KickConfig.InitConfig.DBPath)
	if err != nil {
		log.Fatalf("Error setting up punishment database: %v", err)
	}

	if err := database.LoadConfigFromDB(db, cfg); err != nil {
		log.Fatalf("Error loading config from database: %v", err)
	}

	b, err := bot.New(cfg, db, punishDB)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	handlers.Register(b)

	if err := b.Run(); err != nil {
		log.Fatalf("Error running bot: %v", err)
	}

	log.Println("Bot is now running. Press CTRL-C to exit.")

	// Wait for a shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	// Gracefully shutdown
	log.Println("Shutting down gracefully...")
	b.Close()
	log.Println("Bot has been shut down.")
}
