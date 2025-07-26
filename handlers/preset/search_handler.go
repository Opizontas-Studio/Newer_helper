package preset

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/yanyiwu/gojieba"
)

func HandleSearchPresetByMessage(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		utils.SendEphemeralResponse(s, i, "无法找到服务器配置。")
		return
	}

	permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
	if permissionLevel != utils.SuperAdminPermission && permissionLevel != utils.DeveloperPermission && permissionLevel != utils.AdminPermission && permissionLevel != utils.GuestPermission {
		utils.SendEphemeralResponse(s, i, "您没有权限使用此命令")
		return
	}

	err := utils.DeferResponse(s, i, true)
	if err != nil {
		log.Printf("Failed to defer response: %v", err)
		return
	}

	data := i.ApplicationCommandData()
	targetMessage, ok := data.Resolved.Messages[data.TargetID]
	if !ok {
		log.Printf("Failed to resolve target message")
		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "无法解析目标消息。",
		})
		if err != nil {
			log.Printf("Failed to send followup message: %v", err)
		}
		return
	}

	if targetMessage.Content == "" {
		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "目标消息没有文本内容。",
		})
		if err != nil {
			log.Printf("Failed to send followup message: %v", err)
		}
		return
	}

	jieba := gojieba.NewJieba()
	defer jieba.Free()

	keywords := jieba.CutForSearch(targetMessage.Content, true)

	var matchedPresets []model.PresetMessage
	for _, preset := range serverConfig.PresetMessages {
		for _, keyword := range keywords {
			if strings.Contains(strings.ToLower(preset.Name), strings.ToLower(keyword)) {
				matchedPresets = append(matchedPresets, preset)
				break
			}
		}
	}

	if len(matchedPresets) == 0 {
		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "未找到与消息内容匹配的预设。",
		})
		if err != nil {
			log.Printf("Failed to send followup message: %v", err)
		}
		return
	}

	var components []discordgo.MessageComponent
	var currentRow discordgo.ActionsRow
	for _, preset := range matchedPresets {
		if len(components) == 5 {
			break // Stop if we have already filled 5 rows
		}
		if len(currentRow.Components) == 5 {
			components = append(components, currentRow)
			currentRow = discordgo.ActionsRow{}
		}
		currentRow.Components = append(currentRow.Components, discordgo.Button{
			Label:    preset.Name,
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("search_preset_reply_%s_%s", preset.ID, data.TargetID),
		})
	}
	if len(currentRow.Components) > 0 && len(components) < 5 {
		components = append(components, currentRow)
	}

	messageContent := "找到以下匹配的预设，点击按钮即可回复："
	if len(matchedPresets) > 25 {
		messageContent = fmt.Sprintf("找到超过 25 个匹配的预设，仅显示前 %d 个。请使用更精确的关键词。", len(components)*5)
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content:    messageContent,
		Components: components,
	})
	if err != nil {
		log.Printf("Failed to send followup message with buttons: %v", err)
	}
}

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

	var selectedPreset *model.PresetMessage
	for _, p := range serverConfig.PresetMessages {
		if p.ID == presetID {
			selectedPreset = &p
			break
		}
	}

	if selectedPreset == nil {
		log.Printf("Could not find preset with ID: %s", presetID)
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
		return
	}

	// Edit the original interaction to remove the buttons
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "回复已发送。",
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		log.Printf("Failed to edit original message: %v", err)
	}
}
