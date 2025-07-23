package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const userDbPath = "./data/user.db"

// InitUserDB initializes the user database.
// It creates the database file and the necessary tables if they don't exist.
func InitUserDB() (*sql.DB, error) {
	// Ensure the data directory exists
	if err := os.MkdirAll("./data", os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sql.Open("sqlite3", userDbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open user database: %w", err)
	}

	if err := createUserTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create user tables: %w", err)
	}

	return db, nil
}

// createUserTables creates and alters the necessary tables in the user database.
func createUserTables(db *sql.DB) error {
	query := `
    CREATE TABLE IF NOT EXISTS user_preferences (
        user_id TEXT NOT NULL,
        guild_id TEXT NOT NULL,
        preferred_pools TEXT,
        PRIMARY KEY(user_id, guild_id)
    );`
	if _, err := db.Exec(query); err != nil {
		return err
	}

	// Check if the column already exists
	rows, err := db.Query("PRAGMA table_info(user_preferences);")
	if err != nil {
		return err
	}
	defer rows.Close()

	var columnExists bool
	for rows.Next() {
		var cid int
		var name string
		var type_ string
		var notnull bool
		var dflt_value sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &type_, &notnull, &dflt_value, &pk); err != nil {
			return err
		}
		if name == "skip_preset_confirmation" {
			columnExists = true
			break
		}
	}

	// If the column doesn't exist, add it.
	if !columnExists {
		log.Println("Column 'skip_preset_confirmation' not found, adding it to 'user_preferences' table.")
		alterQuery := `ALTER TABLE user_preferences ADD COLUMN skip_preset_confirmation BOOLEAN DEFAULT FALSE;`
		if _, err := db.Exec(alterQuery); err != nil {
			return fmt.Errorf("failed to add 'skip_preset_confirmation' column: %w", err)
		}
	}

	return nil
}

// GetUserPreferredPools retrieves the preferred pools for a given user in a specific guild.
func GetUserPreferredPools(userID, guildID string) ([]string, error) {
	db, err := InitUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	var preferredPoolsStr sql.NullString
	query := "SELECT preferred_pools FROM user_preferences WHERE user_id = ? AND guild_id = ?"
	err = db.QueryRow(query, userID, guildID).Scan(&preferredPoolsStr)

	if err != nil {
		if err == sql.ErrNoRows {
			return []string{}, nil // No preferences set yet for this guild
		}
		return nil, fmt.Errorf("failed to query user preferences for guild %s: %w", guildID, err)
	}

	if !preferredPoolsStr.Valid || preferredPoolsStr.String == "" {
		return []string{}, nil
	}

	return strings.Split(preferredPoolsStr.String, ","), nil
}

// SetUserPreferredPools sets or updates the preferred pools for a given user in a specific guild.
func SetUserPreferredPools(userID, guildID string, pools []string) error {
	db, err := InitUserDB()
	if err != nil {
		return fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	poolsStr := strings.Join(pools, ",")
	query := `
    INSERT INTO user_preferences (user_id, guild_id, preferred_pools)
    VALUES (?, ?, ?)
    ON CONFLICT(user_id, guild_id) DO UPDATE SET preferred_pools = excluded.preferred_pools;`

	_, err = db.Exec(query, userID, guildID, poolsStr)
	if err != nil {
		return fmt.Errorf("failed to set user preferences for guild %s: %w", guildID, err)
	}

	return nil
}

// GetTotalUserCount retrieves the total number of unique users from the database.
func GetTotalUserCount() (int, error) {
	db, err := InitUserDB()
	if err != nil {
		return 0, fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	var count int
	query := "SELECT COUNT(DISTINCT user_id) FROM user_preferences"
	err = db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to query total user count: %w", err)
	}

	return count, nil
}

// GetUserPresetConfirmationPreference retrieves the user's preference for skipping preset confirmation.
func GetUserPresetConfirmationPreference(userID, guildID string) (bool, error) {
	db, err := InitUserDB()
	if err != nil {
		return false, fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	var skipConfirmation sql.NullBool
	query := "SELECT skip_preset_confirmation FROM user_preferences WHERE user_id = ? AND guild_id = ?"
	err = db.QueryRow(query, userID, guildID).Scan(&skipConfirmation)

	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Default to not skipping
		}
		return false, fmt.Errorf("failed to query user preference: %w", err)
	}

	return skipConfirmation.Valid && skipConfirmation.Bool, nil
}

// SetUserPresetConfirmationPreference sets the user's preference for skipping preset confirmation.
func SetUserPresetConfirmationPreference(userID, guildID string, skip bool) error {
	db, err := InitUserDB()
	if err != nil {
		return fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	query := `
    INSERT INTO user_preferences (user_id, guild_id, skip_preset_confirmation)
    VALUES (?, ?, ?)
    ON CONFLICT(user_id, guild_id) DO UPDATE SET skip_preset_confirmation = excluded.skip_preset_confirmation;`

	_, err = db.Exec(query, userID, guildID, skip)
	if err != nil {
		return fmt.Errorf("failed to set user preference: %w", err)
	}

	return nil
}
