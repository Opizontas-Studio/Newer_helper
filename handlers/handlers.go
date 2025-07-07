package handlers

import (
	"discord-bot/bot"
	"discord-bot/commands"
	"discord-bot/utils"
	"log"

	"github.com/bwmarrin/discordgo"
)

func Register(b *bot.Bot) {
	b.CommandHandlers = commandHandlers(b)
	addHandlers(b)
}

func addHandlers(b *bot.Bot) {
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
		handleInteractionCreate(s, i, b)
	})

	b.Session.AddHandler(func(s *discordgo.Session, t *discordgo.ThreadCreate) {
		// Pass the bot's config to the handler
		commands.HandleThreadCreate(s, t, b.Config)
	})
}
