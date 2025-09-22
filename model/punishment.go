package model

import "time"

// TempRoleRemoval represents a temporary role that needs to be removed at a specific time
type TempRoleRemoval struct {
	RoleID   string    `json:"role_id"`
	RemoveAt time.Time `json:"remove_at"`
}

// PunishmentRecord represents a single punishment record in the database.
// The database table will be named 'punishments'.
type PunishmentRecord struct {
	PunishmentID      int64  `db:"punishment_id"`      // Primary Key, Auto-increment
	MessageID         string `db:"message_id"`
	AdminID           string `db:"admin_id"`
	UserID            string `db:"user_id"`
	UserUsername      string `db:"user_username"`
	Reason            string `db:"reason"`
	GuildID           string `db:"guild_id"`
	Timestamp         int64  `db:"timestamp"`
	Evidence          string `db:"evidence"`           // JSON string with message content and file paths
	ActionType        string `db:"action_type"`        // Type of punishment action (e.g., "re-answer", "cheat", "tag")
	TempRolesJSON     string `db:"temp_roles_json"`    // JSON array of temporary role IDs added by this punishment
	RolesRemoveAt     string `db:"roles_remove_at"`    // JSON object mapping role IDs to their removal timestamps
	PunishmentStatus  string `db:"punishment_status"`  // Status: active, completed, cancelled
}
