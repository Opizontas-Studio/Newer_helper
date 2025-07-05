package model

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// PresetMessage 定义了预设消息的结构
type PresetMessage struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
}

// FindPreset 在给定的服务器配置中查找预设消息
func FindPreset(serverConfig *ServerConfig, presetValue string) *PresetMessage {
	for _, p := range serverConfig.PresetMessages {
		if p.Name == presetValue {
			return &p
		}
	}
	return nil
}

// FormatPresetMessageSend formats a preset message into a MessageSend struct.
func FormatPresetMessageSend(preset *PresetMessage, user string) *discordgo.MessageSend {
	messageSend := &discordgo.MessageSend{}
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
			content = fmt.Sprintf("%s %s", user, content)
		}
		messageSend.Content = content
	}
	return messageSend
}

// HandlePresetMessageInteraction 处理预设消息交互
func HandlePresetMessageInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b interface{}) {
	type bot interface {
		GetConfig() *Config
	}

	appBot := b.(bot)
	serverConfig, ok := appBot.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		return
	}

	// Defer the response ephemerally
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Failed to defer interaction response: %v", err)
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var presetValue, user string
	if option, ok := optionMap["preset"]; ok {
		presetValue = option.StringValue()
	}

	if option, ok := optionMap["user"]; ok {
		u := option.UserValue(s)
		if u != nil {
			user = u.Mention()
		}
	}

	selectedPreset := FindPreset(&serverConfig, presetValue)
	if selectedPreset == nil {
		log.Printf("Could not find preset with value: %s", presetValue)
		_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Could not find the selected preset.",
		})
		if err != nil {
			log.Printf("Failed to send error followup message: %v", err)
		}
		return
	}

	messageSend := FormatPresetMessageSend(selectedPreset, user)
	_, err = s.ChannelMessageSendComplex(i.ChannelID, messageSend)

	if err != nil {
		log.Printf("Failed to send channel message: %v", err)
		_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Failed to send the preset message.",
		})
		if err != nil {
			log.Printf("Failed to send error followup message: %v", err)
		}
		return
	}

	// Delete the original deferred response ("Bot is thinking...")
	err = s.InteractionResponseDelete(i.Interaction)
	if err != nil {
		log.Printf("Failed to delete interaction response: %v", err)
	}
}
