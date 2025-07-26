package handlers

import (
	"discord-bot/bot"
	"discord-bot/model"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func findPreset(presets []model.PresetMessage, id string) (model.PresetMessage, error) {
	for _, p := range presets {
		if p.ID == id {
			return p, nil
		}
	}
	return model.PresetMessage{}, fmt.Errorf("preset with id %s not found", id)
}

func HandleAutoTriggerMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate, b *bot.Bot) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	serverConfig, ok := b.GetConfig().ServerConfigs[m.GuildID]
	if !ok {
		return
	}

	for _, trigger := range serverConfig.AutoTriggers {
		if trigger.ChannelID == m.ChannelID && trigger.Keyword == m.Content {
			preset, err := findPreset(serverConfig.PresetMessages, trigger.PresetID)
			if err != nil {
				log.Printf("Preset not found for ID: %s", trigger.PresetID)
				continue
			}

			_, err = s.ChannelMessageSendReply(m.ChannelID, preset.Value, m.Reference())
			if err != nil {
				log.Printf("Failed to send preset message reply: %v", err)
			}
			return // Stop after first match
		}
	}
}
