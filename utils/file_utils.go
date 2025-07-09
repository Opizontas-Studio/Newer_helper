package utils

import (
	"discord-bot/model"
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadTagMapping loads the tag name mapping from a JSON file.
func LoadTagMapping(file string) (map[string]map[string]string, error) {
	if file == "" {
		return nil, nil // No mapping file configured
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var mapping map[string]map[string]string
	err = json.Unmarshal(data, &mapping)
	if err != nil {
		return nil, err
	}
	return mapping, nil
}

const LeaderboardStateFile = "data/leaderboard_state.json"

func SaveLeaderboardState(states map[string]model.LeaderboardState) error {
	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(LeaderboardStateFile, data, 0644)
}

func LoadLeaderboardState() (map[string]model.LeaderboardState, error) {
	states := make(map[string]model.LeaderboardState)
	data, err := os.ReadFile(LeaderboardStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return states, nil // Return empty map if file doesn't exist
		}
		return nil, err
	}
	if len(data) == 0 {
		return states, nil // Return empty map if file is empty
	}
	err = json.Unmarshal(data, &states)
	return states, err
}

func LoadDatabaseMapping() (map[string]model.GuildMapping, error) {
	var mapping map[string]model.GuildMapping
	data, err := os.ReadFile("data/databaseMapping.json")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &mapping)
	return mapping, err
}

// ListDBFiles lists all files with .db extension in the data directory.
func ListDBFiles() ([]string, error) {
	var files []string
	fileInfos, err := os.ReadDir("./data")
	if err != nil {
		return nil, err
	}
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() && filepath.Ext(fileInfo.Name()) == ".db" {
			files = append(files, filepath.Join("./data", fileInfo.Name()))
		}
	}
	return files, nil
}
