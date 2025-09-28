package preset

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// HandlePresetMessageInteraction handles the preset message interaction
func HandlePresetMessageInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		return
	}

	// Defer the response ephemerally
	if err := utils.DeferResponse(s, i, true); err != nil {
		log.Printf("Failed to defer interaction response: %v", err)
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var presetID, user string
	if option, ok := optionMap["id"]; ok {
		presetID = option.StringValue()
	}

	if option, ok := optionMap["user"]; ok {
		u := option.UserValue(s)
		if u != nil {
			user = u.Mention()
		}
	}

	var messageLink string
	if option, ok := optionMap["message_link"]; ok {
		messageLink = option.StringValue()
	}

	selectedPreset := FindPreset(&serverConfig, presetID)
	if selectedPreset == nil {
		log.Printf("Could not find preset with ID: %s", presetID)
		_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Could not find the selected preset.",
		})
		if err != nil {
			log.Printf("Failed to send error followup message: %v", err)
		}
		return
	}

	sendPreset(s, i, b, selectedPreset, user, messageLink)
}

// sendPreset handles the logic of sending a preset message, including cooldowns, permissions, and confirmations.
func sendPreset(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, selectedPreset *model.PresetMessage, user, messageLink string) {
	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		return
	}

	cooldowns := b.GetPresetCooldowns()
	mutex := b.GetCooldownMutex()

	mutex.Lock()
	if lastUsed, ok := cooldowns[selectedPreset.ID]; ok {
		if time.Since(lastUsed) < 30*time.Second {
			mutex.Unlock()
			log.Printf("Preset '%s' is on cooldown.", selectedPreset.ID)
			_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("Preset '%s' is on cooldown. Please wait.", selectedPreset.Name),
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			if err != nil {
				log.Printf("Failed to send cooldown message: %v", err)
			}
			return
		}
	}
	cooldowns[selectedPreset.ID] = time.Now()
	mutex.Unlock()

	permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)

	if permissionLevel == utils.GuestPermission {
		messageSend := FormatPresetMessageSend(selectedPreset, "") // User mention is ignored for private view
		webhookParams := &discordgo.WebhookParams{
			Content: messageSend.Content,
			Embeds:  messageSend.Embeds,
			Flags:   discordgo.MessageFlagsEphemeral,
		}
		_, err := s.FollowupMessageCreate(i.Interaction, true, webhookParams)
		if err != nil {
			log.Printf("Failed to send private followup message: %v", err)
		}
		return
	}

	skipConfirmation, err := database.GetUserPresetConfirmationPreference(i.Member.User.ID, i.GuildID)
	if err != nil {
		log.Printf("Failed to get user preference, proceeding with confirmation: %v", err)
	}

	messageSend := FormatPresetMessageSend(selectedPreset, user)
	if messageLink != "" {
		_, _, msgID, err := utils.ParseMessageLink(messageLink)
		if err != nil {
			log.Printf("Invalid message link: %v", err)
		} else {
			messageSend.Reference = &discordgo.MessageReference{
				MessageID: msgID,
				ChannelID: i.ChannelID,
				GuildID:   i.GuildID,
			}
		}
	}

	if skipConfirmation {
		message, err := s.ChannelMessageSendComplex(i.ChannelID, messageSend)
		if err != nil {
			log.Printf("Failed to send channel message directly: %v", err)
			utils.SendEphemeralResponse(s, i, "发送消息失败。")
			return
		}
		if b.GetConfig().LogChannelID != "" {
			logMessageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, i.ChannelID, message.ID)
			logInfo := fmt.Sprintf("用户: `%s`\n预设名: `%s`\n[点击查看消息](%s)", i.Member.User.Username, selectedPreset.Name, logMessageLink)
			utils.LogInfo(s, b.GetConfig().LogChannelID, "预设", "使用", logInfo)
		}
		s.InteractionResponseDelete(i.Interaction)
	} else {
		pendingPresets := b.GetPendingPresets()
		pendingPresetsMutex := b.GetPendingPresetsMutex()
		pendingPresetsMutex.Lock()

		interactionID := i.Interaction.ID
		pendingPresets[interactionID] = &bot.PendingPreset{
			MessageSend: messageSend,
			PresetName:  selectedPreset.Name,
			UserID:      i.Member.User.ID,
			Timestamp:   time.Now(),
		}
		pendingPresetsMutex.Unlock()

		webhookParams := &discordgo.WebhookParams{
			Content: messageSend.Content,
			Embeds:  messageSend.Embeds,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "确认发送",
							Style:    discordgo.SuccessButton,
							CustomID: "confirm_preset_" + interactionID,
						},
						discordgo.Button{
							Label:    "取消",
							Style:    discordgo.DangerButton,
							CustomID: "cancel_preset_" + interactionID,
						},
						discordgo.Button{
							Label:    "不再确认",
							Style:    discordgo.SecondaryButton,
							CustomID: "disable_confirm_preset_" + interactionID,
						},
					},
				},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		}

		if webhookParams.Content != "" {
			webhookParams.Content = "请预览并确认发送以下消息：\n\n" + webhookParams.Content
		} else {
			webhookParams.Content = "请预览并确认发送以下消息："
		}

		_, err := s.FollowupMessageCreate(i.Interaction, true, webhookParams)
		if err != nil {
			log.Printf("Failed to send confirmation message: %v", err)
			pendingPresetsMutex.Lock()
			delete(pendingPresets, interactionID)
			pendingPresetsMutex.Unlock()
		}
	}
}

