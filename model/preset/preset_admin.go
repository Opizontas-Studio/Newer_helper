package preset

import (
	"database/sql"
	"discord-bot/model"
	"discord-bot/utils"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func HandlePresetMessageAdminInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b interface{}) {
	type bot interface {
		GetConfig() *model.Config
		RefreshCommands(guildID string)
		GetDB() *sql.DB
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
			for _, p := range serverConfig.PresetMessages {
				if p.ID == id {
					p.Name = input
					db := appBot.GetDB()
					if err := utils.UpdatePreset(db, i.GuildID, p); err != nil {
						responseContent = "无法更新预设。"
						utils.LogError(s, appBot.GetConfig().LogChannelID, "预设管理", "更新预设失败", err.Error())
					} else {
						responseContent = "预设已重命名为 '" + input + "'。"
						logMessage := fmt.Sprintf("ID: `%s`\n新名称: `%s`\n操作者: `%s`", id, input, i.Member.User.Username)
						utils.LogInfo(s, appBot.GetConfig().LogChannelID, "预设管理", "重命名预设", logMessage)
						go appBot.RefreshCommands(i.GuildID)
					}
					found = true
					break
				}
			}
			if !found {
				responseContent = "找不到具有该 ID 的预设。"
			}
		}
	case "del":
		found := false
		for _, p := range serverConfig.PresetMessages {
			if p.ID == id {
				found = true
				break
			}
		}
		if found {
			// 发送确认消息
			confirmMessage := fmt.Sprintf("确认删除预设 %s？", id)
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: confirmMessage,
					Flags:   discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Label:    "确认",
									Style:    discordgo.DangerButton,
									CustomID: "confirm_delete_" + id,
								},
								discordgo.Button{
									Label:    "取消",
									Style:    discordgo.SecondaryButton,
									CustomID: "cancel_delete_" + id,
								},
							},
						},
					},
				},
			})
			if err != nil {
				responseContent = "无法发送确认消息。"
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: responseContent,
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			// 交互逻辑将在这里处理，现在我们只发送消息
			// 为了简单起见，我们暂时不实现等待逻辑
			// 而是假设用户会通过另一个命令来确认
			return // 我们在这里返回，因为响应将是异步的
		} else {
			responseContent = "找不到具有该 ID 的预设。"
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: responseContent,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
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
				for _, p := range serverConfig.PresetMessages {
					if p.ID == id {
						p.Value = strings.Join(messages, "\n")
						p.Type = "text" // Or parse from original message
						db := appBot.GetDB()
						if err := utils.UpdatePreset(db, i.GuildID, p); err != nil {
							responseContent = "无法更新预设。"
							utils.LogError(s, appBot.GetConfig().LogChannelID, "预设管理", "更新预设失败", err.Error())
						} else {
							responseContent = "预设已被覆盖。"
							logMessage := fmt.Sprintf("ID: `%s`\n操作者: `%s`", id, i.Member.User.Username)
							utils.LogInfo(s, appBot.GetConfig().LogChannelID, "预设管理", "覆盖预设", logMessage)
							go appBot.RefreshCommands(i.GuildID)
						}
						found = true
						break
					}
				}
				if !found {
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
