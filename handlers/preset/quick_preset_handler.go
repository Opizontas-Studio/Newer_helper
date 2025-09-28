package preset

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// HandleQuickPresetInteraction handles the /quick-preset command.
func HandleQuickPresetInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	if err := utils.DeferResponse(s, i, true); err != nil {
		log.Printf("Failed to defer interaction response for quick-preset: %v", err)
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	userID := i.Member.User.ID
	guildID := i.GuildID

	action := ""
	if opt, ok := optionMap["action"]; ok {
		action = opt.StringValue()
	}

	slot := 1 // Default slot
	if opt, ok := optionMap["slot"]; ok {
		slot = int(opt.IntValue())
	}

	presetID := ""
	if opt, ok := optionMap["preset_id"]; ok {
		presetID = opt.StringValue()
	}

	switch action {
	case "add":
		handleAddOrReplaceQuickPreset(s, i, userID, guildID, slot, presetID)
	case "remove":
		handleRemoveQuickPreset(s, i, userID, guildID, slot)
	case "show":
		handleShowQuickPresets(s, i, userID, guildID, b)
	default:
		handleSendQuickPreset(s, i, b, userID, guildID, slot)
	}
}

func sendEphemeralFollowUp(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Printf("Failed to send ephemeral followup message: %v", err)
	}
}

func handleAddOrReplaceQuickPreset(s *discordgo.Session, i *discordgo.InteractionCreate, userID, guildID string, slot int, presetID string) {
	if presetID == "" {
		sendEphemeralFollowUp(s, i, "使用 'add' 操作时，必须提供 'preset_id'。")
		return
	}

	err := database.SetUserQuickPreset(userID, guildID, slot, presetID)
	if err != nil {
		log.Printf("Failed to set quick preset for user %s in guild %s: %v", userID, guildID, err)
		sendEphemeralFollowUp(s, i, "设置快速预设失败。")
		return
	}

	content := fmt.Sprintf("成功将预设 `%s` 添加/替换到槽位 %d。", presetID, slot)
	sendEphemeralFollowUp(s, i, content)
}

func handleRemoveQuickPreset(s *discordgo.Session, i *discordgo.InteractionCreate, userID, guildID string, slot int) {
	err := database.RemoveUserQuickPreset(userID, guildID, slot)
	if err != nil {
		log.Printf("Failed to remove quick preset for user %s in guild %s: %v", userID, guildID, err)
		sendEphemeralFollowUp(s, i, "移除快速预设失败。")
		return
	}

	content := fmt.Sprintf("成功从槽位 %d 移除快速预设。", slot)
	sendEphemeralFollowUp(s, i, content)
}

func handleShowQuickPresets(s *discordgo.Session, i *discordgo.InteractionCreate, userID, guildID string, b *bot.Bot) {
	quickPresets, err := database.GetUserQuickPresets(userID, guildID)
	if err != nil {
		log.Printf("Failed to get quick presets for user %s in guild %s: %v", userID, guildID, err)
		sendEphemeralFollowUp(s, i, "获取快速预设列表失败。")
		return
	}

	if len(quickPresets) == 0 {
		sendEphemeralFollowUp(s, i, "您还没有设置任何快速预设。")
		return
	}

	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		sendEphemeralFollowUp(s, i, "找不到服务器配置。")
		return
	}

	var fields []*discordgo.MessageEmbedField
	for slot := 1; slot <= 3; slot++ {
		presetID, ok := quickPresets[slot]
		var value string
		if ok {
			preset := FindPreset(&serverConfig, presetID)
			if preset != nil {
				value = fmt.Sprintf("ID: `%s`\n名称: %s", preset.ID, preset.Name)
			} else {
				value = fmt.Sprintf("ID: `%s` (未找到)", presetID)
			}
		} else {
			value = "空"
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "槽位 " + strconv.Itoa(slot),
			Value: value,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:  "您的快速预设",
		Fields: fields,
		Color:  0x00ff00, // Green
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
		Flags:  discordgo.MessageFlagsEphemeral,
	})
}

func handleSendQuickPreset(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, userID, guildID string, slot int) {
	quickPresets, err := database.GetUserQuickPresets(userID, guildID)
	if err != nil {
		log.Printf("Failed to get quick presets for user %s in guild %s: %v", userID, guildID, err)
		sendEphemeralFollowUp(s, i, "获取快速预设失败。")
		return
	}

	presetID, ok := quickPresets[slot]
	if !ok {
		content := fmt.Sprintf("槽位 %d 为空。请先使用 `action: add` 添加一个预设。", slot)
		sendEphemeralFollowUp(s, i, content)
		return
	}

	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		sendEphemeralFollowUp(s, i, "找不到服务器配置。")
		return
	}

	selectedPreset := FindPreset(&serverConfig, presetID)
	if selectedPreset == nil {
		log.Printf("Could not find preset with ID: %s", presetID)
		sendEphemeralFollowUp(s, i, "找不到所选的预设。")
		return
	}

	// Call the centralized sendPreset function
	sendPreset(s, i, b, selectedPreset, "", "") // No user mention or message link for quick presets
}

