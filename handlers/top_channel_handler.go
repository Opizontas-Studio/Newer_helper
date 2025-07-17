package handlers

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func HandleRegisterTopChannel(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	if _, ok := b.GetConfig().ServerConfigs[i.GuildID]; !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		utils.SendErrorResponse(s, i, "Server config not found.")
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	channel := optionMap["channel"].ChannelValue(s)
	limit := optionMap["limit"].IntValue()
	var excludedIDs []string
	if opt, ok := optionMap["exclude-ids"]; ok {
		// Trim spaces and filter out empty strings
		rawIDs := strings.Split(opt.StringValue(), ",")
		for _, id := range rawIDs {
			trimmedID := strings.TrimSpace(id)
			if trimmedID != "" {
				excludedIDs = append(excludedIDs, trimmedID)
			}
		}
	}

	topChannelConfig := model.TopChannelConfig{
		ChannelID:          channel.ID,
		MessageLimit:       int(limit),
		ExcludedMessageIDs: excludedIDs,
	}

	// Save the new config to the database
	if err := database.SaveTopChannelConfig(b.GetDB(), i.GuildID, topChannelConfig); err != nil {
		log.Printf("Error saving top channel config for guild %s: %v", i.GuildID, err)
		utils.SendErrorResponse(s, i, "Failed to save configuration to the database.")
		return
	}

	if err := b.ReloadConfig(); err != nil {
		log.Printf("Error reloading config after saving top channel for guild %s: %v", i.GuildID, err)
		utils.SendErrorResponse(s, i, "Configuration saved, but failed to reload the config. Please use /reload-config.")
		return
	}

	log.Printf("Successfully registered and reloaded config for top channel %s for guild %s.", channel.ID, i.GuildID)

	utils.SendPublicResponse(s, i, fmt.Sprintf("✅ Channel <#%s> has been successfully registered as a top channel.", channel.ID))
}

// This regex captures the base URL and an optional message ID part.
var discordLinkRegex = regexp.MustCompile(`(https://discord\.com/channels/\d+/\d+)(/\d+)?`)

func HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate, b *bot.Bot) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	serverConfig, ok := b.GetConfig().ServerConfigs[m.GuildID]
	if !ok {
		return
	}

	topChannelConfig, isTopChannel := serverConfig.TopChannels[m.ChannelID]
	if !isTopChannel || topChannelConfig == nil {
		return // Not a registered top channel
	}

	matches := discordLinkRegex.FindAllStringSubmatch(m.Content, -1)

	// If the message contains one or more valid links
	if len(matches) > 0 {
		newContent := m.Content
		modified := false
		for _, match := range matches {
			baseURL := match[1]
			messageIDPart := match[2]

			// Only process links that do NOT have a message ID part.
			if messageIDPart == "" {
				newLink := baseURL + "/0"
				newContent = strings.Replace(newContent, baseURL, newLink, 1)
				modified = true
			}
		}

		// If no links were modified (e.g., all links already had message IDs), do nothing.
		if !modified {
			return
		}

		// Delete the original message and send the new one.
		err := s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
			log.Printf("Failed to delete original message %s: %v", m.ID, err)
		}
		_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s (来自 ： %s)", newContent, m.Author.Mention()))
		if err != nil {
			log.Printf("Failed to send corrected message for user %s: %v", m.Author.ID, err)
		}
	} else {
		// If the message does NOT contain a link, delete it.
		err := s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
			log.Printf("Failed to delete non-link message %s in top channel: %v", m.ID, err)
		}
	}
}
