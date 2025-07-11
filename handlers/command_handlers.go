package handlers

import (
	"discord-bot/bot"
	"discord-bot/handlers/admin"
	"discord-bot/handlers/leaderboard"
	"discord-bot/handlers/preset"
	"discord-bot/handlers/rollcard"
	"discord-bot/scanner"
	"discord-bot/utils"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func commandHandlers(b *bot.Bot) map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"new-cards": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			leaderboard.HandleNewCardsInteraction(s, i, b)
		},
		"rollcard": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			rollcard.HandleRollCardInteraction(s, i, b)
		},
		"preset-message": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
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
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
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
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
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
		"start-scan": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, nil, nil, b.GetConfig().DeveloperUserIDs, nil)
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

			go scanner.Scan(s, b.GetConfig().LogChannelID, scanMode, targetGuildID, nil)
		},
		"setup-roll-panel": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel != utils.DeveloperPermission && permissionLevel != utils.SuperAdminPermission {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "You do not have permission to use this command.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			rollcard.HandleSetupRollPanel(s, i, b)
		},
		"system-info": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			SystemInfoHandler(s, i)
		},
		"reload-config": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			admin.HandleReloadConfig(s, i, b)
		},
		"new-post-push_admin": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			admin.HandleNewPostPushAdminCommand(s, i, b)
		},
	}
}
