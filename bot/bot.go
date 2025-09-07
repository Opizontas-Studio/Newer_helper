package bot

import (
	"context"
	"database/sql"
	"discord-bot/commands"
	"discord-bot/config"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

// PendingPreset holds the data for a preset message awaiting confirmation.
type PendingPreset struct {
	MessageSend *discordgo.MessageSend
	LogInfo     string
	PresetName  string
	UserID      string
	Timestamp   time.Time
}

type Bot struct {
	Session             *discordgo.Session
	AppID               string
	RegisteredCommands  []*discordgo.ApplicationCommand
	commandsMutex       sync.Mutex
	config              atomic.Value // *model.Config
	CommandHandlers     map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
	PresetCooldowns     map[string]time.Time
	CooldownMutex       sync.Mutex
	PendingPresets      map[string]*PendingPreset
	PendingPresetsMutex sync.Mutex
	DB                  *sql.DB
	DBX                 *sqlx.DB
	activeScanCount     int
	scheduler           *Scheduler
	ctx                 context.Context
	cancel              context.CancelFunc
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

func (b *Bot) GetDBX() *sqlx.DB {
	return b.DBX
}

func (b *Bot) ActiveScanCount() *int {
	return &b.activeScanCount
}

func (b *Bot) GetCtx() context.Context {
	return b.ctx
}

func New(cfg *model.Config, db *sql.DB, dbx *sqlx.DB) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, err
	}
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentMessageContent
	dg.StateEnabled = false

	ctx, cancel := context.WithCancel(context.Background())

	b := &Bot{
		Session:         dg,
		AppID:           cfg.AppID,
		PresetCooldowns: make(map[string]time.Time),
		PendingPresets:  make(map[string]*PendingPreset),
		DB:              db,
		DBX:             dbx,
		activeScanCount: 0,
		ctx:             ctx,
		cancel:          cancel,
	}
	b.config.Store(cfg)
	b.scheduler = NewScheduler(b)
	// b.CommandHandlers = handlers.commandHandlers(b)

	go b.cleanupExpiredPresets(ctx)

	taskConfig, err := utils.LoadTaskConfig("data/task_config.json")
	if err != nil {
		log.Printf("Error loading task config, skipping tag mapping generation: %v", err)
	} else {
		err := utils.GenerateTagMappingFiles(dg, taskConfig)
		if err != nil {
			log.Printf("Error generating tag mapping files: %v", err)
		} else {
			log.Println("Successfully generated tag mapping files.")
		}
	}

	return b, nil
}

func (b *Bot) Close() {
	log.Println("Gracefully shutting down.")

	// Signal all background goroutines to stop
	b.cancel()

	if b.scheduler != nil {
		b.scheduler.Stop()
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
	registeredCmds, err := b.Session.ApplicationCommandBulkOverwrite(b.AppID, serverCfg.GuildID, cmds)
	if err != nil {
		var restErr *discordgo.RESTError
		if errors.As(err, &restErr) && restErr.Response.StatusCode == 429 {
			log.Printf("Rate limit hit for guild '%s'.", serverCfg.GuildID)
		}
		log.Printf("cannot update commands for guild '%s': %v", serverCfg.GuildID, err)
		return
	}

	log.Printf("Successfully registered %d commands for guild %s.", len(registeredCmds), serverCfg.GuildID)
	b.commandsMutex.Lock()
	b.RegisteredCommands = append(b.RegisteredCommands, registeredCmds...)
	b.commandsMutex.Unlock()
}

func (b *Bot) UnregisterCommands(guildID string) {
	log.Printf("Unregistering all commands for guild %s", guildID)
	_, err := b.Session.ApplicationCommandBulkOverwrite(b.AppID, guildID, []*discordgo.ApplicationCommand{})
	if err != nil {
		var restErr *discordgo.RESTError
		if errors.As(err, &restErr) && restErr.Response.StatusCode == 429 {
			log.Printf("Rate limit hit while unregistering commands for guild '%s'.", guildID)
		}
		log.Printf("cannot unregister commands for guild '%s': %v", guildID, err)
	}
}

func (b *Bot) GetScheduler() *Scheduler {
	return b.scheduler
}

func (b *Bot) GetPendingPresets() map[string]*PendingPreset {
	return b.PendingPresets
}

func (b *Bot) GetPendingPresetsMutex() *sync.Mutex {
	return &b.PendingPresetsMutex
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

func (b *Bot) cleanupExpiredPresets(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.PendingPresetsMutex.Lock()
			for id, preset := range b.PendingPresets {
				if time.Since(preset.Timestamp) > 5*time.Minute {
					log.Printf("Removing expired pending preset: %s", id)
					delete(b.PendingPresets, id)
				}
			}
			b.PendingPresetsMutex.Unlock()
		case <-ctx.Done():
			log.Println("Stopping expired presets cleanup.")
			return
		}
	}
}
