package utils

import (
	"discord-bot/model"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const newCardPushConfigDir = "data/new_card_push_config"

// getNewCardPushConfigPath returns the file path for a given guild's new card push config.
func getNewCardPushConfigPath(guildID string) string {
	return filepath.Join(newCardPushConfigDir, fmt.Sprintf("%s.json", guildID))
}

// LoadNewCardPushConfig loads the new card push configuration for a specific guild.
// If the config file does not exist, it returns a new empty config struct.
func LoadNewCardPushConfig(guildID string) (*model.NewCardPushConfig, error) {
	filePath := getNewCardPushConfigPath(guildID)

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return a default config if the file doesn't exist, not an error.
			return &model.NewCardPushConfig{}, nil
		}
		return nil, fmt.Errorf("error reading new card push config file %s: %w", filePath, err)
	}

	var config model.NewCardPushConfig
	if err := json.Unmarshal(fileData, &config); err != nil {
		return nil, fmt.Errorf("error unmarshalling new card push config from %s: %w", filePath, err)
	}

	return &config, nil
}

// SaveNewCardPushConfig saves the new card push configuration for a specific guild.
func SaveNewCardPushConfig(guildID string, config *model.NewCardPushConfig) error {
	if err := os.MkdirAll(newCardPushConfigDir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %w", newCardPushConfigDir, err)
	}

	filePath := getNewCardPushConfigPath(guildID)

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling new card push config to JSON: %w", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("error writing new card push config to file %s: %w", filePath, err)
	}

	return nil
}
