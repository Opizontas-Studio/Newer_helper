package database

import (
	"database/sql"
	"fmt"
)

// GetUserQuickPresets retrieves the quick presets for a given user in a specific guild.
// It returns a map where the key is the slot number (1, 2, 3) and the value is the preset ID.
func GetUserQuickPresets(userID, guildID string) (map[int]string, error) {
	db, err := InitUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	var preset1, preset2, preset3 sql.NullString
	query := "SELECT quick_preset_1, quick_preset_2, quick_preset_3 FROM user_preferences WHERE user_id = ? AND guild_id = ?"
	err = db.QueryRow(query, userID, guildID).Scan(&preset1, &preset2, &preset3)

	if err != nil {
		if err == sql.ErrNoRows {
			// No record for this user/guild, return empty map
			return make(map[int]string), nil
		}
		return nil, fmt.Errorf("failed to query user quick presets for guild %s: %w", guildID, err)
	}

	presets := make(map[int]string)
	if preset1.Valid {
		presets[1] = preset1.String
	}
	if preset2.Valid {
		presets[2] = preset2.String
	}
	if preset3.Valid {
		presets[3] = preset3.String
	}

	return presets, nil
}

// SetUserQuickPreset sets or updates a quick preset for a given user in a specific guild.
func SetUserQuickPreset(userID, guildID string, slotID int, presetID string) error {
	if slotID < 1 || slotID > 3 {
		return fmt.Errorf("invalid slot ID: %d, must be between 1 and 3", slotID)
	}

	db, err := InitUserDB()
	if err != nil {
		return fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	columnName := fmt.Sprintf("quick_preset_%d", slotID)

	// Use INSERT ON CONFLICT to handle both new and existing user rows.
	query := fmt.Sprintf(`
    INSERT INTO user_preferences (user_id, guild_id, %s)
    VALUES (?, ?, ?)
    ON CONFLICT(user_id, guild_id) DO UPDATE SET %s = excluded.%s;`, columnName, columnName, columnName)

	_, err = db.Exec(query, userID, guildID, presetID)
	if err != nil {
		return fmt.Errorf("failed to set user quick preset for slot %d in guild %s: %w", slotID, guildID, err)
	}

	return nil
}

// RemoveUserQuickPreset removes a quick preset for a given user in a specific guild by setting it to NULL.
func RemoveUserQuickPreset(userID, guildID string, slotID int) error {
	if slotID < 1 || slotID > 3 {
		return fmt.Errorf("invalid slot ID: %d, must be between 1 and 3", slotID)
	}

	db, err := InitUserDB()
	if err != nil {
		return fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	columnName := fmt.Sprintf("quick_preset_%d", slotID)
	query := fmt.Sprintf("UPDATE user_preferences SET %s = NULL WHERE user_id = ? AND guild_id = ?", columnName)

	_, err = db.Exec(query, userID, guildID)
	if err != nil {
		return fmt.Errorf("failed to remove user quick preset for slot %d in guild %s: %w", slotID, guildID, err)
	}

	return nil
}
