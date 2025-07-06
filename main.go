package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"discord-bot/commands"
	"discord-bot/model"
	"discord-bot/utils"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	Session            *discordgo.Session
	registeredCommands []*discordgo.ApplicationCommand
	Config             *model.Config
	CommandHandlers    map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
	PresetCooldowns    map[string]time.Time
	cooldownMutex      sync.Mutex
	fullScanTicker     *time.Ticker
	activeScanTicker   *time.Ticker
}

func (b *Bot) GetConfig() *model.Config {
	return b.Config
}

func (b *Bot) GetPresetCooldowns() map[string]time.Time {
	return b.PresetCooldowns
}

func (b *Bot) GetCooldownMutex() *sync.Mutex {
	return &b.cooldownMutex
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

	commands := GenerateCommands(&serverCfg)
	log.Printf("Adding %d new commands for guild %s...", len(commands), serverCfg.GuildID)
	for _, v := range commands {
		cmd, err := b.Session.ApplicationCommandCreate(b.Session.State.User.ID, serverCfg.GuildID, v)
		if err != nil {
			log.Printf("Cannot create '%v' command for guild '%s': %v", v.Name, serverCfg.GuildID, err)
		} else {
			b.registeredCommands = append(b.registeredCommands, cmd)
		}
	}
}

func NewBot(cfg *model.Config) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, err
	}
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentMessageContent

	bot := &Bot{
		Session:         dg,
		Config:          cfg,
		PresetCooldowns: make(map[string]time.Time),
	}
	bot.CommandHandlers = GetCommandHandlers(bot)
	bot.addHandlers()

	return bot, nil
}

func (b *Bot) addHandlers() {
	b.Session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
		if b.Config.LogChannelID != "" {
			err := utils.LogInfo(s, b.Config.LogChannelID, "System", "启动", "Bot has started successfully.")
			if err != nil {
				log.Printf("Failed to send startup log: %v", err)
			}
		}
	})
	b.Session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := b.CommandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
	b.Session.AddHandler(func(s *discordgo.Session, t *discordgo.ThreadCreate) {
		// Pass the log channel ID from the config to the handler
		commands.HandleThreadCreate(s, t, b.Config.LogChannelID)
	})
}

func (b *Bot) Run() {
	err := b.Session.Open()
	if err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}

	log.Println("Adding commands...")
	b.registeredCommands = make([]*discordgo.ApplicationCommand, 0)

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

	for _, serverCfg := range b.Config.ServerConfigs {
		b.RefreshCommands(serverCfg.GuildID)
	}

	// Perform initial scan
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
	go commands.Scan(b.Session, b.Config.LogChannelID, scanMode)

	b.startScanScheduler()

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	b.Close()
}

func (b *Bot) Close() {
	log.Println("Removing commands...")
	for _, v := range b.registeredCommands {
		err := b.Session.ApplicationCommandDelete(b.Session.State.User.ID, v.GuildID, v.ID)
		if err != nil {
			log.Printf("Cannot delete '%v' command: %v", v.Name, err)
		}
	}

	log.Println("Gracefully shutting down.")
	if b.fullScanTicker != nil {
		b.fullScanTicker.Stop()
	}
	if b.activeScanTicker != nil {
		b.activeScanTicker.Stop()
	}
	b.Session.Close()
}

func (b *Bot) startScanScheduler() {
	// Schedule a full scan every 7 days (168 hours)
	b.fullScanTicker = time.NewTicker(168 * time.Hour)
	go func() {
		for range b.fullScanTicker.C {
			log.Println("Starting scheduled full forum scan...")
			commands.Scan(b.Session, b.Config.LogChannelID, "full")
		}
	}()

	// Schedule an active scan every 24 hours
	b.activeScanTicker = time.NewTicker(24 * time.Hour)
	go func() {
		for range b.activeScanTicker.C {
			log.Println("Starting scheduled active forum scan...")
			commands.Scan(b.Session, b.Config.LogChannelID, "active")
		}
	}()
}

func main() {
	cfg := LoadConfig()
	bot, err := NewBot(cfg)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	bot.Run()
}
