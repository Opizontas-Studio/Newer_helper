package utils

import (
	"database/sql"
	"fmt"
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

// createUserTables creates the necessary tables in the user database.
func createUserTables(db *sql.DB) error {
	query := `
    CREATE TABLE IF NOT EXISTS user_preferences (
        user_id TEXT NOT NULL,
        guild_id TEXT NOT NULL,
        preferred_pools TEXT,
        PRIMARY KEY(user_id, guild_id)
    );`
	_, err := db.Exec(query)
	return err
}

// GetUserPreferredPools retrieves the preferred pools for a given user in a specific guild.
func GetUserPreferredPools(userID, guildID string) ([]string, error) {
	db, err := InitUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	var preferredPoolsStr string
	query := "SELECT preferred_pools FROM user_preferences WHERE user_id = ? AND guild_id = ?"
	err = db.QueryRow(query, userID, guildID).Scan(&preferredPoolsStr)

	if err != nil {
		if err == sql.ErrNoRows {
			return []string{}, nil // No preferences set yet for this guild
		}
		return nil, fmt.Errorf("failed to query user preferences for guild %s: %w", guildID, err)
	}

	if preferredPoolsStr == "" {
		return []string{}, nil
	}

	return strings.Split(preferredPoolsStr, ","), nil
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
