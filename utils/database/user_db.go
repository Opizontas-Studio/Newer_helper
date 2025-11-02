package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"newer_helper/model"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	userDbPath = "./data/user.db"
	userDbDSN  = "file:./data/user.db?_busy_timeout=8000&_journal_mode=WAL&_sync=NORMAL"
)

var (
	userDBEnsureOnce sync.Once
	userDBEnsureErr  error
)

// InitUserDB initializes the user database.
// It creates the database file and the necessary tables if they don't exist.
func InitUserDB() (*sql.DB, error) {
	// Ensure the data directory exists
	if err := os.MkdirAll("./data", os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sql.Open("sqlite3", userDbDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open user database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Printf("Failed to enable foreign keys on user.db: %v", err)
	}

	userDBEnsureOnce.Do(func() {
		start := time.Now()
		log.Printf("personal-nav: ensuring user db schema...")
		userDBEnsureErr = ensureUserTables(db)
		if userDBEnsureErr != nil {
			log.Printf("personal-nav: ensure user db schema failed after %s: %v", time.Since(start), userDBEnsureErr)
		} else {
			log.Printf("personal-nav: ensure user db schema completed in %s", time.Since(start))
		}
	})
	if userDBEnsureErr != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create user tables: %w", userDBEnsureErr)
	}

	return db, nil
}

// ensureUserTables ensures required tables and columns exist, retrying if locked.
func ensureUserTables(db *sql.DB) error {
	log.Println("personal-nav: ensureUserTables called")
	satisfied, err := userSchemaSatisfied(db)
	if err != nil {
		log.Printf("personal-nav: schema verification failed, attempting migration: %v", err)
	} else if satisfied {
		log.Printf("personal-nav: user db schema already satisfied, skipping migrations.")
		return nil
	}

	log.Println("personal-nav: schema not satisfied, proceeding with table creation/migration...")
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Printf("personal-nav: createOrUpdateUserTables attempt %d/%d", attempt, maxAttempts)
		if err := createOrUpdateUserTables(db); err != nil {
			if isSQLiteBusyError(err) && attempt < maxAttempts {
				log.Printf("personal-nav: ensure schema attempt %d hit busy, retrying...", attempt)
				time.Sleep(time.Duration(attempt) * 150 * time.Millisecond)
				continue
			}
			log.Printf("personal-nav: createOrUpdateUserTables failed: %v", err)
			return err
		}
		log.Println("personal-nav: createOrUpdateUserTables succeeded")
		return nil
	}
	return fmt.Errorf("unable to ensure user tables after retries")
}

// createOrUpdateUserTables creates and alters the necessary tables in the user database.
func createOrUpdateUserTables(db *sql.DB) error {
	log.Println("personal-nav: createOrUpdateUserTables - creating user_preferences table...")
	query := `
    CREATE TABLE IF NOT EXISTS user_preferences (
        user_id TEXT NOT NULL,
        guild_id TEXT NOT NULL,
        preferred_pools TEXT,
        PRIMARY KEY(user_id, guild_id)
    );`
	if _, err := db.Exec(query); err != nil {
		log.Printf("personal-nav: failed to create user_preferences table: %v", err)
		return err
	}
	log.Println("personal-nav: user_preferences table OK")

	// Check if the column already exists
	log.Println("personal-nav: checking skip_preset_confirmation column...")
	rows, err := db.Query("PRAGMA table_info(user_preferences);")
	if err != nil {
		log.Printf("personal-nav: failed to query user_preferences table info: %v", err)
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
			log.Printf("personal-nav: failed to scan user_preferences column info: %v", err)
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
		log.Println("personal-nav: skip_preset_confirmation column added")
	} else {
		log.Println("personal-nav: skip_preset_confirmation column already exists")
	}

	// Check if personal_navigation table needs migration (adding id column)
	needsMigration, err := personalNavigationNeedsMigration(db)
	if err != nil {
		return fmt.Errorf("failed to check personal_navigation migration status: %w", err)
	}
	if needsMigration {
		log.Println("personal-nav: migrating personal_navigation table to add id column...")
		if err := migratePersonalNavigationTable(db); err != nil {
			return fmt.Errorf("failed to migrate personal_navigation table: %w", err)
		}
		log.Println("personal-nav: migration completed successfully")
	} else {
		log.Println("personal-nav: no migration needed, ensuring table exists with new schema...")
		// Create table with new schema if it doesn't exist
		personalNavQuery := `
		CREATE TABLE IF NOT EXISTS personal_navigation (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			guild_id TEXT NOT NULL,
			nav_id INTEGER NOT NULL,
			channel_id TEXT NOT NULL,
			table_name TEXT NOT NULL,
			channel_name TEXT,
			message_channel_id TEXT NOT NULL,
			message_id_my_works TEXT NOT NULL,
			message_id_top_works TEXT NOT NULL,
			message_id_latest_works TEXT NOT NULL,
			update_mode TEXT DEFAULT 'edit',
			UNIQUE(user_id, guild_id, nav_id)
		);`
		if _, err := db.Exec(personalNavQuery); err != nil {
			return fmt.Errorf("failed to ensure personal_navigation table: %w", err)
		}
		log.Println("personal-nav: personal_navigation table OK")
	}

	return nil
}

