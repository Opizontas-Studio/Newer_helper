package utils

import (
	"database/sql"
	"discord-bot/model"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(filepath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func CreateTable(db *sql.DB, tableName string) error {
	createTableSQL := `CREATE TABLE IF NOT EXISTS "` + tableName + `" (
		"id" TEXT NOT NULL PRIMARY KEY,
		"title" TEXT,
		"author" TEXT,
		"author_id" TEXT,
		"content" TEXT,
		"tags" TEXT,
		"message_count" INTEGER,
		"timestamp" INTEGER,
		"cover_image_url" TEXT
	);`

	_, err := db.Exec(createTableSQL)
	return err
}

func InsertPost(db *sql.DB, post model.Post, tableName string) error {
	if err := CreateTable(db, tableName); err != nil {
		return err
	}

	insertSQL := `INSERT OR REPLACE INTO "` + tableName + `"(id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(insertSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(post.ID, post.Title, post.Author, post.AuthorID, post.Content, post.Tags, post.MessageCount, post.Timestamp, post.CoverImageURL)
	return err
}
