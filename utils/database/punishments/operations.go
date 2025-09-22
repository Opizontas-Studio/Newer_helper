package punishments

import (
	"discord-bot/model"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// AddPunishmentRecord adds a new punishment record to the database and returns the new record's ID.
func AddPunishmentRecord(db *sqlx.DB, record model.PunishmentRecord) (int64, error) {
	query := `INSERT INTO punishments (message_id, admin_id, user_id, user_username, reason, guild_id, timestamp, evidence, action_type, temp_roles_json, roles_remove_at, punishment_status)
			  VALUES (:message_id, :admin_id, :user_id, :user_username, :reason, :guild_id, :timestamp, :evidence, :action_type, :temp_roles_json, :roles_remove_at, :punishment_status)`

	result, err := db.NamedExec(query, record)
	if err != nil {
		return 0, fmt.Errorf("failed to insert punishment record: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
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

// GetLatestPunishmentByUserID retrieves the latest punishment record for a specific user in a specific guild.
func GetLatestPunishmentByUserID(db *sqlx.DB, guildID, userID string) (*model.PunishmentRecord, error) {
	var record model.PunishmentRecord
	query := "SELECT * FROM punishments WHERE guild_id = ? AND user_id = ? ORDER BY timestamp DESC LIMIT 1"
	err := db.Get(&record, query, guildID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest punishment record for user %s in guild %s: %w", userID, guildID, err)
	}
	return &record, nil
}

// GetAdminPunishmentStats retrieves the punishment count for each admin within a given time range.
func GetAdminPunishmentStats(db *sqlx.DB, guildID string, since time.Time) (map[string]int, error) {
	query := `SELECT admin_id, COUNT(*) as count FROM punishments WHERE guild_id = ? AND timestamp >= ? GROUP BY admin_id ORDER BY count DESC`
	rows, err := db.Query(query, guildID, since.Unix())
	if err != nil {
		return nil, fmt.Errorf("failed to get admin punishment stats for guild %s: %w", guildID, err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var adminID string
		var count int
		if err := rows.Scan(&adminID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan admin punishment stats row: %w", err)
		}
		stats[adminID] = count
	}
	return stats, nil
}

// GetTotalPunishmentCount retrieves the total number of punishments within a given time range.
func GetTotalPunishmentCount(db *sqlx.DB, guildID string, since time.Time) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM punishments WHERE guild_id = ? AND timestamp >= ?`
	err := db.Get(&count, query, guildID, since.Unix())
	if err != nil {
		return 0, fmt.Errorf("failed to get total punishment count for guild %s: %w", guildID, err)
	}
	return count, nil
}

// GetPunishmentCountByAction retrieves the number of punishments for a specific user and action type within a given time range.
func GetPunishmentCountByAction(db *sqlx.DB, guildID, userID, actionType string, since time.Time) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM punishments WHERE guild_id = ? AND user_id = ? AND action_type = ? AND timestamp >= ?`
	err := db.Get(&count, query, guildID, userID, actionType, since.Unix())
	if err != nil {
		return 0, fmt.Errorf("failed to get punishment count for user %s with action %s in guild %s: %w", userID, actionType, guildID, err)
	}
	return count, nil
}

// GetActivePunishments retrieves all active punishment records that have temporary roles.
func GetActivePunishments(db *sqlx.DB) ([]model.PunishmentRecord, error) {
	var records []model.PunishmentRecord
	query := `SELECT * FROM punishments
			  WHERE punishment_status = 'active'
			  AND roles_remove_at != '{}'
			  AND roles_remove_at != ''`
	err := db.Select(&records, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active punishments: %w", err)
	}
	return records, nil
}

// UpdatePunishmentStatus updates the status of a punishment record.
func UpdatePunishmentStatus(db *sqlx.DB, punishmentID int64, status string) error {
	query := "UPDATE punishments SET punishment_status = ? WHERE punishment_id = ?"
	result, err := db.Exec(query, status, punishmentID)
	if err != nil {
		return fmt.Errorf("failed to update punishment status for ID %d: %w", punishmentID, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected for punishment ID %d: %w", punishmentID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no punishment found with ID %d", punishmentID)
	}
	return nil
}

// RemoveExpiredRoleFromPunishment removes a specific role from the roles_remove_at JSON of a punishment.
func RemoveExpiredRoleFromPunishment(db *sqlx.DB, punishmentID int64, roleID string, newRolesRemoveAtJSON string) error {
	query := "UPDATE punishments SET roles_remove_at = ? WHERE punishment_id = ?"
	result, err := db.Exec(query, newRolesRemoveAtJSON, punishmentID)
	if err != nil {
		return fmt.Errorf("failed to update roles_remove_at for punishment ID %d: %w", punishmentID, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected for punishment ID %d: %w", punishmentID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no punishment found with ID %d", punishmentID)
	}
	return nil
}