func ensureColumnExists(db *sql.DB, table, column, alterStmt string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s);", table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var type_ string
		var notnull bool
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &type_, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}

	if alterStmt == "" {
		return nil
	}

	if _, err := db.Exec(alterStmt); err != nil {
		return fmt.Errorf("failed to add '%s' column to '%s': %w", column, table, err)
	}
	return nil
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "database is busy")
}

func userSchemaSatisfied(db *sql.DB) (bool, error) {
	log.Println("personal-nav: checking if user db schema is satisfied...")
	requiredTables := map[string][]string{
		"user_preferences": {
			"user_id", "guild_id", "preferred_pools", "skip_preset_confirmation",
		},
		"personal_navigation": {
			"id", "user_id", "guild_id", "nav_id", "channel_id", "table_name",
			"channel_name", "message_channel_id", "message_id_my_works",
			"message_id_top_works", "message_id_latest_works", "update_mode",
		},
	}

	for table, columns := range requiredTables {
		log.Printf("personal-nav: checking table %s...", table)
		exists, err := tableExists(db, table)
		if err != nil {
			log.Printf("personal-nav: error checking if table %s exists: %v", table, err)
			return false, err
		}
		if !exists {
			log.Printf("personal-nav: table %s does not exist", table)
			return false, nil
		}
		ok, err := tableHasColumns(db, table, columns)
		if err != nil {
			log.Printf("personal-nav: error checking columns for table %s: %v", table, err)
			return false, err
		}
		if !ok {
			log.Printf("personal-nav: table %s is missing required columns", table)
			return false, nil
		}
		log.Printf("personal-nav: table %s is OK", table)
	}
	log.Println("personal-nav: schema is satisfied")
	return true, nil
}

