package handlers

import (
	"discord-bot/bot"
	"log"

	"github.com/bwmarrin/discordgo"
)

func Register(b *bot.Bot) {
	// TODO: This is a temporary bridge to use the new middleware system
	// In the complete integration phase, we will remove the old commandHandlers entirely
	// and use the new middleware system directly in the interaction handler
	b.SetCommandHandlers(CreateLegacyCommandHandlers(b))
	addHandlers(b)
}

func addHandlers(b *bot.Bot) {
	session := b.GetSession()

	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})

	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handleInteractionCreate(s, i, b)
	})

	session.AddHandler(func(s *discordgo.Session, t *discordgo.ThreadCreate) {
		// Pass the bot's config to the handler
		HandleThreadCreate(s, t, b.GetConfig())
	})

	session.AddHandler(func(s *discordgo.Session, t *discordgo.ThreadDelete) {
		HandleThreadDelete(s, t, b.GetConfig())
	})

	session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		HandleMessageCreate(s, m, b)
	})
}
