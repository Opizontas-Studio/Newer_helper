package database

import (
	"discord-bot/model"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// InitPunishmentDB initializes the punishment database and ensures the table exists.
func InitPunishmentDB(dbPath string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to punishment database: %w", err)
	}

	schema := `CREATE TABLE IF NOT EXISTS punishments (
	          punishment_id INTEGER PRIMARY KEY AUTOINCREMENT,
	          message_id TEXT NOT NULL,
	          admin_id TEXT NOT NULL,
	          user_id TEXT NOT NULL,
	          user_username TEXT NOT NULL,
	          reason TEXT NOT NULL,
	          guild_id TEXT NOT NULL,
	          timestamp INTEGER NOT NULL,
	          evidence TEXT
	      );`
	_, err = db.Exec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create punishments table: %w", err)
	}

	return db, nil
}

// AddPunishmentRecord adds a new punishment record to the database.
func AddPunishmentRecord(db *sqlx.DB, record model.PunishmentRecord) error {
	query := `INSERT INTO punishments (message_id, admin_id, user_id, user_username, reason, guild_id, timestamp, evidence) VALUES (:message_id, :admin_id, :user_id, :user_username, :reason, :guild_id, :timestamp, :evidence)`

	_, err := db.NamedExec(query, record)
	if err != nil {
		return fmt.Errorf("failed to insert punishment record: %w", err)
	}

	return nil
}

// GetPunishmentRecordsByUserID retrieves punishment records for a specific user, optionally filtered by a start time.
func GetPunishmentRecordsByUserID(db *sqlx.DB, userID string, since *time.Time) ([]model.PunishmentRecord, error) {
	var records []model.PunishmentRecord
	query := "SELECT * FROM punishments WHERE user_id = ?"
	args := []interface{}{userID}

	if since != nil {
		query += " AND timestamp >= ?"
		args = append(args, since.Unix())
	}

	err := db.Select(&records, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get punishment records for user %s: %w", userID, err)
	}
	return records, nil
}

// GetPunishmentRecordByID retrieves a single punishment record by its primary key.
func GetPunishmentRecordByID(db *sqlx.DB, id int64) (*model.PunishmentRecord, error) {
	var record model.PunishmentRecord
	query := "SELECT * FROM punishments WHERE punishment_id = ?"
	err := db.Get(&record, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get punishment record by id %d: %w", id, err)
	}
	return &record, nil
}

// DeletePunishmentRecordByID deletes a punishment record by its primary key.
func DeletePunishmentRecordByID(db *sqlx.DB, id int64) error {
	query := "DELETE FROM punishments WHERE punishment_id = ?"
	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete punishment record by id %d: %w", id, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected for punishment id %d: %w", id, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no punishment record found with id %d", id)
	}
	return nil
}

// GetPunishmentRecordsByAdminID retrieves punishment records for a specific admin.
func GetPunishmentRecordsByAdminID(db *sqlx.DB, adminID string) ([]model.PunishmentRecord, error) {
	var records []model.PunishmentRecord
	query := "SELECT * FROM punishments WHERE admin_id = ?"
	err := db.Select(&records, query, adminID)
	if err != nil {
		return nil, fmt.Errorf("failed to get punishment records for admin %s: %w", adminID, err)
	}
	return records, nil
}

// GetAllPunishmentRecords retrieves all punishment records for a specific guild.
func GetAllPunishmentRecords(db *sqlx.DB, guildID string) ([]model.PunishmentRecord, error) {
	var records []model.PunishmentRecord
	query := "SELECT * FROM punishments WHERE guild_id = ?"
	err := db.Select(&records, query, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all punishment records for guild %s: %w", guildID, err)
	}
	return records, nil
}
