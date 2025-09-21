package model

// PunishmentRecord represents a single punishment record in the database.
// The database table will be named 'punishments'.
type PunishmentRecord struct {
	PunishmentID int64  `db:"punishment_id"` // Primary Key, Auto-increment
	MessageID    string `db:"message_id"`
	AdminID      string `db:"admin_id"`
	UserID       string `db:"user_id"`
	UserUsername string `db:"user_username"`
	Reason       string `db:"reason"`
	GuildID      string `db:"guild_id"`
	Timestamp    int64  `db:"timestamp"`
	Evidence     string `db:"evidence"`    // JSON string with message content and file paths
	ActionType   string `db:"action_type"` // Type of punishment action (e.g., "re-answer", "cheat", "tag")
}
