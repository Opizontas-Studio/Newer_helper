package punishments

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Init initializes the database and ensures all necessary tables are created.
func Init(dbPath string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create punishments table with new columns
	punishmentsSchema := `CREATE TABLE IF NOT EXISTS punishments (
	          punishment_id INTEGER PRIMARY KEY AUTOINCREMENT,
	          message_id TEXT NOT NULL,
	          admin_id TEXT NOT NULL,
	          user_id TEXT NOT NULL,
	          user_username TEXT NOT NULL,
	          reason TEXT NOT NULL,
	          guild_id TEXT NOT NULL,
	          timestamp INTEGER NOT NULL,
	          evidence TEXT,
		  action_type TEXT DEFAULT '',
		  temp_roles_json TEXT DEFAULT '[]',
		  roles_remove_at TEXT DEFAULT '{}',
		  punishment_status TEXT DEFAULT 'active'
	      );`
	_, err = db.Exec(punishmentsSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to create punishments table: %w", err)
	}

	// Add new columns if they don't exist (for migration from old schema)
	alterStatements := []string{
		`ALTER TABLE punishments ADD COLUMN action_type TEXT DEFAULT ''`,
		`ALTER TABLE punishments ADD COLUMN temp_roles_json TEXT DEFAULT '[]'`,
		`ALTER TABLE punishments ADD COLUMN roles_remove_at TEXT DEFAULT '{}'`,
		`ALTER TABLE punishments ADD COLUMN punishment_status TEXT DEFAULT 'active'`,
	}

	for _, stmt := range alterStatements {
		_, err = db.Exec(stmt)
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return nil, fmt.Errorf("failed to execute ALTER statement %s: %w", stmt, err)
		}
	}

	// Drop the old timed_tasks table since we're integrating it into punishments
	_, err = db.Exec(`DROP TABLE IF EXISTS timed_tasks`)
	if err != nil {
		return nil, fmt.Errorf("failed to drop timed_tasks table: %w", err)
	}

	return db, nil
}