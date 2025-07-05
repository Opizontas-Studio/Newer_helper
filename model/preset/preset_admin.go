package preset

import (
	"discord-bot/model"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// HandlePresetMessageAdminInteraction handles the admin interaction for preset messages.
func HandlePresetMessageAdminInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b interface{}) {
	type bot interface {
		GetConfig() *model.Config
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

	name := optionMap["name"].StringValue()
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
				if p.Name == name {
					serverConfig.PresetMessages[i].Name = input
					found = true
					break
				}
			}
			if found {
				if err := model.SaveConfig(appBot.GetConfig()); err != nil {
					responseContent = "无法保存配置。"
					log.Printf("Failed to save config: %v", err)
				} else {
					responseContent = "预设 '" + name + "' 已重命名为 '" + input + "'。"
				}
			} else {
				responseContent = "找不到名为 '" + name + "' 的预设。"
			}
		}
	case "del":
		found := false
		var indexToRemove = -1
		for i, p := range serverConfig.PresetMessages {
			if p.Name == name {
				indexToRemove = i
				found = true
				break
			}
		}
		if found {
			serverConfig.PresetMessages = append(serverConfig.PresetMessages[:indexToRemove], serverConfig.PresetMessages[indexToRemove+1:]...)
			if err := model.SaveConfig(appBot.GetConfig()); err != nil {
				responseContent = "无法保存配置。"
				log.Printf("Failed to save config: %v", err)
			} else {
				responseContent = "预设 '" + name + "' 已被删除。"
			}
		} else {
			responseContent = "找不到名为 '" + name + "' 的预设。"
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
					if p.Name == name {
						serverConfig.PresetMessages[i].Value = strings.Join(messages, "\n")
						serverConfig.PresetMessages[i].Type = "text" // Or parse from original message
						found = true
						break
					}
				}
				if found {
					if err := model.SaveConfig(appBot.GetConfig()); err != nil {
						responseContent = "无法保存配置。"
						log.Printf("Failed to save config: %v", err)
					} else {
						responseContent = "预设 '" + name + "' 已被覆盖。"
					}
				} else {
					responseContent = "找不到名为 '" + name + "' 的预设。"
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
