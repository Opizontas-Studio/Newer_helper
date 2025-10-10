package utils

import (
	"encoding/json"
	"fmt"
	"newer_helper/model"
	"os"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
)

const tagMappingDir = "data/tag_mapping"

// GenerateTagMappingFiles generates tag mapping configuration files based on the task configuration.
func GenerateTagMappingFiles(s *discordgo.Session, taskConfig model.TaskConfig) error {
	if err := os.MkdirAll(tagMappingDir, 0755); err != nil {
		return fmt.Errorf("error creating tag mapping directory %s: %w", tagMappingDir, err)
	}

	for guildID, guildConfig := range taskConfig {
		guildTagMapping := make(map[string]map[string]string)

		for categoryName, channelTask := range guildConfig.Data {
			channel, err := s.Channel(channelTask.ChannelID)
			if err != nil {
				return fmt.Errorf("error fetching channel %s for guild %s: %w", channelTask.ChannelID, guildID, err)
			}

			if len(channel.AvailableTags) > 0 {
				tagMap := make(map[string]string)
				for _, tag := range channel.AvailableTags {
					tagMap[tag.ID] = tag.Name
				}
				guildTagMapping[categoryName] = tagMap
			}
		}

		filePath := filepath.Join(tagMappingDir, fmt.Sprintf("%s_config.json", guildID))
		jsonData, err := json.MarshalIndent(guildTagMapping, "", "    ")
		if err != nil {
			return fmt.Errorf("error marshalling tag mapping for guild %s: %w", guildID, err)
		}

		if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
			return fmt.Errorf("error writing tag mapping file for guild %s: %w", guildID, err)
		}
	}

	return nil
}
