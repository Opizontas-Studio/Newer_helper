package handlers

import (
	"discord-bot/bot"
	"discord-bot/handlers/preset"
	"discord-bot/handlers/rollcard"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if h, ok := b.CommandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	case discordgo.InteractionMessageComponent:
		customID := i.MessageComponentData().CustomID
		if strings.HasPrefix(customID, "confirm_delete_") || strings.HasPrefix(customID, "cancel_delete_") {
			preset.HandlePresetDeleteInteraction(s, i, b)
		} else if strings.HasPrefix(customID, "roll_again:") {
			rollcard.HandleRollCardComponent(s, i, b, customID)
		}
	case discordgo.InteractionApplicationCommandAutocomplete:
		if i.ApplicationCommandData().Name == "rollcard" {
			rollcard.HandleRollCardAutocomplete(s, i, b.Config)
		} else {
			handleAutocomplete(s, i, b.Config)
		}
	}
}
