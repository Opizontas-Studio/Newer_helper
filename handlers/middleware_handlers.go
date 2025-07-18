package handlers

import (
	"discord-bot/bot"
	"discord-bot/handlers/admin"
	"discord-bot/handlers/leaderboard"
	"discord-bot/handlers/preset"
	"discord-bot/handlers/punish"
	"discord-bot/handlers/rollcard"
	"discord-bot/internal/commands"
	"discord-bot/internal/middleware"
	"discord-bot/scanner"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// RegisterMiddlewareHandlers 注册使用中间件的命令处理器
func RegisterMiddlewareHandlers(b *bot.Bot) {
	// 获取冷却服务
	cooldownService := b.GetCooldownService()
	session := b.GetSession()
	config := b.GetConfig()

	// 创建命令管理器
	manager := commands.NewManager(session, config, cooldownService)

	// 注册管理员命令
	manager.RegisterAdminCommand("punish", func(ctx *middleware.CommandContext) {
		// 转换为原始的调用方式
		punish.HandlePunishCommand(ctx.Session, ctx.Interaction, b)
	})

	manager.RegisterAdminCommand("punish_admin", func(ctx *middleware.CommandContext) {
		punish.HandlePunishAdminCommandV2(ctx.Session, ctx.Interaction, b)
	})

	manager.RegisterAdminCommand("preset-message_upd", func(ctx *middleware.CommandContext) {
		preset.HandlePresetMessageUpdateInteraction(ctx.Session, ctx.Interaction, b)
	})

	manager.RegisterAdminCommand("preset-message_admin", func(ctx *middleware.CommandContext) {
		preset.HandlePresetMessageAdminInteraction(ctx.Session, ctx.Interaction, b)
	})

	// 注册超级管理员命令
	manager.RegisterSuperAdminCommand("setup-roll-panel", func(ctx *middleware.CommandContext) {
		rollcard.HandleSetupRollPanel(ctx.Session, ctx.Interaction, b)
	})

	manager.RegisterSuperAdminCommand("register-top-channel", func(ctx *middleware.CommandContext) {
		HandleRegisterTopChannel(ctx.Session, ctx.Interaction, b)
	})

	manager.RegisterSuperAdminCommand("new-post-push_admin", func(ctx *middleware.CommandContext) {
		admin.HandleNewPostPushAdminCommand(ctx.Session, ctx.Interaction, b)
	})

	// 注册开发者命令
	manager.RegisterDeveloperCommand("start-scan", func(ctx *middleware.CommandContext) {
		options := ctx.Interaction.ApplicationCommandData().Options
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
			ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Channel cleanup started.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			go scanner.CleanAllChannels(ctx.Session, ctx.Config)
			return
		}

		ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Scan started with mode: %s. Target guild: %s", scanMode, targetGuildID),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

		go scanner.Scan(ctx.Session, ctx.Config.LogChannelID, scanMode, targetGuildID, nil)
	})

	manager.RegisterDeveloperCommand("reload-config", func(ctx *middleware.CommandContext) {
		admin.HandleReloadConfig(ctx.Session, ctx.Interaction, b)
	})

	// 注册用户命令
	manager.RegisterUserCommand("new-cards", func(ctx *middleware.CommandContext) {
		leaderboard.HandleNewCardsInteraction(ctx.Session, ctx.Interaction, b)
	})

	// 注册访客命令
	manager.RegisterGuestCommand("rollcard", func(ctx *middleware.CommandContext) {
		rollcard.HandleRollCardInteraction(ctx.Session, ctx.Interaction, b)
	})

	manager.RegisterGuestCommand("preset-message", func(ctx *middleware.CommandContext) {
		preset.HandlePresetMessageInteraction(ctx.Session, ctx.Interaction, b)
	})

	manager.RegisterGuestCommand("system-info", func(ctx *middleware.CommandContext) {
		SystemInfoHandler(ctx.Session, ctx.Interaction)
	})

	// 设置交互处理器
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			manager.HandleInteraction(s, i)
		}
	})

	log.Printf("已注册 %d 个使用中间件的命令", len(manager.GetRegisteredCommands()))
}

// CreateLegacyCommandHandlers 创建传统的命令处理器映射（向后兼容）
func CreateLegacyCommandHandlers(b *bot.Bot) map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"punish": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// 使用中间件系统的简化版本
			factory := middleware.NewFactory(
				middleware.NewDefaultLogger(),
				b.GetCooldownService(),
			)
			
			handler := middleware.NewCommandBuilder(factory).
				WithErrorHandling().
				WithLogging().
				WithPermission(middleware.RequireAdmin()).
				Build(func(ctx *middleware.CommandContext) {
					punish.HandlePunishCommand(ctx.Session, ctx.Interaction, b)
				})
			
			handler.Handle(s, i, b.GetConfig())
		},
		"rollcard": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			factory := middleware.NewFactory(
				middleware.NewDefaultLogger(),
				b.GetCooldownService(),
			)
			
			handler := middleware.NewCommandBuilder(factory).
				WithErrorHandling().
				WithLogging().
				WithCooldown().
				WithPermission(middleware.AllowGuests()).
				Build(func(ctx *middleware.CommandContext) {
					rollcard.HandleRollCardInteraction(ctx.Session, ctx.Interaction, b)
				})
			
			handler.Handle(s, i, b.GetConfig())
		},
		// 可以继续添加其他命令的中间件版本...
	}
}