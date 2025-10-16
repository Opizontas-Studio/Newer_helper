package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"newer_helper/bot"
	"newer_helper/config"
	grpcclient "newer_helper/grpc/client"
	grpcserver "newer_helper/grpc/server"
	"newer_helper/handlers"
	"newer_helper/utils"
	"newer_helper/utils/database"
	punishments_db "newer_helper/utils/database/punishments"
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

	// Load punish config and initialize punishment database
	punishConfig, err := utils.LoadPunishConfig("config/config_file/punish_config.json")
	if err != nil {
		log.Fatalf("Error loading punish config: %v", err)
	}

	punishDB, err := punishments_db.Init(punishConfig.DatabasePath)
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

	// Initialize gRPC punish server with punishment database
	punishServer := grpcserver.NewPunishServer(punishDB)
	log.Println("Initialized gRPC Punish Server")

	// Initialize and connect gRPC client with punish server
	grpcClient, err := grpcclient.NewClient(punishServer)
	if err != nil {
		log.Printf("Warning: Failed to create gRPC client: %v", err)
		log.Println("Continuing without gRPC connection...")
	} else {
		if err := grpcClient.Connect(); err != nil {
			log.Printf("Warning: Failed to connect to gRPC gateway: %v", err)
			log.Println("Continuing without gRPC connection...")
		} else {
			defer grpcClient.Close()
			log.Println("gRPC client connected and ready to handle punish service requests")
		}
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
