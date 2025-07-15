package model

import "time"

// TimedTask represents a task to be executed at a specific time.
type TimedTask struct {
	ID       int64     `db:"id"`
	GuildID  string    `db:"guild_id"`
	UserID   string    `db:"user_id"`
	RoleID   string    `db:"role_id"`
	RemoveAt time.Time `db:"remove_at"`
}
