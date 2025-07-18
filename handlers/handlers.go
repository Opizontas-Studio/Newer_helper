package handlers

import (
	"discord-bot/bot"
	"discord-bot/handlers/preset"
	"discord-bot/handlers/punish"
	"discord-bot/handlers/rollcard"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func Register(b *bot.Bot) {
	// TODO: This is a temporary bridge to use the new middleware system
	// In the complete integration phase, we will remove the old commandHandlers entirely
	// and use the new middleware system directly in the interaction handler
	b.SetCommandHandlers(CreateLegacyCommandHandlers(b))
	addHandlers(b)
}

// RegisterNonInteractionHandlers 注册非交互处理器（新系统）
func RegisterNonInteractionHandlers(b *bot.Bot) {
	session := b.GetSession()

	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
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

	// 处理非命令交互（组件、分页等）
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionMessageComponent:
			handleMessageComponent(s, i, b)
		case discordgo.InteractionApplicationCommandAutocomplete:
			handleAutocompleteInteraction(s, i, b)
		}
	})
}

// handleMessageComponent 处理消息组件交互
func handleMessageComponent(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	customID := i.MessageComponentData().CustomID
	if strings.HasPrefix(customID, "confirm_delete_") || strings.HasPrefix(customID, "cancel_delete_") {
		preset.HandlePresetDeleteInteraction(s, i, b)
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
}

// handleAutocompleteInteraction 处理自动完成交互
func handleAutocompleteInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	if i.ApplicationCommandData().Name == "rollcard" {
		rollcard.HandleRollCardAutocomplete(s, i, b.GetConfig())
	} else {
		handleAutocomplete(s, i, b.GetConfig())
	}
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
