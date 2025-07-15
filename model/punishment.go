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
}
