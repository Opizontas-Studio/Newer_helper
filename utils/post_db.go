package utils

import (
	"database/sql"
	"discord-bot/model"
	"fmt"
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

func GetRandomPostsByTag(db *sql.DB, tableName string, tagID string, count int, excludeTags []string) ([]model.Post, error) {
	var queryArgs []interface{}
	var whereClauses []string

	if tagID != "" {
		whereClauses = append(whereClauses, "tags LIKE ?")
		queryArgs = append(queryArgs, "%"+tagID+"%")
	}

	for _, excludedTag := range excludeTags {
		whereClauses = append(whereClauses, "tags NOT LIKE ?")
		queryArgs = append(queryArgs, "%"+excludedTag+"%")
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	query := `SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "` + tableName + `" ` + whereClause + ` ORDER BY RANDOM() LIMIT ?`
	queryArgs = append(queryArgs, count)

	rows, err := db.Query(query, queryArgs...)
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

func GetRandomPostsByTagFromAllTables(db *sql.DB, tagID string, count int, excludeTags []string) ([]model.Post, error) {
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

	var whereClauses []string
	var whereArgs []interface{}

	if tagID != "" {
		whereClauses = append(whereClauses, "tags LIKE ?")
		whereArgs = append(whereArgs, "%"+tagID+"%")
	}
	for _, excludedTag := range excludeTags {
		whereClauses = append(whereClauses, "tags NOT LIKE ?")
		whereArgs = append(whereArgs, "%"+excludedTag+"%")
	}
	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	for i, tableName := range tableNames {
		if i > 0 {
			allPostsQuery.WriteString(" UNION ALL ")
		}
		allPostsQuery.WriteString(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "`)
		allPostsQuery.WriteString(tableName)
		allPostsQuery.WriteString(`" `)
		allPostsQuery.WriteString(whereClause)
		queryArgs = append(queryArgs, whereArgs...)
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

func CountPostsInTimeRange(db *sql.DB, tableNames []string, startTime int64, endTime int64) (int, error) {
	if len(tableNames) == 0 {
		return 0, nil
	}

	var queryBuilder strings.Builder
	var queryArgs []interface{}

	queryBuilder.WriteString("SELECT SUM(count) FROM (")
	for i, tableName := range tableNames {
		queryBuilder.WriteString(`SELECT COUNT(*) as count FROM "`)
		queryBuilder.WriteString(tableName)
		queryBuilder.WriteString(`" WHERE timestamp >= ? AND timestamp < ?`)
		queryArgs = append(queryArgs, startTime, endTime)
		if i < len(tableNames)-1 {
			queryBuilder.WriteString(" UNION ALL ")
		}
	}
	queryBuilder.WriteString(")")

	var totalCount sql.NullInt64
	err := db.QueryRow(queryBuilder.String(), queryArgs...).Scan(&totalCount)
	if err != nil {
		return 0, err
	}
	if !totalCount.Valid {
		return 0, nil
	}

	return int(totalCount.Int64), nil
}

func GetLatestPosts(db *sql.DB, tableNames []string, count int) ([]model.Post, error) {
	if len(tableNames) == 0 {
		return []model.Post{}, nil
	}

	var allPostsQuery strings.Builder
	for i, tableName := range tableNames {
		query := fmt.Sprintf(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url, '%s' as tableName FROM "%s"`, tableName, tableName)
		allPostsQuery.WriteString(query)
		if i < len(tableNames)-1 {
			allPostsQuery.WriteString(" UNION ALL ")
		}
	}

	finalQuery := `SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url, tableName FROM (` + allPostsQuery.String() + `) ORDER BY timestamp DESC LIMIT ?`
	rows, err := db.Query(finalQuery, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(&post.ID, &post.Title, &post.Author, &post.AuthorID, &post.Content, &post.Tags, &post.MessageCount, &post.Timestamp, &post.CoverImageURL, &post.TableName); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}
