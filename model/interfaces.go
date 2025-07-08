package model

import (
	"database/sql"

	"github.com/bwmarrin/discordgo"
)

// Bot provides an interface for bot functionality to avoid circular dependencies.
type Bot interface {
	GetConfig() *Config
	GetSession() *discordgo.Session
	GetDB() *sql.DB
}
