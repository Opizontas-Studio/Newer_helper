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

	log.Println("Adding commands...")
	b.RegisteredCommands = make([]*discordgo.ApplicationCommand, 0)
	for _, serverCfg := range b.GetConfig().ServerConfigs {
		b.RefreshCommands(serverCfg.GuildID)
	}

	// Start the scheduler
	b.GetScheduler().Start()

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	utils.LogInfo(b.Session, b.GetConfig().LogChannelID, "System", "Startup", "Bot has started successfully.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
