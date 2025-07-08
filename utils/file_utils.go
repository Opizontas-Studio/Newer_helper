package utils

import (
	"discord-bot/model"
	"encoding/json"
	"os"
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

func SaveLeaderboardState(state model.LeaderboardState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(LeaderboardStateFile, data, 0644)
}

func LoadLeaderboardState() (model.LeaderboardState, error) {
	var state model.LeaderboardState
	data, err := os.ReadFile(LeaderboardStateFile)
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(data, &state)
	return state, err
}
