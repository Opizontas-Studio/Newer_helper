package handlers

import (
	"context"
	"discord-bot/bot"
	"discord-bot/handlers/admin"
	"discord-bot/handlers/auto_trigger"
	"discord-bot/handlers/leaderboard"
	"discord-bot/handlers/preset"
	"discord-bot/handlers/punish"
	punish_admin "discord-bot/handlers/punish/admin"
	"discord-bot/handlers/rollcard"
	"discord-bot/scanner"
	"discord-bot/utils"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func commandHandlers(b *bot.Bot) map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"punish": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel != utils.AdminPermission && permissionLevel != utils.SuperAdminPermission && permissionLevel != utils.DeveloperPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
				return
			}
			punish.HandlePunishCommand(s, i, b)
		},
		"punish_search": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel < utils.AdminPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
				return
			}
			punish_admin.HandlePunishSearchCommand(s, i)
		},
		"punish_revoke": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel < utils.AdminPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
				return
			}
			punish_admin.HandlePunishRevokeCommand(s, i)
		},
		"punish_delete": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel < utils.AdminPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
				return
			}
			punish_admin.HandlePunishDeleteCommand(s, i)
		},
		"punish_print_evidence": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel < utils.AdminPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
				return
			}
			punish_admin.HandlePunishPrintEvidenceCommand(s, i)
		},
		"reset_punish_cooldown": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel < utils.AdminPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
				return
			}
			punish.HandleResetPunishCooldownCommand(s, i, b)
		},
		"new-cards": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			leaderboard.HandleNewCardsInteraction(s, i, b)
		},
		"ads_board_admin": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			config := b.GetConfig()
			serverConfig, ok := config.ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Guild config not found for guild ID: %s", i.GuildID)
				utils.SendEphemeralResponse(s, i, "服务器配置未找到")
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, config.DeveloperUserIDs, config.SuperAdminRoleIDs)
			if permissionLevel != utils.SuperAdminPermission && permissionLevel != utils.DeveloperPermission && permissionLevel != utils.AdminPermission {
				utils.SendEphemeralResponse(s, i, "您没有权限使用此命令")
				return
			}
			leaderboard.HandleAdsBoardAdminCommand(s, i, b)
		},
		"rollcard": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			rollcard.HandleRollCardInteraction(s, i, b)
		},
		"preset-message": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
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
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
				return
			}
			preset.HandlePresetMessageAdminInteraction(s, i, b)
		},
		"quick-preset": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel == utils.GuestPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
				return
			}
			preset.HandleQuickPresetInteraction(s, i, b)
		},
		"start-scan": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, nil, nil, b.GetConfig().DeveloperUserIDs, nil)
			if permissionLevel != utils.DeveloperPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
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

			if scanMode == "clean" {
				utils.SendEphemeralResponse(s, i, "Channel cleanup started.")
				go scanner.CleanAllChannels(s, b.GetConfig())
				return
			}

			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Scan started with mode: %s. Target guild: %s", scanMode, targetGuildID))

			go scanner.Scan(s, b.GetConfig().LogChannelID, scanMode, targetGuildID, context.Background())
		},
		"setup-roll-panel": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel != utils.DeveloperPermission && permissionLevel != utils.SuperAdminPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
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
		"register-top-channel": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel != utils.SuperAdminPermission && permissionLevel != utils.DeveloperPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
				return
			}
			HandleRegisterTopChannel(s, i, b)
		},
		"daily_punishment_stats": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
			if permissionLevel != utils.SuperAdminPermission && permissionLevel != utils.DeveloperPermission {
				utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
				return
			}
			punish.HandlePunishmentStatsCommand(s, i, b)
		},
		"manage-auto-trigger": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			auto_trigger.HandleManageAutoTriggerCommand(s, i, b)
		},
		"guilds_admin": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			admin.HandleGuildsAdminCommand(s, i, b.GetDB(), b.GetConfig())
		},
	}
}
