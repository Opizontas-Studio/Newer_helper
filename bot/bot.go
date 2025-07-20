package bot

import (
	"database/sql"
	"discord-bot/commands"
	"discord-bot/config"
	"discord-bot/model"
	"discord-bot/utils/database"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	Session                 *discordgo.Session
	RegisteredCommands      []*discordgo.ApplicationCommand
	config                  atomic.Value // *model.Config
	CommandHandlers         map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
	PresetCooldowns         map[string]time.Time
	CooldownMutex           sync.Mutex
	FullScanTicker          *time.Ticker
	ActiveScanTicker        *time.Ticker
	CooldownTicker          *time.Ticker
	PostCleanupTicker       *time.Ticker
	LeaderboardUpdateTicker *time.Ticker
	PostScanTicker          *time.Ticker
	DegradedPostScanTicker  *time.Ticker
	DB                      *sql.DB
	ActiveScanCount         int
	done                    chan struct{}
}

func (b *Bot) GetConfig() *model.Config {
	return b.config.Load().(*model.Config)
}

func (b *Bot) GetPresetCooldowns() map[string]time.Time {
	return b.PresetCooldowns
}

func (b *Bot) GetCooldownMutex() *sync.Mutex {
	return &b.CooldownMutex
}

func (b *Bot) GetDB() *sql.DB {
	return b.DB
}

func (b *Bot) GetSession() *discordgo.Session {
	return b.Session
}

func New(cfg *model.Config, db *sql.DB) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, err
	}
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentMessageContent
	dg.StateEnabled = false

	b := &Bot{
		Session:         dg,
		PresetCooldowns: make(map[string]time.Time),
		DB:              db,
		ActiveScanCount: 0,
		done:            make(chan struct{}),
	}
	b.config.Store(cfg)
	return b, nil
}

func (b *Bot) Close() {
	log.Println("Gracefully shutting down.")
	close(b.done) // Signal all goroutines to stop

	if b.FullScanTicker != nil {
		b.FullScanTicker.Stop()
	}
	if b.ActiveScanTicker != nil {
		b.ActiveScanTicker.Stop()
	}
	if b.CooldownTicker != nil {
		b.CooldownTicker.Stop()
	}
	if b.PostCleanupTicker != nil {
		b.PostCleanupTicker.Stop()
	}
	if b.LeaderboardUpdateTicker != nil {
		b.LeaderboardUpdateTicker.Stop()
	}
	if b.PostScanTicker != nil {
		b.PostScanTicker.Stop()
	}
	if b.DegradedPostScanTicker != nil {
		b.DegradedPostScanTicker.Stop()
	}
	b.Session.Close()
}

func (b *Bot) RefreshCommands(guildID string) {
	serverCfg, ok := b.GetConfig().ServerConfigs[guildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", guildID)
		return
	}
	log.Printf("Updating commands for guild %s", serverCfg.GuildID)

	cmds := commands.GenerateCommands(&serverCfg)
	log.Printf("Registering %d new commands for guild %s...", len(cmds), serverCfg.GuildID)
	registeredCmds, err := b.Session.ApplicationCommandBulkOverwrite(b.Session.State.User.ID, serverCfg.GuildID, cmds)
	if err != nil {
		log.Printf("cannot update commands for guild '%s': %v", serverCfg.GuildID, err)
		return
	}
	b.RegisteredCommands = append(b.RegisteredCommands, registeredCmds...)
}

func (b *Bot) CleanupCooldowns() {
	b.CooldownMutex.Lock()
	defer b.CooldownMutex.Unlock()

	for id, t := range b.PresetCooldowns {
		if time.Since(t) > 1*time.Hour {
			delete(b.PresetCooldowns, id)
		}
	}
}

func (b *Bot) ReloadConfig() error {
	log.Println("Reloading configuration...")
	newCfg, err := config.Load()
	if err != nil {
		log.Printf("Error reloading config: %v", err)
		return err
	}

	// Load server-specific configs from DB
	if err := database.LoadConfigFromDB(b.DB, newCfg); err != nil {
		log.Printf("Error loading config from database during reload: %v", err)
		return err
	}

	b.config.Store(newCfg)
	log.Println("Configuration reloaded successfully.")

	log.Println("Refreshing commands for all guilds...")
	for _, serverCfg := range newCfg.ServerConfigs {
		go b.RefreshCommands(serverCfg.GuildID)
	}

	return nil
}
