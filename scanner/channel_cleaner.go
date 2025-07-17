package scanner

import (
	"discord-bot/model"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func StartChannelCleaner(s *discordgo.Session, cfg *model.Config, done <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Minute) // Clean every 5 minutes
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Println("Running scheduled channel cleanup...")
				CleanAllChannels(s, cfg)
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()
}

func CleanAllChannels(s *discordgo.Session, cfg *model.Config) {
	log.Println("Executing CleanAllChannels...")
	if len(cfg.ServerConfigs) == 0 {
		log.Println("No server configs found.")
		return
	}
	for guildID, serverConfig := range cfg.ServerConfigs {
		if len(serverConfig.TopChannels) == 0 {
			log.Printf("No top channels configured for guild %s.", guildID)
			continue
		}
		log.Printf("Found %d top channels for guild %s.", len(serverConfig.TopChannels), guildID)
		for _, topChannel := range serverConfig.TopChannels {
			go cleanSingleChannel(s, topChannel)
		}
	}
	log.Println("Finished CleanAllChannels.")
}

func cleanSingleChannel(s *discordgo.Session, cfg *model.TopChannelConfig) {
	log.Printf("Cleaning channel %s with message limit %d.", cfg.ChannelID, cfg.MessageLimit)
	messages, err := s.ChannelMessages(cfg.ChannelID, 100, "", "", "")
	if err != nil {
		log.Printf("Error fetching messages for channel %s: %v", cfg.ChannelID, err)
		return
	}

	if len(messages) <= cfg.MessageLimit {
		return // No need to clean
	}

	// Create a set of excluded IDs for quick lookup
	excludedSet := make(map[string]struct{}, len(cfg.ExcludedMessageIDs))
	for _, id := range cfg.ExcludedMessageIDs {
		excludedSet[id] = struct{}{}
	}

	messagesToDelete := make([]string, 0)
	// Messages are returned newest first, so we iterate backwards to find the oldest.
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.ID == "" {
			continue
		}
		if _, isExcluded := excludedSet[msg.ID]; !isExcluded {
			messagesToDelete = append(messagesToDelete, msg.ID)
		}
		if len(messages)-len(messagesToDelete) < cfg.MessageLimit {
			break // We have reached the target message count
		}
	}

	if len(messagesToDelete) > 0 {
		err := s.ChannelMessagesBulkDelete(cfg.ChannelID, messagesToDelete)
		if err != nil {
			log.Printf("Error bulk deleting messages in channel %s. Error: %v. Messages to delete: %v", cfg.ChannelID, err, messagesToDelete)
		} else {
			log.Printf("Successfully deleted %d messages from channel %s.", len(messagesToDelete), cfg.ChannelID)
		}
	}
}