// HandleQuickPresetReplyCommand handles the "快速预设回复" user command.
func HandleQuickPresetReplyCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	targetMessage, ok := i.ApplicationCommandData().Resolved.Messages[i.ApplicationCommandData().TargetID]
	if !ok {
		log.Printf("Failed to resolve target message for quick preset reply")
		utils.SendEphemeralResponse(s, i, "无法获取目标消息。")
		return
	}
	targetUserID := targetMessage.Author.ID
	userID := i.Member.User.ID
	guildID := i.GuildID

	quickPresets, err := database.GetUserQuickPresets(userID, guildID)
	if err != nil {
		log.Printf("Failed to get quick presets for user %s in guild %s: %v", userID, guildID, err)
		utils.SendEphemeralResponse(s, i, "获取快速预设失败。")
		return
	}

	if len(quickPresets) == 0 {
		utils.SendEphemeralResponse(s, i, "您还没有设置任何快速预设。")
		return
	}

	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		utils.SendEphemeralResponse(s, i, "找不到服务器配置。")
		return
	}

	// Create embed with preset information
	var embedFields []*discordgo.MessageEmbedField
	var buttons []discordgo.MessageComponent

	for slot := 1; slot <= 3; slot++ {
		presetID, ok := quickPresets[slot]
		var label, emoji string
		var style discordgo.ButtonStyle
		var disabled bool
		var fieldValue string

		if ok {
			preset := FindPreset(&serverConfig, presetID)
			if preset != nil {
				// Set button properties
				label = preset.Name
				style = discordgo.PrimaryButton
				disabled = false

				// Choose emoji based on preset type
				if preset.Type == "embed" {
					emoji = "📝"
				} else {
					emoji = "💬"
				}

				// Create field for embed
				previewText := preset.Value
				if len(previewText) > 50 {
					previewText = previewText[:47] + "..."
				}
				fieldValue = fmt.Sprintf("%s **%s**\n`%s`", emoji, preset.Name, previewText)
			} else {
				// Preset not found
				label = "❌ 预设丢失"
				style = discordgo.DangerButton
				disabled = true
				fieldValue = "❌ **预设丢失**\n`找不到预设: " + presetID + "`"
			}
		} else {
			// Empty slot
			label = "➕ 空槽位"
			style = discordgo.SecondaryButton
			disabled = true
			fieldValue = "➕ **空槽位**\n`点击主命令添加预设`"
		}

		// Add field to embed
		embedFields = append(embedFields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("槽位 %d", slot),
			Value:  fieldValue,
			Inline: true,
		})

		// Add button
		buttons = append(buttons, discordgo.Button{
			Label:    label,
			Style:    style,
			Disabled: disabled,
			CustomID: fmt.Sprintf("quick_preset_reply_%d_%s_%s", slot, targetUserID, targetMessage.ID),
		})
	}

	// Create embed
	embed := &discordgo.MessageEmbed{
		Title:       "📋 选择快速预设回复",
		Description: fmt.Sprintf("回复给 <@%s> 的消息", targetUserID),
		Fields:      embedFields,
		Color:       0x5865F2, // Discord Blurple
		Footer: &discordgo.MessageEmbedFooter{
			Text: "💡 提示: 使用 /quick-preset 命令管理您的快速预设",
		},
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: buttons,
				},
			},
		},
	})
	if err != nil {
		log.Printf("Failed to send quick preset reply selection: %v", err)
	}
}

func HandleQuickPresetReplyButton(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, "_")
	if len(parts) != 6 {
		log.Printf("Invalid custom ID for quick preset reply button: %s", customID)
		return
	}

	slot, err := strconv.Atoi(parts[3])
	if err != nil {
		log.Printf("Invalid slot in custom ID for quick preset reply button: %s", customID)
		return
	}
	// parts[4] is targetUserID, not needed for reply functionality
	targetMessageID := parts[5]

	userID := i.Member.User.ID
	guildID := i.GuildID

	quickPresets, err := database.GetUserQuickPresets(userID, guildID)
	if err != nil {
		log.Printf("Failed to get quick presets for user %s in guild %s: %v", userID, guildID, err)
		sendEphemeralFollowUp(s, i, "获取快速预设失败。")
		return
	}

	presetID, ok := quickPresets[slot]
	if !ok {
		sendEphemeralFollowUp(s, i, "找不到所选槽位的预设。")
		return
	}

	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		sendEphemeralFollowUp(s, i, "找不到服务器配置。")
		return
	}

	selectedPreset := FindPreset(&serverConfig, presetID)
	if selectedPreset == nil {
		log.Printf("Could not find preset with ID: %s", presetID)
		sendEphemeralFollowUp(s, i, "找不到所选的预设。")
		return
	}

	// Acknowledge the interaction
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: "预设已发送。",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Failed to acknowledge quick preset reply button click: %v", err)
	}

	// Send the preset as a reply to the target message
	sendPresetAsReply(s, i, b, selectedPreset, targetMessageID)
}

// sendPresetAsReply sends a preset as a reply to a specific message
func sendPresetAsReply(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, selectedPreset *model.PresetMessage, targetMessageID string) {
	messageSend := FormatPresetMessageSend(selectedPreset, "")

	// Set the message reference to reply to the target message
	messageSend.Reference = &discordgo.MessageReference{
		MessageID: targetMessageID,
		ChannelID: i.ChannelID,
		GuildID:   i.GuildID,
	}

	message, err := s.ChannelMessageSendComplex(i.ChannelID, messageSend)
	if err != nil {
		log.Printf("Failed to send preset reply: %v", err)
		return
	}

	// Log the usage
	if b.GetConfig().LogChannelID != "" {
		logMessageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, i.ChannelID, message.ID)
		logInfo := fmt.Sprintf("用户: `%s`\n预设名: `%s`\n[点击查看消息](%s)", i.Member.User.Username, selectedPreset.Name, logMessageLink)
		utils.LogInfo(s, b.GetConfig().LogChannelID, "快速预设回复", "使用", logInfo)
	}
}