// FindPreset finds a preset message in the given server configuration.
func FindPreset(serverConfig *model.ServerConfig, presetID string) *model.PresetMessage {
	for _, p := range serverConfig.PresetMessages {
		if p.ID == presetID {
			return &p
		}
	}
	log.Printf("No preset found with ID: '%s'", presetID)
	return nil
}

// FormatPresetMessageSend formats a preset message into a MessageSend struct.
func FormatPresetMessageSend(preset *model.PresetMessage, user string) *discordgo.MessageSend {
	messageSend := &discordgo.MessageSend{
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
		},
	}

	if preset.Type == "embed" {
		description := preset.Description
		if user != "" {
			description = fmt.Sprintf("%s %s", user, description)
		}
		embed := &discordgo.MessageEmbed{
			Title:       preset.Value,
			Description: description,
		}
		messageSend.Embeds = []*discordgo.MessageEmbed{embed}
	} else {
		content := preset.Value
		if user != "" {
			content = fmt.Sprintf("%s\n%s", user, content)
		}
		messageSend.Content = content
	}

	if user == "" {
		messageSend.AllowedMentions = nil
	}

	return messageSend
}

// HandlePresetConfirmationInteraction handles the confirmation or cancellation of a preset message.
func HandlePresetConfirmationInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	customID := i.MessageComponentData().CustomID
	var action, interactionID string

	if strings.HasPrefix(customID, "disable_confirm_preset_") {
		action = "disable"
		interactionID = strings.TrimPrefix(customID, "disable_confirm_preset_")
	} else {
		parts := strings.Split(customID, "_")
		if len(parts) < 3 {
			log.Printf("Invalid custom ID for preset confirmation: %s", customID)
			return
		}
		action = parts[0]
		interactionID = parts[2]
	}

	pendingPresets := b.GetPendingPresets()
	pendingPresetsMutex := b.GetPendingPresetsMutex()
	pendingPresetsMutex.Lock()
	defer pendingPresetsMutex.Unlock()

	pending, ok := pendingPresets[interactionID]
	if !ok {
		utils.SendEphemeralResponse(s, i, "This confirmation has expired or is invalid.")
		return
	}

	// Security check: ensure the user clicking the button is the one who initiated the command
	if i.Member.User.ID != pending.UserID {
		utils.SendEphemeralResponse(s, i, "You are not authorized to respond to this confirmation.")
		return
	}

	delete(pendingPresets, interactionID) // The request is handled, remove it from pending

	switch action {
	case "cancel":
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "操作已取消。",
				Components: []discordgo.MessageComponent{},
				Embeds:     []*discordgo.MessageEmbed{},
			},
		})
		if err != nil {
			log.Printf("Failed to update cancellation message: %v", err)
		}
		return
	case "confirm":
		message, err := s.ChannelMessageSendComplex(i.ChannelID, pending.MessageSend)
		if err != nil {
			log.Printf("Failed to send channel message after confirmation: %v", err)
			// Try to inform the user about the failure
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content:    "发送消息失败。",
					Components: []discordgo.MessageComponent{},
					Embeds:     []*discordgo.MessageEmbed{},
				},
			})
			return
		}

		// Log the successful preset usage
		if b.GetConfig().LogChannelID != "" {
			logMessageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, i.ChannelID, message.ID)
			logInfo := fmt.Sprintf("用户: `%s`\n预设名: `%s`\n[点击查看消息](%s)", i.Member.User.Username, pending.PresetName, logMessageLink)
			err = utils.LogInfo(s, b.GetConfig().LogChannelID, "预设", "使用", logInfo)
			if err != nil {
				log.Printf("Failed to send log: %v", err)
			}
		}

		// Update the original interaction to show it was successful
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "消息已成功发送！",
				Components: []discordgo.MessageComponent{},
				Embeds:     []*discordgo.MessageEmbed{},
			},
		})
		if err != nil {
			log.Printf("Failed to update confirmation message: %v", err)
		}
	case "disable":
		// 1. Immediately acknowledge the interaction to prevent "interaction failed" error.
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		if err != nil {
			log.Printf("Failed to defer update: %v", err)
			// If we can't even defer, we probably can't do anything else.
			return
		}

		// 2. Perform the background tasks (DB update and message sending).
		go func() {
			// Set the user's preference to skip confirmation in the future
			dbErr := database.SetUserPresetConfirmationPreference(i.Member.User.ID, i.GuildID, true)
			if dbErr != nil {
				log.Printf("Failed to set user preference: %v", dbErr)
			}

			// Send the message
			message, sendErr := s.ChannelMessageSendComplex(i.ChannelID, pending.MessageSend)
			if sendErr != nil {
				log.Printf("Failed to send channel message after disabling confirmation: %v", sendErr)
				s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: "发送消息失败。",
				})
				return
			}

			// Log the usage
			if b.GetConfig().LogChannelID != "" {
				logMessageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, i.ChannelID, message.ID)
				logInfo := fmt.Sprintf("用户: `%s`\n预设名: `%s`\n[点击查看消息](%s)", i.Member.User.Username, pending.PresetName, logMessageLink)
				utils.LogInfo(s, b.GetConfig().LogChannelID, "预设", "使用", logInfo)
			}

			// 3. Edit the original interaction response with the final success message.
			finalContent := "消息已发送，并为您保存设置：以后将不再进行二次确认。"
			if dbErr != nil {
				finalContent = "消息已发送，但保存您的偏好设置时出错。"
			}
			_, editErr := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content:    &finalContent,
				Components: &[]discordgo.MessageComponent{},
				Embeds:     &[]*discordgo.MessageEmbed{},
			})
			if editErr != nil {
				log.Printf("Failed to edit final message: %v", editErr)
			}
		}()
	}
}
