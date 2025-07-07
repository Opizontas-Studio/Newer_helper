package handlers

import (
	"discord-bot/bot"
	"discord-bot/commands"
	"discord-bot/model/preset"
	"discord-bot/utils"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func commandHandlers(b *bot.Bot) map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"rollcard": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			HandleRollCardInteraction(s, i, b)
		},
		"preset-message": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.Config.ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.Config.DeveloperUserIDs, b.Config.SuperAdminRoleIDs)
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
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.Config.DeveloperUserIDs, b.Config.SuperAdminRoleIDs)
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
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.Config.DeveloperUserIDs, b.Config.SuperAdminRoleIDs)
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
		"start_scan": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, nil, nil, b.Config.DeveloperUserIDs, nil)
			if permissionLevel != utils.DeveloperPermission {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "You do not have permission to use this command.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			scanMode := "active"
			if opt, ok := optionMap["mode"]; ok {
				scanMode = opt.StringValue()
			}

			targetGuildID := ""
			if opt, ok := optionMap["guild"]; ok {
				targetGuildID = opt.StringValue()
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Scan started with mode: %s. Target guild: %s", scanMode, targetGuildID),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			go commands.Scan(s, b.Config.LogChannelID, scanMode, targetGuildID)
		},
	}
}
