package handlers

import (
	"discord-bot/bot"
	"discord-bot/commands"
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
			commands.HandlePresetDeleteInteraction(s, i, b)
		} else if strings.HasPrefix(customID, "roll_again:") {
			parts := strings.Split(customID, ":")
			if len(parts) >= 2 {
				poolName := parts[1]
				tagID := ""
				if len(parts) >= 3 {
					tagID = parts[2]
				}
				HandleRollAgain(s, i, b, poolName, tagID)
			}
		}
	case discordgo.InteractionApplicationCommandAutocomplete:
		handleAutocomplete(s, i, b.Config)
	}
}
