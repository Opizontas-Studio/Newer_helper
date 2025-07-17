package database

import (
	"discord-bot/model"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// InitTimedTaskDB initializes the timed task database and ensures the table exists.
func InitTimedTaskDB(dbPath string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to timed task database: %w", err)
	}

	schema := `
    CREATE TABLE IF NOT EXISTS timed_tasks (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        guild_id TEXT NOT NULL,
        user_id TEXT NOT NULL,
        role_id TEXT NOT NULL,
        remove_at DATETIME NOT NULL
    );`

	_, err = db.Exec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create timed_tasks table: %w", err)
	}

	return db, nil
}

// AddTimedTask adds a new timed task to the database.
func AddTimedTask(db *sqlx.DB, task model.TimedTask) error {
	query := `INSERT INTO timed_tasks (guild_id, user_id, role_id, remove_at)
              VALUES (:guild_id, :user_id, :role_id, :remove_at)`

	_, err := db.NamedExec(query, task)
	if err != nil {
		return fmt.Errorf("failed to insert timed task: %w", err)
	}
	return nil
}

// GetDueTasks retrieves all tasks that are due to be executed.
func GetDueTasks(db *sqlx.DB) ([]model.TimedTask, error) {
	var tasks []model.TimedTask
	query := "SELECT * FROM timed_tasks WHERE remove_at <= ?"
	err := db.Select(&tasks, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get due tasks: %w", err)
	}
	return tasks, nil
}

// DeleteTask deletes a task from the database by its ID.
func DeleteTask(db *sqlx.DB, taskID int64) error {
	query := "DELETE FROM timed_tasks WHERE id = ?"
	result, err := db.Exec(query, taskID)
	if err != nil {
		return fmt.Errorf("failed to delete task %d: %w", taskID, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected for task id %d: %w", taskID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no task found with id %d", taskID)
	}
	return nil
}

// DeleteTaskByDetails deletes a task from the database by guild, user, and role ID.
func DeleteTaskByDetails(db *sqlx.DB, guildID, userID, roleID string) error {
	query := "DELETE FROM timed_tasks WHERE guild_id = ? AND user_id = ? AND role_id = ?"
	_, err := db.Exec(query, guildID, userID, roleID)
	if err != nil {
		return fmt.Errorf("failed to delete task by details for user %s in guild %s: %w", userID, guildID, err)
	}
	return nil
}
