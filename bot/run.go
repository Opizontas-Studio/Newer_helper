package bot

import (
	"discord-bot/utils"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) Run() {
	err := b.Session.Open()
	if err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}

	log.Println("Unregistering all commands from all guilds...")
	guilds, err := b.Session.UserGuilds(100, "", "", true)
	if err != nil {
		log.Printf("Could not fetch guilds: %v", err)
	} else {
		for _, guild := range guilds {
			b.UnregisterCommands(guild.ID)
		}
	}

	log.Println("Registering commands for enabled guilds...")
	b.RegisteredCommands = make([]*discordgo.ApplicationCommand, 0)
	for _, serverCfg := range b.GetConfig().ServerConfigs {
		if serverCfg.Enable {
			b.RefreshCommands(serverCfg.GuildID)
		}
	}

	// Start the scheduler
	b.GetScheduler().Start()

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	utils.LogInfo(b.Session, b.GetConfig().LogChannelID, "System", "Startup", "Bot has started successfully.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
