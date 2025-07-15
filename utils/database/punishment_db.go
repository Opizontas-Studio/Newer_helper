package database

import (
	"discord-bot/model"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// InitPunishmentDB initializes the punishment database and ensures the table exists.
func InitPunishmentDB(dbPath string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to punishment database: %w", err)
	}

	schema := `
    CREATE TABLE IF NOT EXISTS punishments (
        punishment_id INTEGER PRIMARY KEY AUTOINCREMENT,
        message_id TEXT NOT NULL,
        admin_id TEXT NOT NULL,
        user_id TEXT NOT NULL,
        user_username TEXT NOT NULL,
        reason TEXT NOT NULL
    );`

	_, err = db.Exec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create punishments table: %w", err)
	}

	return db, nil
}

// AddPunishmentRecord adds a new punishment record to the database.
func AddPunishmentRecord(db *sqlx.DB, record model.PunishmentRecord) error {
	query := `INSERT INTO punishments (message_id, admin_id, user_id, user_username, reason)
              VALUES (:message_id, :admin_id, :user_id, :user_username, :reason)`

	_, err := db.NamedExec(query, record)
	if err != nil {
		return fmt.Errorf("failed to insert punishment record: %w", err)
	}

	return nil
}

// GetPunishmentRecordsByUserID retrieves all punishment records for a specific user.
func GetPunishmentRecordsByUserID(db *sqlx.DB, userID string) ([]model.PunishmentRecord, error) {
	var records []model.PunishmentRecord
	query := "SELECT * FROM punishments WHERE user_id = ?"
	err := db.Select(&records, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get punishment records for user %s: %w", userID, err)
	}
	return records, nil
}
