package bot

import (
	"database/sql"
	"discord-bot/commands"
	"discord-bot/handlers/leaderboard"
	"discord-bot/model"
	"discord-bot/utils"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	Session                 *discordgo.Session
	RegisteredCommands      []*discordgo.ApplicationCommand
	Config                  *model.Config
	CommandHandlers         map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
	PresetCooldowns         map[string]time.Time
	CooldownMutex           sync.Mutex
	FullScanTicker          *time.Ticker
	ActiveScanTicker        *time.Ticker
	CooldownTicker          *time.Ticker
	PostCleanupTicker       *time.Ticker
	LeaderboardUpdateTicker *time.Ticker
	DB                      *sql.DB
	ActiveScanCount         int
}

func (b *Bot) GetConfig() *model.Config {
	return b.Config
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

	return &Bot{
		Session:         dg,
		Config:          cfg,
		PresetCooldowns: make(map[string]time.Time),
		DB:              db,
		ActiveScanCount: 0,
	}, nil
}
func (b *Bot) Close() {
	log.Println("Gracefully shutting down.")
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
	b.Session.Close()
}

func (b *Bot) UpdateLeaderboard() {
	state, err := utils.LoadLeaderboardState()
	if err != nil {
		log.Printf("Error loading leaderboard state for update: %v", err)
		return
	}

	for _, serverCfg := range b.Config.ServerConfigs {
		if serverCfg.GuildID == state.GuildID {
			log.Printf("Updating leaderboard for guild: %s", state.GuildID)
			leaderboard.UpdateLeaderboard(b, state.GuildID)
			break
		}
	}
}

func (b *Bot) RefreshCommands(guildID string) {
	serverCfg, ok := b.Config.ServerConfigs[guildID]
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
