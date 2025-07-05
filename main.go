package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"

	"discord-bot/model"
)

type Bot struct {
	Session            *discordgo.Session
	registeredCommands []*discordgo.ApplicationCommand
	Config             *model.Config
	CommandHandlers    map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

func (b *Bot) GetConfig() *model.Config {
	return b.Config
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
		log.Panicf("cannot get commands for guild '%s': %v", serverCfg.GuildID, err)
	}

	if len(existingCommands) > 0 {
		log.Printf("Removing %d old commands for guild %s...", len(existingCommands), serverCfg.GuildID)
		for _, cmd := range existingCommands {
			err := b.Session.ApplicationCommandDelete(b.Session.State.User.ID, serverCfg.GuildID, cmd.ID)
			if err != nil {
				log.Panicf("cannot delete '%v' command for guild '%s': %v", cmd.Name, serverCfg.GuildID, err)
			}
		}
	}

	commands := GenerateCommands(&serverCfg)
	for _, v := range commands {
		cmd, err := b.Session.ApplicationCommandCreate(b.Session.State.User.ID, serverCfg.GuildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command for guild '%s': %v", v.Name, serverCfg.GuildID, err)
		}
		b.registeredCommands = append(b.registeredCommands, cmd)
	}
}

func NewBot(cfg *model.Config) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, err
	}

	bot := &Bot{Session: dg, Config: cfg}
	bot.CommandHandlers = GetCommandHandlers(bot)
	bot.addHandlers()

	return bot, nil
}

func (b *Bot) addHandlers() {
	b.Session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})
	b.Session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := b.CommandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
}

func (b *Bot) Run() {
	err := b.Session.Open()
	if err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}

	log.Println("Adding commands...")
	b.registeredCommands = make([]*discordgo.ApplicationCommand, 0)
	for _, serverCfg := range b.Config.ServerConfigs {
		b.RefreshCommands(serverCfg.GuildID)
	}

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
			log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
		}
	}

	log.Println("Gracefully shutting down.")
	b.Session.Close()
}

func main() {
	cfg := LoadConfig()
	bot, err := NewBot(cfg)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	bot.Run()
}
