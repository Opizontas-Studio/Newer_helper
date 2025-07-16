package database

import (
	"database/sql"
	"discord-bot/model"
)

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

func DeletePost(db *sql.DB, tableName string, postID string) error {
	deleteSQL := `DELETE FROM "` + tableName + `" WHERE id = ?`
	stmt, err := db.Prepare(deleteSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(postID)
	return err
}

// DeletePostsOlderThan deletes posts from a table that are older than the given timestamp.
// It returns the number of posts deleted.
func DeletePostsOlderThan(db *sql.DB, tableName string, timestamp int64) (int64, error) {
	deleteSQL := `DELETE FROM "` + tableName + `" WHERE timestamp < ?`
	stmt, err := db.Prepare(deleteSQL)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	result, err := stmt.Exec(timestamp)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}
