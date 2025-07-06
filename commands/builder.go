package commands

import (
	"discord-bot/model"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func GenerateCommands(serverCfg *model.ServerConfig) []*discordgo.ApplicationCommand {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(serverCfg.PresetMessages))
	for _, p := range serverCfg.PresetMessages {
		name := p.Name
		if len(name) > 80 {
			name = name[:80]
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("(%s) %s", p.ID[:4], name),
			Value: p.ID,
		})
	}

	return []*discordgo.ApplicationCommand{
		{
			Name:        "preset-message",
			Description: "发送预设消息并提及一位成员。",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "id",
					Description: "要发送的预设消息 ID。",
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
					Name:        "id",
					Description: "要管理的预设 ID",
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