func tableExists(db *sql.DB, table string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func tableHasColumns(db *sql.DB, table string, columns []string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s);", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	found := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var type_ string
		var notnull bool
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &type_, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		found[name] = true
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	for _, col := range columns {
		if !found[col] {
			return false, nil
		}
	}
	return true, nil
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

// GetPersonalNavigations returns all personal navigations for a user within a guild ordered by nav_id.
func GetPersonalNavigations(userID, guildID string) ([]model.PersonalNavigation, error) {
	log.Printf("personal-nav: db -> GetPersonalNavigations guild=%s user=%s", guildID, userID)
	db, err := InitUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT id, nav_id, channel_id, table_name, COALESCE(channel_name, ''), message_channel_id, message_id_my_works, message_id_top_works, message_id_latest_works, COALESCE(update_mode, 'edit')
		FROM personal_navigation
		WHERE user_id = ? AND guild_id = ?
		ORDER BY nav_id ASC`, userID, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to query personal navigation: %w", err)
	}
	defer rows.Close()

	var entries []model.PersonalNavigation
	for rows.Next() {
		var nav model.PersonalNavigation
		if err := rows.Scan(&nav.ID, &nav.NavID, &nav.ChannelID, &nav.TableName, &nav.ChannelName, &nav.MessageChannelID, &nav.MessageIDMyWorks, &nav.MessageIDTopWorks, &nav.MessageIDLatestWorks, &nav.UpdateMode); err != nil {
			return nil, err
		}
		nav.UserID = userID
		nav.GuildID = guildID
		entries = append(entries, nav)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	log.Printf("personal-nav: db <- GetPersonalNavigations guild=%s user=%s count=%d", guildID, userID, len(entries))
	return entries, nil
}

// GetPersonalNavigation retrieves a single navigation entry.
func GetPersonalNavigation(userID, guildID string, navID int) (*model.PersonalNavigation, error) {
	db, err := InitUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	row := db.QueryRow(`
		SELECT id, channel_id, table_name, COALESCE(channel_name, ''), message_channel_id, message_id_my_works, message_id_top_works, message_id_latest_works, COALESCE(update_mode, 'edit')
		FROM personal_navigation
		WHERE user_id = ? AND guild_id = ? AND nav_id = ?`, userID, guildID, navID)

	var nav model.PersonalNavigation
	nav.UserID = userID
	nav.GuildID = guildID
	nav.NavID = navID
	err = row.Scan(&nav.ID, &nav.ChannelID, &nav.TableName, &nav.ChannelName, &nav.MessageChannelID, &nav.MessageIDMyWorks, &nav.MessageIDTopWorks, &nav.MessageIDLatestWorks, &nav.UpdateMode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &nav, nil
}

// UpsertPersonalNavigation inserts or updates a navigation entry.
func UpsertPersonalNavigation(nav model.PersonalNavigation) error {
	db, err := InitUserDB()
	if err != nil {
		return fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	// Note: id is not included in INSERT because it's AUTOINCREMENT
	// ON CONFLICT will preserve the existing id when updating
	query := `
	INSERT INTO personal_navigation (
		user_id, guild_id, nav_id, channel_id, table_name, channel_name, message_channel_id,
		message_id_my_works, message_id_top_works, message_id_latest_works, update_mode
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, guild_id, nav_id) DO UPDATE SET
		channel_id = excluded.channel_id,
		table_name = excluded.table_name,
		channel_name = excluded.channel_name,
		message_channel_id = excluded.message_channel_id,
		message_id_my_works = excluded.message_id_my_works,
		message_id_top_works = excluded.message_id_top_works,
		message_id_latest_works = excluded.message_id_latest_works,
		update_mode = excluded.update_mode;
	`

	_, err = db.Exec(query,
		nav.UserID,
		nav.GuildID,
		nav.NavID,
		nav.ChannelID,
		nav.TableName,
		nav.ChannelName,
		nav.MessageChannelID,
		nav.MessageIDMyWorks,
		nav.MessageIDTopWorks,
		nav.MessageIDLatestWorks,
		nav.UpdateMode,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert personal navigation: %w", err)
	}
	return nil
}

// DeletePersonalNavigation removes a navigation entry by user_id, guild_id, and nav_id.
func DeletePersonalNavigation(userID, guildID string, navID int) error {
	db, err := InitUserDB()
	if err != nil {
		return fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	_, err = db.Exec(`DELETE FROM personal_navigation WHERE user_id = ? AND guild_id = ? AND nav_id = ?`, userID, guildID, navID)
	if err != nil {
		return fmt.Errorf("failed to delete personal navigation: %w", err)
	}
	return nil
}

// GetPersonalNavigationByID retrieves a navigation entry by its unique ID.
func GetPersonalNavigationByID(id int64) (*model.PersonalNavigation, error) {
	db, err := InitUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	row := db.QueryRow(`
		SELECT id, user_id, guild_id, nav_id, channel_id, table_name,
			   COALESCE(channel_name, ''), message_channel_id,
			   message_id_my_works, message_id_top_works, message_id_latest_works,
			   COALESCE(update_mode, 'edit')
		FROM personal_navigation
		WHERE id = ?`, id)

	var nav model.PersonalNavigation
	err = row.Scan(&nav.ID, &nav.UserID, &nav.GuildID, &nav.NavID, &nav.ChannelID,
		&nav.TableName, &nav.ChannelName, &nav.MessageChannelID,
		&nav.MessageIDMyWorks, &nav.MessageIDTopWorks, &nav.MessageIDLatestWorks,
		&nav.UpdateMode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &nav, nil
}

// DeletePersonalNavigationByID removes a navigation entry by its unique ID.
func DeletePersonalNavigationByID(id int64) error {
	db, err := InitUserDB()
	if err != nil {
		return fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	result, err := db.Exec(`DELETE FROM personal_navigation WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete personal navigation: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no navigation found with id %d", id)
	}

	return nil
}

