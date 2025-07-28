package bot

import (
	"discord-bot/utils"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) Run() error {
	err := b.Session.Open()
	if err != nil {
		log.Printf("Error opening connection: %v", err)
		return err
	}

	if !b.GetConfig().DisableCommandUnregister {
		log.Println("Unregistering all commands from all guilds...")
		guilds, err := b.Session.UserGuilds(100, "", "", true)
		if err != nil {
			log.Printf("Could not fetch guilds: %v", err)
		} else {
			for _, guild := range guilds {
				b.UnregisterCommands(guild.ID)
				time.Sleep(1 * time.Second)
			}
		}
	} else {
		log.Println("Skipping command unregistering as per environment configuration.")
	}

	log.Println("Registering commands for enabled guilds...")
	b.RegisteredCommands = make([]*discordgo.ApplicationCommand, 0)

	for _, serverCfg := range b.GetConfig().ServerConfigs {
		if serverCfg.Enable {
			b.RefreshCommands(serverCfg.GuildID)
			time.Sleep(1 * time.Second)
		}
	}

	// Start the scheduler
	b.GetScheduler().Start()

	fmt.Println("Bot is now running.")
	utils.LogInfo(b.Session, b.GetConfig().LogChannelID, "System", "Startup", "Bot has started successfully.")
	return nil
}
