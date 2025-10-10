package preset

import (
	"fmt"
	"log"
	"newer_helper/bot"
	"newer_helper/model"
	"newer_helper/utils"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// HandleSearchPresetCommand handles the context menu command for searching presets.
// It responds with a modal for the user to enter a search keyword.
func HandleSearchPresetCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	targetMessageID := i.ApplicationCommandData().TargetID

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("search_preset_modal_%s", targetMessageID),
			Title:    "搜索预设",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "search_keyword",
							Label:       "关键词",
							Style:       discordgo.TextInputShort,
							Placeholder: "输入要搜索的预设名称",
							Required:    true,
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Failed to show modal: %v", err)
	}
}

// HandleSearchPresetModal handles the submission of the search preset modal.
func HandleSearchPresetModal(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	parts := strings.Split(i.ModalSubmitData().CustomID, "_")
	if len(parts) != 4 {
		log.Printf("Invalid custom id for preset search modal: %s", i.ModalSubmitData().CustomID)
		return
	}
	messageID := parts[3]

	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		utils.SendEphemeralResponse(s, i, "无法找到服务器配置。")
		return
	}

	keyword := i.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	if keyword == "" {
		utils.SendEphemeralResponse(s, i, "关键词不能为空。")
		return
	}

	var matchedPresets []model.PresetMessage
	for _, preset := range serverConfig.PresetMessages {
		if strings.Contains(strings.ToLower(preset.Name), strings.ToLower(keyword)) {
			matchedPresets = append(matchedPresets, preset)
		}
	}

	if len(matchedPresets) == 0 {
		utils.SendEphemeralResponse(s, i, "未找到匹配的预设。")
		return
	}

	var components []discordgo.MessageComponent
	var currentRow discordgo.ActionsRow
	for idx, preset := range matchedPresets {
		if len(components) == 4 && len(currentRow.Components) == 5 { // Leave space for the last row with the again button
			break
		}
		if len(currentRow.Components) == 5 {
			components = append(components, currentRow)
			currentRow = discordgo.ActionsRow{}
		}
		button := discordgo.Button{
			Label:    preset.Name,
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("search_preset_reply_%s_%s", preset.ID, messageID),
		}
		currentRow.Components = append(currentRow.Components, button)

		if idx == len(matchedPresets)-1 {
			components = append(components, currentRow)
		}
	}

	// Add the "Search Again" button
	if len(components) < 5 {
		if len(components) == 0 {
			components = append(components, discordgo.ActionsRow{})
		}
		lastRow := components[len(components)-1].(discordgo.ActionsRow)
		if len(lastRow.Components) < 5 {
			lastRow.Components = append(lastRow.Components, discordgo.Button{
				Label:    "重新搜索",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("search_preset_again_%s", messageID),
			})
			components[len(components)-1] = lastRow
		} else {
			components = append(components, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "重新搜索",
						Style:    discordgo.SecondaryButton,
						CustomID: fmt.Sprintf("search_preset_again_%s", messageID),
					},
				},
			})
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:       "预设搜索结果",
		Description: fmt.Sprintf("为关键词“%s”找到 %d 个匹配的预设。", keyword, len(matchedPresets)),
		Color:       0x00ff00, // Green
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Failed to send followup message with buttons: %v", err)
	}
}

// HandleSearchPresetAgain handles the "search again" button press.
func HandleSearchPresetAgain(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	parts := strings.Split(i.MessageComponentData().CustomID, "_")
	if len(parts) != 4 {
		log.Printf("Invalid custom id for search again: %s", i.MessageComponentData().CustomID)
		return
	}
	messageID := parts[3]

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("search_preset_modal_%s", messageID),
			Title:    "重新搜索预设",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "search_keyword",
							Label:       "关键词",
							Style:       discordgo.TextInputShort,
							Placeholder: "输入新的搜索关键词",
							Required:    true,
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Failed to show modal for search again: %v", err)
	}
}

// HandleSearchPresetReply handles the reply from a searched preset button.
func HandleSearchPresetReply(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	parts := strings.Split(i.MessageComponentData().CustomID, "_")
	if len(parts) != 5 {
		log.Printf("Invalid custom id for preset search reply: %s", i.MessageComponentData().CustomID)
		return
	}
	presetID := parts[3]
	messageID := parts[4]

	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		return
	}

	selectedPreset := FindPreset(&serverConfig, presetID)
	if selectedPreset == nil {
		log.Printf("Could not find preset with ID: %s", presetID)
		utils.SendEphemeralResponse(s, i, "找不到选择的预设。")
		return
	}

	messageSend := FormatPresetMessageSend(selectedPreset, "")
	messageSend.Reference = &discordgo.MessageReference{
		MessageID: messageID,
		ChannelID: i.ChannelID,
		GuildID:   i.GuildID,
	}

	_, err := s.ChannelMessageSendComplex(i.ChannelID, messageSend)
	if err != nil {
		log.Printf("Failed to send preset reply: %v", err)
		utils.SendEphemeralResponse(s, i, "发送回复失败。")
		return
	}

	// Edit the original interaction to remove the buttons and show a confirmation.
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "回复已发送。",
			Components: []discordgo.MessageComponent{},
			Embeds:     []*discordgo.MessageEmbed{},
		},
	})
	if err != nil {
		log.Printf("Failed to edit original message: %v", err)
	}
}
