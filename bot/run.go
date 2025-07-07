package bot

import (
	"discord-bot/commands"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) Run() {
	err := b.Session.Open()
	if err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}

	log.Println("Removing old global commands...")
	existingGlobalCommands, err := b.Session.ApplicationCommands(b.Session.State.User.ID, "")
	if err != nil {
		log.Printf("Could not fetch global commands: %v", err)
	} else if len(existingGlobalCommands) > 0 {
		log.Printf("Removing %d old global commands...", len(existingGlobalCommands))
		for _, cmd := range existingGlobalCommands {
			err := b.Session.ApplicationCommandDelete(b.Session.State.User.ID, "", cmd.ID)
			if err != nil {
				log.Printf("Could not delete global command %s: %v", cmd.Name, err)
			}
		}
	}

	log.Println("Adding commands...")
	b.RegisteredCommands = make([]*discordgo.ApplicationCommand, 0)
	for _, serverCfg := range b.Config.ServerConfigs {
		b.RefreshCommands(serverCfg.GuildID)
	}

	// Perform initial scan
	if !b.Config.DisableInitialScan {
		scanMode := "full"
		lockFile, err := os.ReadFile("data/scan_lock.json")
		if err == nil {
			var lockData map[string]interface{}
			if json.Unmarshal(lockFile, &lockData) == nil {
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
		go commands.Scan(b.Session, b.Config.LogChannelID, scanMode, "")
	} else {
		log.Println("Initial scan is disabled by environment variable.")
	}

	b.startScanScheduler()

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func (b *Bot) RefreshCommands(guildID string) {
	serverCfg, ok := b.Config.ServerConfigs[guildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", guildID)
		return
	}

	log.Printf("Fetching commands for guild %s", serverCfg.GuildID)
	existingCommands, err := b.Session.ApplicationCommands(b.Session.State.User.ID, serverCfg.GuildID)
	if err != nil {
		log.Printf("cannot get commands for guild '%s': %v", serverCfg.GuildID, err)
		return
	}

	if len(existingCommands) > 0 {
		log.Printf("Removing %d old commands for guild %s...", len(existingCommands), serverCfg.GuildID)
		for _, cmd := range existingCommands {
			err := b.Session.ApplicationCommandDelete(b.Session.State.User.ID, serverCfg.GuildID, cmd.ID)
			if err != nil {
				log.Printf("cannot delete '%v' command for guild '%s': %v", cmd.Name, serverCfg.GuildID, err)
			}
		}
	}

	cmds := commands.GenerateCommands(&serverCfg)
	log.Printf("Adding %d new commands for guild %s...", len(cmds), serverCfg.GuildID)
	for _, v := range cmds {
		cmd, err := b.Session.ApplicationCommandCreate(b.Session.State.User.ID, serverCfg.GuildID, v)
		if err != nil {
			log.Printf("Cannot create '%v' command for guild '%s': %v", v.Name, serverCfg.GuildID, err)
		} else {
			b.RegisteredCommands = append(b.RegisteredCommands, cmd)
		}
	}
}

func (b *Bot) startScanScheduler() {
	// Schedule a full scan every 7 days (168 hours)
	b.FullScanTicker = time.NewTicker(168 * time.Hour)
	go func() {
		for range b.FullScanTicker.C {
			log.Println("Starting scheduled full forum scan...")
			commands.Scan(b.Session, b.Config.LogChannelID, "full", "")
		}
	}()

	// Schedule a cooldown cleanup every hour
	b.CooldownTicker = time.NewTicker(1 * time.Hour)
	go func() {
		for range b.CooldownTicker.C {
			log.Println("Cleaning up preset cooldowns...")
			b.cleanupCooldowns()
		}
	}()

	// Schedule daily tasks at 5:00 AM
	go func() {
		for {
			now := time.Now()
			// Calculate the next 5:00 AM
			next := time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, now.Location())
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}

			log.Printf("Next daily task scheduled for: %v", next)
			// Wait until the next 5:00 AM
			time.Sleep(next.Sub(now))

			// Run the tasks
			log.Println("Starting scheduled active forum scan...")
			commands.Scan(b.Session, b.Config.LogChannelID, "active", "")

			log.Println("Cleaning up old posts...")
			commands.CleanOldPosts(b.Session, b.Config.LogChannelID)
		}
	}()
}
