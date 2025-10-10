package preset

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"newer_helper/bot"
	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func HandlePresetMessageAdminInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
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
			responseContent = "重命名操作需要 'input' 参数 "
		} else {
			found := false
			for idx, p := range serverConfig.PresetMessages {
				if p.ID == id {
					serverConfig.PresetMessages[idx].Name = input
					db := b.DB
					if err := database.UpdatePreset(db, i.GuildID, serverConfig.PresetMessages[idx]); err != nil {
						responseContent = "无法更新预设 "
						utils.LogError(s, b.GetConfig().LogChannelID, "预设管理", "更新预设失败", err.Error())
					} else {
						responseContent = "预设已重命名为 '" + input + "' "
						logMessage := fmt.Sprintf("ID: `%s`\n新名称: `%s`\n操作者: `%s`", id, input, i.Member.User.Username)
						utils.LogInfo(s, b.GetConfig().LogChannelID, "预设管理", "重命名预设", logMessage)
						go b.RefreshCommands(i.GuildID)
					}
					found = true
					break
				}
			}
			if !found {
				responseContent = "找不到具有该 ID 的预设 "
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
				responseContent = "无法发送确认消息 "
				utils.SendEphemeralResponse(s, i, responseContent)
				return
			}
			return // The response will be handled asynchronously
		} else {
			responseContent = "找不到具有该 ID 的预设 "
			utils.SendEphemeralResponse(s, i, responseContent)
		}
	case "overwrite":
		if input == "" {
			responseContent = "覆盖操作需要 'input' 参数 "
		} else {
			parsedMessages, err := utils.ParseMessageLinks(s, input)
			if err != nil {
				responseContent = "解析消息链接时出错: " + err.Error()
			} else if len(parsedMessages) == 0 {
				responseContent = "在输入中找不到有效的消息链接 "
			} else {
				var messageContents []string
				for _, msg := range parsedMessages {
					messageContents = append(messageContents, msg.Content)
				}

				found := false
				for idx, p := range serverConfig.PresetMessages {
					if p.ID == id {
						serverConfig.PresetMessages[idx].Value = strings.Join(messageContents, "\n")
						serverConfig.PresetMessages[idx].Type = "text" // Or parse from original message
						db := b.DB
						if err := database.UpdatePreset(db, i.GuildID, serverConfig.PresetMessages[idx]); err != nil {
							responseContent = "无法更新预设 "
							utils.LogError(s, b.GetConfig().LogChannelID, "预设管理", "更新预设失败", err.Error())
						} else {
							responseContent = "预设已被覆盖 "
							logMessage := fmt.Sprintf("ID: `%s`\n操作者: `%s`", id, i.Member.User.Username)
							utils.LogInfo(s, b.GetConfig().LogChannelID, "预设管理", "覆盖预设", logMessage)
							go b.RefreshCommands(i.GuildID)
						}
						found = true
						break
					}
				}
				if !found {
					responseContent = "找不到具有该 ID 的预设 "
				}
			}
		}
	default:
		responseContent = "未知的操作 "
	}

	utils.SendEphemeralResponse(s, i, responseContent)
}

func HandlePresetMessageUpdateInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	if err := utils.DeferResponse(s, i, true); err != nil {
		log.Printf("Error sending deferred response: %v", err)
		return
	}

	go func() {
		serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
		if !ok {
			log.Printf("Could not find server config for guild: %s", i.GuildID)
			return
		}

		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		var messageLinks string
		if option, ok := optionMap["messagelinks"]; ok {
			messageLinks = option.StringValue()
		}

		var customName string
		if option, ok := optionMap["name"]; ok {
			customName = option.StringValue()
		}

		parsedMessages, err := utils.ParseMessageLinks(s, messageLinks)
		if err != nil {
			utils.SendFollowUpError(s, i.Interaction, err.Error())
			return
		}

		var presetName string
		if len(parsedMessages) > 0 {
			var messageContents []string
			for _, msg := range parsedMessages {
				messageContents = append(messageContents, msg.Content)
			}

			presetName = customName
			if presetName == "" {
				presetName = fmt.Sprintf("New Preset %d", len(serverConfig.PresetMessages)+1)
			}
			newPreset := model.PresetMessage{
				ID:    generateUniqueID(),
				Name:  presetName,
				Value: strings.Join(messageContents, "\n"),
				Type:  "text",
			}
			serverConfig.PresetMessages = append(serverConfig.PresetMessages, newPreset)
			b.GetConfig().ServerConfigs[i.GuildID] = serverConfig

			db := b.DB
			if err := database.AddPreset(db, i.GuildID, newPreset); err != nil {
				log.Printf("Error saving preset: %v", err)
				utils.SendFollowUpError(s, i.Interaction, "Error processing preset: could not save to database.")
				return
			}

			// Log the successful preset update
			channelLink := fmt.Sprintf("https://discord.com/channels/%s/%s", i.GuildID, i.ChannelID)
			logInfo := fmt.Sprintf("用户 <@%s> 创建了新的预设 `%s`\n[在频道中查看](%s)", i.Member.User.Username, presetName, channelLink)
			if b.GetConfig().LogChannelID != "" {
				err = utils.LogInfo(s, b.GetConfig().LogChannelID, "预设", "创建/更新", logInfo)
				if err != nil {
					log.Printf("Failed to send log: %v", err)
				}
			}
		}

		b.RefreshCommands(i.GuildID)

		var webhookEdit discordgo.WebhookEdit
		if len(parsedMessages) == 0 {
			response := "未找到或解析任何消息链接 没有预设被创建或更新 "
			webhookEdit = discordgo.WebhookEdit{
				Content: &response,
			}
		} else {
			var messageContents []string
			for _, msg := range parsedMessages {
				messageContents = append(messageContents, msg.Content)
			}
			description := fmt.Sprintf(
				"已成功为您保存预设 `%s` \n\n**预设内容预览:**\n```\n%s\n```",
				presetName,
				strings.Join(messageContents, "\n---\n"),
			)
			embed := &discordgo.MessageEmbed{
				Title:       "✅ 预设创建/更新成功",
				Description: description,
				Color:       0x57F287, // Green
				Footer: &discordgo.MessageEmbedFooter{
					Text: fmt.Sprintf("由 %s 操作", i.Member.User.Username),
				},
			}
			webhookEdit = discordgo.WebhookEdit{
				Embeds: &[]*discordgo.MessageEmbed{embed},
			}
		}

		s.InteractionResponseEdit(i.Interaction, &webhookEdit)
	}()
}

func generateUniqueID() string {
	bytes := make([]byte, 8) // 16 characters
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%x", os.Getpid())
	}
	return hex.EncodeToString(bytes)
}