// GetPersonalNavigationByMessageID finds the navigation entry that contains the given message ID.
func GetPersonalNavigationByMessageID(guildID, messageID string) (*model.PersonalNavigation, error) {
	db, err := InitUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	row := db.QueryRow(`
		SELECT id, user_id, nav_id, channel_id, table_name, COALESCE(channel_name, ''),
			   message_channel_id,
			   message_id_my_works, message_id_top_works, message_id_latest_works, COALESCE(update_mode, 'edit')
		FROM personal_navigation
		WHERE guild_id = ? AND (
			message_id_my_works LIKE '%' || ? || '%' OR
			message_id_top_works = ? OR
			message_id_latest_works = ?
		)`,
		guildID, messageID, messageID, messageID)

	var nav model.PersonalNavigation
	nav.GuildID = guildID
	err = row.Scan(&nav.ID, &nav.UserID, &nav.NavID, &nav.ChannelID, &nav.TableName, &nav.ChannelName, &nav.MessageChannelID, &nav.MessageIDMyWorks, &nav.MessageIDTopWorks, &nav.MessageIDLatestWorks, &nav.UpdateMode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &nav, nil
}

// GetAllPersonalNavigations retrieves all personal navigation entries across all users and guilds.
// Used for scheduled batch updates.
func GetAllPersonalNavigations() ([]model.PersonalNavigation, error) {
	log.Printf("personal-nav: db -> GetAllPersonalNavigations")
	db, err := InitUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT id, user_id, guild_id, nav_id, channel_id, table_name,
			   COALESCE(channel_name, ''), message_channel_id,
			   message_id_my_works, message_id_top_works, message_id_latest_works,
			   COALESCE(update_mode, 'edit')
		FROM personal_navigation
		ORDER BY guild_id, user_id, nav_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query all personal navigations: %w", err)
	}
	defer rows.Close()

	var entries []model.PersonalNavigation
	for rows.Next() {
		var nav model.PersonalNavigation
		if err := rows.Scan(&nav.ID, &nav.UserID, &nav.GuildID, &nav.NavID, &nav.ChannelID,
			&nav.TableName, &nav.ChannelName, &nav.MessageChannelID,
			&nav.MessageIDMyWorks, &nav.MessageIDTopWorks, &nav.MessageIDLatestWorks,
			&nav.UpdateMode); err != nil {
			return nil, err
		}
		entries = append(entries, nav)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	log.Printf("personal-nav: db <- GetAllPersonalNavigations count=%d", len(entries))
	return entries, nil
}

// personalNavigationNeedsMigration checks if the personal_navigation table needs migration
// (i.e., it exists but doesn't have the id column)
func personalNavigationNeedsMigration(db *sql.DB) (bool, error) {
	log.Println("personal-nav: checking if personal_navigation needs migration...")

	// Check if table exists
	exists, err := tableExists(db, "personal_navigation")
	if err != nil {
		log.Printf("personal-nav: error checking if table exists: %v", err)
		return false, err
	}
	if !exists {
		log.Println("personal-nav: table doesn't exist, no migration needed")
		return false, nil // Table doesn't exist, no migration needed
	}

	// Check if id column exists
	log.Println("personal-nav: checking if id column exists...")
	rows, err := db.Query("PRAGMA table_info(personal_navigation);")
	if err != nil {
		log.Printf("personal-nav: error querying table info: %v", err)
		return false, err
	}
	defer rows.Close()

	hasIDColumn := false
	for rows.Next() {
		var cid int
		var name string
		var type_ string
		var notnull bool
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &type_, &notnull, &dfltValue, &pk); err != nil {
			log.Printf("personal-nav: error scanning table info: %v", err)
			return false, err
		}
		if name == "id" {
			hasIDColumn = true
			break
		}
	}

	// Need migration if table exists but id column doesn't
	needsMigration := !hasIDColumn
	log.Printf("personal-nav: migration needed: %v (hasIDColumn: %v)", needsMigration, hasIDColumn)
	return needsMigration, nil
}

