package utils

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(filepath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// SetupDatabase initializes the database, creates tables, and returns the DB connection.
func SetupDatabase() (*sql.DB, error) {
	if err := os.MkdirAll("./data", os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	db, err := InitDB("./data/guilds.db")
	if err != nil {
		return nil, fmt.Errorf("error initializing database: %w", err)
	}
	if err := CreateGuildTables(db); err != nil {
		return nil, fmt.Errorf("error creating guild tables: %w", err)
	}
	return db, nil
}
