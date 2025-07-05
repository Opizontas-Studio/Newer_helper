package main

import (
	"discord-bot/model"
	"discord-bot/model/preset"
	"discord-bot/utils"
	"log"

	"github.com/bwmarrin/discordgo"
)

func GetCommandHandlers(b *Bot) map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

func GenerateCommands(serverCfg *model.ServerConfig) []*discordgo.ApplicationCommand {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(serverCfg.PresetMessages))
	for _, p := range serverCfg.PresetMessages {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  p.Name,
			Value: p.Name,
		})
	}

	return []*discordgo.ApplicationCommand{
		{
			Name:        "preset-message",
			Description: "发送预设消息并提及一位成员。",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "preset",
					Description: "要发送的预设消息。",
					Required:    true,
					Choices:     choices,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "要提及的用户。",
					Required:    false,
				},
			},
		},
		{
			Name:        "preset-message_upd",
			Description: "从消息链接中解析和创建预设",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "messagelinks",
					Description: "要解析的消息链接。",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "为预设指定一个自定义名称。",
					Required:    true,
				},
			},
		},
		{
			Name:        "preset-message_admin",
			Description: "管理预设消息",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "要管理的预设名称",
					Required:    true,
					Choices:     choices,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "要执行的操作",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "重命名", Value: "rename"},
						{Name: "删除", Value: "del"},
						{Name: "覆盖", Value: "overwrite"},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "input",
					Description: "重命名或覆盖的新内容",
					Required:    false,
				},
			},
		},
	}
}
