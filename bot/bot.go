package bot

import (
	"database/sql"
	"discord-bot/model"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	Session            *discordgo.Session
	RegisteredCommands []*discordgo.ApplicationCommand
	Config             *model.Config
	CommandHandlers    map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
	PresetCooldowns    map[string]time.Time
	CooldownMutex      sync.Mutex
	FullScanTicker     *time.Ticker
	ActiveScanTicker   *time.Ticker
	DB                 *sql.DB
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
	b.Session.Close()
	b.DB.Close()
}
