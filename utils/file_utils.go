package utils

import (
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
