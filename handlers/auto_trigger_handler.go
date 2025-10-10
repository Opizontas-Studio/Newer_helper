package handlers

import (
	"log"
	"newer_helper/bot"
	"newer_helper/model"
	"newer_helper/utils"

	"github.com/bwmarrin/discordgo"
)

func AutoTriggerHandler(s *discordgo.Session, m *discordgo.MessageCreate, b *bot.Bot) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	serverConfig, ok := b.GetConfig().ServerConfigs[m.GuildID]
	if !ok {
		return
	}

	for _, trigger := range serverConfig.AutoTriggers {
		for _, keyword := range trigger.Keywords {
			if trigger.ChannelID == m.ChannelID && keyword == m.Content {
				var preset *model.PresetMessage
				for _, p := range serverConfig.PresetMessages {
					if p.ID == trigger.PresetID {
						preset = &p
						break
					}
				}

				if preset != nil {
					_, err := s.ChannelMessageSend(m.ChannelID, preset.Value)
					if err != nil {
						log.Printf("Error sending auto-trigger message: %v", err)
						utils.SendPrivateMessage(s, m.ChannelID, "Error sending preset message.")
					}
				}
				return
			}
		}
	}
}
