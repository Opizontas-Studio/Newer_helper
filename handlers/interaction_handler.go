package handlers

import (
	"discord-bot/bot"
	"discord-bot/handlers/preset"
	"discord-bot/handlers/punish"
	"discord-bot/handlers/rollcard"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		switch i.ApplicationCommandData().Name {
		case "快速处罚":
			punish.HandleQuickPunishCommand(s, i, b)
		case "以消息文本搜索预设":
			preset.HandleSearchPresetByMessage(s, i, b)
		default:
			if h, ok := b.CommandHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i)
			}
		}
	case discordgo.InteractionMessageComponent:
		customID := i.MessageComponentData().CustomID
		if strings.HasPrefix(customID, "confirm_delete_") || strings.HasPrefix(customID, "cancel_delete_") {
			preset.HandlePresetDeleteInteraction(s, i, b)
		} else if strings.HasPrefix(customID, "confirm_preset_") || strings.HasPrefix(customID, "cancel_preset_") || strings.HasPrefix(customID, "disable_confirm_preset_") {
			preset.HandlePresetConfirmationInteraction(s, i, b)
		} else if strings.HasPrefix(customID, "search_preset_reply_") {
			preset.HandleSearchPresetReply(s, i, b)
		} else if strings.HasPrefix(customID, "punish_page_v2:") {
			punish.HandlePunishPaginationV2(s, i, b)
		} else if strings.HasPrefix(customID, "roll_again:") {
			rollcard.HandleRollCardComponent(s, i, b, customID)
		} else if strings.HasPrefix(customID, "persistent_roll:") {
			rollcard.HandlePersistentRoll(s, i, b, customID)
		} else if strings.HasPrefix(customID, "global_roll:") {
			rollcard.HandleGlobalRoll(s, i, b, customID)
		} else if strings.HasPrefix(customID, "custom_roll:") {
			rollcard.HandleCustomRoll(s, i, b, customID)
		} else if customID == "edit_my_pools" {
			rollcard.HandleEditPools(s, i, b)
		} else if customID == "select_pools_menu" {
			rollcard.HandlePoolSelectionResponse(s, i, b)
		}
	case discordgo.InteractionModalSubmit:
		customID := i.ModalSubmitData().CustomID
		if strings.HasPrefix(customID, "punish_modal_") {
			punish.HandlePunishModalSubmit(s, i, b)
		}
	case discordgo.InteractionApplicationCommandAutocomplete:
		if i.ApplicationCommandData().Name == "rollcard" {
			rollcard.HandleRollCardAutocomplete(s, i, b.GetConfig())
		} else {
			handleAutocomplete(s, i, b.GetConfig())
		}
	}
}
