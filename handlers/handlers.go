package handlers

import (
	"discord-bot/bot"
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
	})

	b.Session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handleInteractionCreate(s, i, b)
	})

	b.Session.AddHandler(func(s *discordgo.Session, t *discordgo.ThreadCreate) {
		// Pass the bot's config to the handler
		HandleThreadCreate(s, t, b.GetConfig())
	})

	b.Session.AddHandler(func(s *discordgo.Session, t *discordgo.ThreadDelete) {
		HandleThreadDelete(s, t, b.GetConfig())
	})

	b.Session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		HandleMessageCreate(s, m, b)
	})
}
