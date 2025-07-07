package utils

import (
	"database/sql"
	"discord-bot/model"
	"strings"
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

func GetAllPosts(db *sql.DB, tableName string) ([]model.Post, error) {
	rows, err := db.Query(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "` + tableName + `"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(&post.ID, &post.Title, &post.Author, &post.AuthorID, &post.Content, &post.Tags, &post.MessageCount, &post.Timestamp, &post.CoverImageURL); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func GetRandomPosts(db *sql.DB, tableName string, count int) ([]model.Post, error) {
	rows, err := db.Query(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "`+tableName+`" ORDER BY RANDOM() LIMIT ?`, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(&post.ID, &post.Title, &post.Author, &post.AuthorID, &post.Content, &post.Tags, &post.MessageCount, &post.Timestamp, &post.CoverImageURL); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func GetRandomPostsByTag(db *sql.DB, tableName string, tagID string, count int) ([]model.Post, error) {
	query := `SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "` + tableName + `" WHERE tags LIKE ? ORDER BY RANDOM() LIMIT ?`
	rows, err := db.Query(query, "%"+tagID+"%", count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(&post.ID, &post.Title, &post.Author, &post.AuthorID, &post.Content, &post.Tags, &post.MessageCount, &post.Timestamp, &post.CoverImageURL); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}
func GetRandomPostsFromAllTables(db *sql.DB, count int) ([]model.Post, error) {
	// 1. Get all table names
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	if len(tableNames) == 0 {
		return []model.Post{}, nil // No tables in the database
	}

	// 2. Build a UNION ALL query
	var allPostsQuery strings.Builder
	for i, tableName := range tableNames {
		allPostsQuery.WriteString(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "`)
		allPostsQuery.WriteString(tableName)
		allPostsQuery.WriteString(`"`)
		if i < len(tableNames)-1 {
			allPostsQuery.WriteString(" UNION ALL ")
		}
	}

	// 3. Execute the final query to get random posts
	finalQuery := `SELECT * FROM (` + allPostsQuery.String() + `) ORDER BY RANDOM() LIMIT ?`
	rows, err = db.Query(finalQuery, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 4. Scan the results
	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(&post.ID, &post.Title, &post.Author, &post.AuthorID, &post.Content, &post.Tags, &post.MessageCount, &post.Timestamp, &post.CoverImageURL); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func GetRandomPostsByTagFromAllTables(db *sql.DB, tagID string, count int) ([]model.Post, error) {
	// 1. Get all table names
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	if len(tableNames) == 0 {
		return []model.Post{}, nil // No tables in the database
	}

	// 2. Build a UNION ALL query
	var allPostsQuery strings.Builder
	var queryArgs []interface{}
	for i, tableName := range tableNames {
		if i > 0 {
			allPostsQuery.WriteString(" UNION ALL ")
		}
		allPostsQuery.WriteString(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "`)
		allPostsQuery.WriteString(tableName)
		allPostsQuery.WriteString(`" WHERE tags LIKE ?`)
		queryArgs = append(queryArgs, "%"+tagID+"%")
	}

	if allPostsQuery.Len() == 0 {
		return []model.Post{}, nil
	}

	// 3. Execute the final query to get random posts
	finalQuery := `SELECT * FROM (` + allPostsQuery.String() + `) ORDER BY RANDOM() LIMIT ?`
	queryArgs = append(queryArgs, count)

	rows, err = db.Query(finalQuery, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 4. Scan the results
	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(&post.ID, &post.Title, &post.Author, &post.AuthorID, &post.Content, &post.Tags, &post.MessageCount, &post.Timestamp, &post.CoverImageURL); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}
