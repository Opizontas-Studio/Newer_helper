package preset

import (
	"discord-bot/model"
	"discord-bot/utils"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// HandlePresetMessageAdminInteraction handles the admin interaction for preset messages.
func HandlePresetMessageAdminInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b interface{}) {
	type bot interface {
		GetConfig() *model.Config
		RefreshCommands(guildID string)
	}
	appBot := b.(bot)
	serverConfig, ok := appBot.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	id := optionMap["id"].StringValue()
	action := optionMap["action"].StringValue()
	input := ""
	if val, ok := optionMap["input"]; ok {
		input = val.StringValue()
	}

	var responseContent string

	switch action {
	case "rename":
		if input == "" {
			responseContent = "重命名操作需要 'input' 参数。"
		} else {
			found := false
			for i, p := range serverConfig.PresetMessages {
				if p.ID == id {
					serverConfig.PresetMessages[i].Name = input
					found = true
					break
				}
			}
			if found {
				appBot.GetConfig().ServerConfigs[i.GuildID] = serverConfig
				if err := model.SaveConfig(appBot.GetConfig()); err != nil {
					responseContent = "无法保存配置。"
					utils.LogError(s, appBot.GetConfig().LogChannelID, "预设管理", "保存配置失败", err.Error())
				} else {
					responseContent = "预设已重命名为 '" + input + "'。"
					logMessage := fmt.Sprintf("ID: `%s`\n新名称: `%s`\n操作者: `%s`", id, input, i.Member.User.Username)
					utils.LogInfo(s, appBot.GetConfig().LogChannelID, "预设管理", "重命名预设", logMessage)
					go appBot.RefreshCommands(i.GuildID)
				}
			} else {
				responseContent = "找不到具有该 ID 的预设。"
			}
		}
	case "del":
		found := false
		var indexToRemove = -1
		for i, p := range serverConfig.PresetMessages {
			if p.ID == id {
				indexToRemove = i
				found = true
				break
			}
		}
		if found {
			serverConfig.PresetMessages = append(serverConfig.PresetMessages[:indexToRemove], serverConfig.PresetMessages[indexToRemove+1:]...)
			appBot.GetConfig().ServerConfigs[i.GuildID] = serverConfig
			if err := model.SaveConfig(appBot.GetConfig()); err != nil {
				responseContent = "无法保存配置。"
				utils.LogError(s, appBot.GetConfig().LogChannelID, "预设管理", "保存配置失败", err.Error())
			} else {
				responseContent = "预设已被删除。"
				logMessage := fmt.Sprintf("ID: `%s`\n操作者: `%s`", id, i.Member.User.Username)
				utils.LogInfo(s, appBot.GetConfig().LogChannelID, "预设管理", "删除预设", logMessage)
				go appBot.RefreshCommands(i.GuildID)
			}
		} else {
			responseContent = "找不到具有该 ID 的预设。"
		}
	case "overwrite":
		if input == "" {
			responseContent = "覆盖操作需要 'input' 参数。"
		} else {
			messages, err := ParseMessageLinks(s, input)
			if err != nil {
				responseContent = "解析消息链接时出错: " + err.Error()
			} else if len(messages) == 0 {
				responseContent = "在输入中找不到有效的消息链接。"
			} else {
				found := false
				for i, p := range serverConfig.PresetMessages {
					if p.ID == id {
						serverConfig.PresetMessages[i].Value = strings.Join(messages, "\n")
						serverConfig.PresetMessages[i].Type = "text" // Or parse from original message
						found = true
						break
					}
				}
				if found {
					appBot.GetConfig().ServerConfigs[i.GuildID] = serverConfig
					if err := model.SaveConfig(appBot.GetConfig()); err != nil {
						responseContent = "无法保存配置。"
						utils.LogError(s, appBot.GetConfig().LogChannelID, "预设管理", "保存配置失败", err.Error())
					} else {
						responseContent = "预设已被覆盖。"
						logMessage := fmt.Sprintf("ID: `%s`\n操作者: `%s`", id, i.Member.User.Username)
						utils.LogInfo(s, appBot.GetConfig().LogChannelID, "预设管理", "覆盖预设", logMessage)
						go appBot.RefreshCommands(i.GuildID)
					}
				} else {
					responseContent = "找不到具有该 ID 的预设。"
				}
			}
		}
	default:
		responseContent = "未知的操作。"
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: responseContent,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
