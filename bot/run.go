package bot

import (
	"discord-bot/scanner"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (b *Bot) Run() {
	err := b.discord.Connect()
	if err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}

	log.Println("Adding commands...")
	for _, serverCfg := range b.GetConfig().ServerConfigs {
		b.RefreshCommands(serverCfg.GuildID)
	}

	// Perform initial scan
	if !b.GetConfig().DisableInitialScan {
		scanMode := "full"
		lockFile, err := os.ReadFile("data/scan_lock.json")
		if err == nil {
			var lockData map[string]interface{}
			if json.Unmarshal(lockFile, &lockData) == nil {
				if count, ok := lockData["active_scan_count"].(float64); ok {
					b.ActiveScanCount = int(count)
				}
				if mode, ok := lockData["scan_mode"].(string); ok && mode == "full" {
					if timestamp, ok := lockData["timestamp"].(float64); ok {
						if time.Since(time.Unix(int64(timestamp), 0)) < 24*time.Hour {
							scanMode = "active"
							log.Println("Last full scan was less than 24 hours ago. Starting with active scan.")
						}
					}
				}
			}
		}
		go scanner.Scan(b.GetSession(), b.GetConfig().LogChannelID, scanMode, "", b.done)
	} else {
		log.Println("Initial scan is disabled by environment variable.")
	}

	b.startScanScheduler()

	// Start the role remover scheduler
	// Use ConfigService to get punishment configuration
	punishConfig := b.GetConfigService().GetPunishConfig()
	timedTaskDB, err := database.InitTimedTaskDB(punishConfig.InitConfig.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize timed task DB: %v", err)
	}
	scanner.StartRoleRemover(b.GetSession(), timedTaskDB)

	// Start the channel cleaner scheduler
	go scanner.StartChannelCleaner(b.GetSession(), b.GetConfig(), b.done)

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	utils.LogInfo(b.GetSession(), b.GetConfig().LogChannelID, "System", "Startup", "Bot has started successfully.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func (b *Bot) startScanScheduler() {
	// 使用新的调度器服务
	b.scheduler.Start()

	// Schedule a cooldown cleanup every hour
	b.scheduler.AddJob("cooldown_cleanup", 1*time.Hour, func() {
		log.Println("Cleaning up preset cooldowns...")
		b.CleanupCooldowns()
	})

	// Schedule leaderboard update every 10 minutes
	b.scheduler.AddJob("leaderboard_update", 10*time.Minute, func() {
		log.Println("Updating leaderboard...")
		b.UpdateLeaderboard()
	})

	// Schedule post deletion checks
	b.scheduler.AddJob("post_scan", 30*time.Minute, func() {
		log.Println("Running post deletion scan...")
		scanner.CheckDeletedPosts(b.GetSession(), b.GetConfig().LogChannelID)
	})

	// Schedule daily tasks at 5:00, 13:00, and 21:00
	go func() {
		runHours := []int{5, 13, 21} // 5 AM, 1 PM, 9 PM

		for {
			now := time.Now()
			var next time.Time

			// Find the next scheduled time
			for _, h := range runHours {
				t := time.Date(now.Year(), now.Month(), now.Day(), h, 0, 0, 0, now.Location())
				if now.Before(t) {
					next = t
					break
				}
			}

			// If no scheduled time is found for today, schedule for the first hour tomorrow
			if next.IsZero() {
				tomorrow := now.Add(24 * time.Hour)
				next = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), runHours[0], 0, 0, 0, now.Location())
			}

			log.Printf("Next daily task scheduled for: %v", next)
			// Wait until the next scheduled time
			select {
			case <-time.After(next.Sub(now)):
				// Run the tasks
				log.Println("Starting scheduled active forum scan...")
				scanner.Scan(b.GetSession(), b.GetConfig().LogChannelID, "active", "", b.done)

				b.ActiveScanCount++
				log.Printf("Active scan count: %d", b.ActiveScanCount)

				if b.ActiveScanCount >= 21 { // 3 scans/day * 7 days
					log.Println("Active scan count reached 21. Starting full scan...")
					scanner.Scan(b.GetSession(), b.GetConfig().LogChannelID, "full", "", b.done)
					b.ActiveScanCount = 0
				}

				// Persist the scan count
				lockData := make(map[string]interface{})
				lockFile, err := os.ReadFile("data/scan_lock.json")
				if err == nil {
					json.Unmarshal(lockFile, &lockData)
				}
				lockData["active_scan_count"] = b.ActiveScanCount
				lockFile, err = json.Marshal(lockData)
				if err == nil {
					os.WriteFile("data/scan_lock.json", lockFile, 0644)
				}

				log.Println("Cleaning up old posts...")
				scanner.CleanOldPosts(b.GetSession(), b.GetConfig(), b.done)
			case <-b.done:
				return
			}
		}
	}()
}
