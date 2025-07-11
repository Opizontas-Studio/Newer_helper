package database

import (
	"database/sql"
	"discord-bot/model"
	"fmt"
	"os"
	"path/filepath"
)

func SaveNewPost(cfg *model.Config, post model.Post, guildID string, channelID string) error {
	threadGuildConfig, ok := cfg.ThreadConfig[guildID]
	if !ok {
		return fmt.Errorf("thread config not found for guild %s", guildID)
	}

	dbPath := threadGuildConfig.Database
	if dbPath == "" {
		return fmt.Errorf("thread database path not found for guild %s", guildID)
	}

	tableName := threadGuildConfig.TableName
	if tableName == "" {
		return fmt.Errorf("table name not found for guild %s", guildID)
	}

	// Ensure the directory exists before opening the database
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("error opening database %s: %w", dbPath, err)
	}
	defer db.Close()

	if err := InsertPost(db, post, tableName); err != nil {
		return fmt.Errorf("error inserting post %s into table `%s` in db `%s`: %w", post.ID, tableName, dbPath, err)
	}

	return nil
}
