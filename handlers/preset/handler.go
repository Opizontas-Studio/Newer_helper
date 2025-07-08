package preset

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

// HandlePresetMessageInteraction handles the preset message interaction
func HandlePresetMessageInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	cooldowns := b.GetPresetCooldowns()
	mutex := b.GetCooldownMutex()

	serverConfig, ok := b.Config.ServerConfigs[i.GuildID]
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

	mutex.Lock()
	defer mutex.Unlock()

	log.Printf("Checking cooldown for preset: %s", selectedPreset.ID)
	if lastUsed, ok := cooldowns[selectedPreset.ID]; ok {
		log.Printf("Preset '%s' was last used at %v", selectedPreset.ID, lastUsed)
		if time.Since(lastUsed) < 30*time.Second {
			log.Printf("Preset '%s' is on cooldown.", selectedPreset.ID)
			_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("Preset '%s' is on cooldown. Please wait.", selectedPreset.Name),
			})
			if err != nil {
				log.Printf("Failed to send cooldown message: %v", err)
			}
			return
		}
	}

	log.Printf("Updating cooldown for preset: %s", selectedPreset.ID)
	cooldowns[selectedPreset.ID] = time.Now()

	messageSend := FormatPresetMessageSend(selectedPreset, user)
	message, err := s.ChannelMessageSendComplex(i.ChannelID, messageSend)
	if err == nil {
		// Log the successful preset usage
		if b.Config.LogChannelID != "" {
			messageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, i.ChannelID, message.ID)
			logInfo := fmt.Sprintf("用户: `%s`\n预设名: `%s`\n[点击查看消息](%s)", i.Member.User.Username, selectedPreset.Name, messageLink)
			err = utils.LogInfo(s, b.Config.LogChannelID, "预设", "使用", logInfo)
			if err != nil {
				log.Printf("Failed to send log: %v", err)
			}
		}
	}

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
			content = fmt.Sprintf("%s \n%s", user, content)
		}
		messageSend.Content = content
	}
	return messageSend
}