// migratePersonalNavigationTable migrates the personal_navigation table to add id column
func migratePersonalNavigationTable(db *sql.DB) error {
	log.Println("personal-nav: starting table migration...")

	// First, ensure all required columns exist in the old table before migration
	log.Println("personal-nav: ensuring old table has all columns before migration...")
	if err := ensureColumnExists(db, "personal_navigation", "channel_name", "ALTER TABLE personal_navigation ADD COLUMN channel_name TEXT;"); err != nil {
		return fmt.Errorf("failed to add channel_name column: %w", err)
	}
	if err := ensureColumnExists(db, "personal_navigation", "message_channel_id", "ALTER TABLE personal_navigation ADD COLUMN message_channel_id TEXT DEFAULT '';"); err != nil {
		return fmt.Errorf("failed to add message_channel_id column: %w", err)
	}
	if err := ensureColumnExists(db, "personal_navigation", "update_mode", "ALTER TABLE personal_navigation ADD COLUMN update_mode TEXT DEFAULT 'edit';"); err != nil {
		return fmt.Errorf("failed to add update_mode column: %w", err)
	}
	log.Println("personal-nav: old table columns verified")

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Create new table with id column
	log.Println("personal-nav: creating new table with id column...")
	createNewTable := `
	CREATE TABLE personal_navigation_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		guild_id TEXT NOT NULL,
		nav_id INTEGER NOT NULL,
		channel_id TEXT NOT NULL,
		table_name TEXT NOT NULL,
		channel_name TEXT,
		message_channel_id TEXT NOT NULL,
		message_id_my_works TEXT NOT NULL,
		message_id_top_works TEXT NOT NULL,
		message_id_latest_works TEXT NOT NULL,
		update_mode TEXT DEFAULT 'edit',
		UNIQUE(user_id, guild_id, nav_id)
	);`
	if _, err := tx.Exec(createNewTable); err != nil {
		return fmt.Errorf("failed to create new table: %w", err)
	}
	log.Println("personal-nav: new table created")

	// Copy data from old table to new table
	log.Println("personal-nav: copying data from old table to new table...")
	copyData := `
	INSERT INTO personal_navigation_new
		(user_id, guild_id, nav_id, channel_id, table_name, channel_name,
		 message_channel_id, message_id_my_works, message_id_top_works,
		 message_id_latest_works, update_mode)
	SELECT
		user_id, guild_id, nav_id, channel_id, table_name,
		COALESCE(channel_name, ''),
		COALESCE(message_channel_id, ''),
		message_id_my_works, message_id_top_works, message_id_latest_works,
		COALESCE(update_mode, 'edit')
	FROM personal_navigation;`
	result, err := tx.Exec(copyData)
	if err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	log.Printf("personal-nav: migrated %d rows of data", rowsAffected)

	// Drop old table
	if _, err := tx.Exec("DROP TABLE personal_navigation;"); err != nil {
		return fmt.Errorf("failed to drop old table: %w", err)
	}
	log.Println("personal-nav: dropped old table")

	// Rename new table to original name
	if _, err := tx.Exec("ALTER TABLE personal_navigation_new RENAME TO personal_navigation;"); err != nil {
		return fmt.Errorf("failed to rename new table: %w", err)
	}
	log.Println("personal-nav: renamed new table")

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Println("personal-nav: migration transaction committed successfully")
	return nil
}
