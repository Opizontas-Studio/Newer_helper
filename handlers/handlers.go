package handlers

import (
	"discord-bot/bot"
	"discord-bot/commands"
	"discord-bot/model/preset"
	"discord-bot/utils"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func Register(b *bot.Bot) {
	b.CommandHandlers = commandHandlers(b)
	addHandlers(b)
}

func commandHandlers(b *bot.Bot) map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"preset-message": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.Config.ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs)
			if permissionLevel == utils.GuestPermission {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "You do not have permission to use this command.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			preset.HandlePresetMessageInteraction(s, i, b)
		},
		"preset-message_upd": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.Config.ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs)
			if permissionLevel != utils.AdminPermission {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "You do not have permission to use this command.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			preset.HandlePresetMessageUpdateInteraction(s, i, b)
		},
		"preset-message_admin": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.Config.ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs)
			if permissionLevel != utils.AdminPermission {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "You do not have permission to use this command.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			preset.HandlePresetMessageAdminInteraction(s, i, b)
		},
	}
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
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			if h, ok := b.CommandHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i)
			}
		case discordgo.InteractionMessageComponent:
			if strings.HasPrefix(i.MessageComponentData().CustomID, "confirm_delete_") || strings.HasPrefix(i.MessageComponentData().CustomID, "cancel_delete_") {
				commands.HandlePresetDeleteInteraction(s, i, b)
			}
		}
	})
	b.Session.AddHandler(func(s *discordgo.Session, t *discordgo.ThreadCreate) {
		// Pass the bot's config to the handler
		commands.HandleThreadCreate(s, t, b.Config)
	})
}
