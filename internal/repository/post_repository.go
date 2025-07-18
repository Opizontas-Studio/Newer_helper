package repository

import (
	"database/sql"
	"discord-bot/internal/database"
	"discord-bot/model"
	"fmt"
	"log"
	"strings"
)

// postRepository 帖子数据访问实现
type postRepository struct {
	dbService *database.Service
}

// NewPostRepository 创建新的帖子仓库实例
func NewPostRepository(dbService *database.Service) PostRepository {
	return &postRepository{
		dbService: dbService,
	}
}

// validateTableName 验证表名安全性，防止SQL注入
func (r *postRepository) validateTableName(tableName string) error {
	// 只允许字母、数字、下划线和中文字符
	for _, char := range tableName {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' ||
			char == '中' || char == '文' || char == '字' || char == '符' ||
			(char >= 0x4e00 && char <= 0x9fff)) { // 中文字符范围
			return fmt.Errorf("invalid table name: %s", tableName)
		}
	}

	// 检查表名长度
	if len(tableName) == 0 || len(tableName) > 64 {
		return fmt.Errorf("table name length must be between 1 and 64 characters")
	}

	return nil
}

// GetByID 通过ID获取帖子
func (r *postRepository) GetByID(guildID, tableName, postID string) (*model.Post, error) {
	if err := r.validateTableName(tableName); err != nil {
		return nil, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}

	// 使用参数化查询，表名通过验证后拼接
	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		WHERE id = ?`, tableName)

	var post model.Post
	err = db.QueryRow(query, postID).Scan(
		&post.ID, &post.Title, &post.Author, &post.AuthorID,
		&post.Content, &post.Tags, &post.MessageCount,
		&post.Timestamp, &post.CoverImageURL,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get post by id: %w", err)
	}

	return &post, nil
}

// GetAll 获取所有帖子
func (r *postRepository) GetAll(guildID, tableName string) ([]model.Post, error) {
	if err := r.validateTableName(tableName); err != nil {
		return nil, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		ORDER BY timestamp DESC`, tableName)

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query posts: %w", err)
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(
			&post.ID, &post.Title, &post.Author, &post.AuthorID,
			&post.Content, &post.Tags, &post.MessageCount,
			&post.Timestamp, &post.CoverImageURL,
		); err != nil {
			log.Printf("Failed to scan post row: %v", err)
			continue
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// GetRandom 获取随机帖子
func (r *postRepository) GetRandom(guildID, tableName string, count int) ([]model.Post, error) {
	if err := r.validateTableName(tableName); err != nil {
		return nil, err
	}

	if count <= 0 {
		return []model.Post{}, nil
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		ORDER BY RANDOM() 
		LIMIT ?`, tableName)

	rows, err := db.Query(query, count)
	if err != nil {
		return nil, fmt.Errorf("failed to query random posts: %w", err)
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(
			&post.ID, &post.Title, &post.Author, &post.AuthorID,
			&post.Content, &post.Tags, &post.MessageCount,
			&post.Timestamp, &post.CoverImageURL,
		); err != nil {
			log.Printf("Failed to scan post row: %v", err)
			continue
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// Create 创建新帖子
func (r *postRepository) Create(guildID, tableName string, post *model.Post) error {
	if err := r.validateTableName(tableName); err != nil {
		return err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO "%s" (id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, tableName)

	_, err = db.Exec(query,
		post.ID, post.Title, post.Author, post.AuthorID,
		post.Content, post.Tags, post.MessageCount,
		post.Timestamp, post.CoverImageURL,
	)

	if err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	return nil
}

// Update 更新帖子
func (r *postRepository) Update(guildID, tableName string, post *model.Post) error {
	if err := r.validateTableName(tableName); err != nil {
		return err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`
		UPDATE "%s" 
		SET title = ?, author = ?, author_id = ?, content = ?, tags = ?, 
		    message_count = ?, timestamp = ?, cover_image_url = ?
		WHERE id = ?`, tableName)

	result, err := db.Exec(query,
		post.Title, post.Author, post.AuthorID, post.Content,
		post.Tags, post.MessageCount, post.Timestamp,
		post.CoverImageURL, post.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("post not found: %s", post.ID)
	}

	return nil
}

// Delete 删除帖子
func (r *postRepository) Delete(guildID, tableName, postID string) error {
	if err := r.validateTableName(tableName); err != nil {
		return err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`DELETE FROM "%s" WHERE id = ?`, tableName)

	result, err := db.Exec(query, postID)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("post not found: %s", postID)
	}

	return nil
}

// GetByAuthor 通过作者ID获取帖子
func (r *postRepository) GetByAuthor(guildID, tableName, authorID string) ([]model.Post, error) {
	if err := r.validateTableName(tableName); err != nil {
		return nil, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		WHERE author_id = ?
		ORDER BY timestamp DESC`, tableName)

	rows, err := db.Query(query, authorID)
	if err != nil {
		return nil, fmt.Errorf("failed to query posts by author: %w", err)
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(
			&post.ID, &post.Title, &post.Author, &post.AuthorID,
			&post.Content, &post.Tags, &post.MessageCount,
			&post.Timestamp, &post.CoverImageURL,
		); err != nil {
			log.Printf("Failed to scan post row: %v", err)
			continue
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// GetByTag 通过标签获取帖子
func (r *postRepository) GetByTag(guildID, tableName, tag string) ([]model.Post, error) {
	if err := r.validateTableName(tableName); err != nil {
		return nil, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		WHERE tags LIKE ?
		ORDER BY timestamp DESC`, tableName)

	rows, err := db.Query(query, "%"+tag+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to query posts by tag: %w", err)
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(
			&post.ID, &post.Title, &post.Author, &post.AuthorID,
			&post.Content, &post.Tags, &post.MessageCount,
			&post.Timestamp, &post.CoverImageURL,
		); err != nil {
			log.Printf("Failed to scan post row: %v", err)
			continue
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// GetByTimeRange 通过时间范围获取帖子
func (r *postRepository) GetByTimeRange(guildID, tableName string, startTime, endTime int64) ([]model.Post, error) {
	if err := r.validateTableName(tableName); err != nil {
		return nil, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		WHERE timestamp >= ? AND timestamp < ?
		ORDER BY timestamp DESC`, tableName)

	rows, err := db.Query(query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query posts by time range: %w", err)
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(
			&post.ID, &post.Title, &post.Author, &post.AuthorID,
			&post.Content, &post.Tags, &post.MessageCount,
			&post.Timestamp, &post.CoverImageURL,
		); err != nil {
			log.Printf("Failed to scan post row: %v", err)
			continue
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// Count 统计帖子数量
func (r *postRepository) Count(guildID, tableName string) (int, error) {
	if err := r.validateTableName(tableName); err != nil {
		return 0, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, tableName)

	var count int
	err = db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count posts: %w", err)
	}

	return count, nil
}

// CountByAuthor 统计作者的帖子数量
func (r *postRepository) CountByAuthor(guildID, tableName, authorID string) (int, error) {
	if err := r.validateTableName(tableName); err != nil {
		return 0, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE author_id = ?`, tableName)

	var count int
	err = db.QueryRow(query, authorID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count posts by author: %w", err)
	}

	return count, nil
}

// CountByTag 统计包含特定标签的帖子数量
func (r *postRepository) CountByTag(guildID, tableName, tag string) (int, error) {
	if err := r.validateTableName(tableName); err != nil {
		return 0, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE tags LIKE ?`, tableName)

	var count int
	err = db.QueryRow(query, "%"+tag+"%").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count posts by tag: %w", err)
	}

	return count, nil
}

// CountByTimeRange 统计时间范围内的帖子数量
func (r *postRepository) CountByTimeRange(guildID, tableName string, startTime, endTime int64) (int, error) {
	if err := r.validateTableName(tableName); err != nil {
		return 0, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE timestamp >= ? AND timestamp < ?`, tableName)

	var count int
	err = db.QueryRow(query, startTime, endTime).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count posts by time range: %w", err)
	}

	return count, nil
}

// CountInMultipleTables 统计多个表中的帖子数量
func (r *postRepository) CountInMultipleTables(guildID string, tableNames []string, startTime, endTime int64) (int, error) {
	if len(tableNames) == 0 {
		return 0, nil
	}

	// 验证所有表名
	for _, tableName := range tableNames {
		if err := r.validateTableName(tableName); err != nil {
			return 0, err
		}
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild database: %w", err)
	}

	// 构建安全的查询
	var queryBuilder strings.Builder
	var queryArgs []interface{}

	queryBuilder.WriteString("SELECT SUM(count) FROM (")
	for i, tableName := range tableNames {
		queryBuilder.WriteString(fmt.Sprintf(`SELECT COUNT(*) as count FROM "%s" WHERE timestamp >= ? AND timestamp < ?`, tableName))
		queryArgs = append(queryArgs, startTime, endTime)
		if i < len(tableNames)-1 {
			queryBuilder.WriteString(" UNION ALL ")
		}
	}
	queryBuilder.WriteString(")")

	var totalCount sql.NullInt64
	err = db.QueryRow(queryBuilder.String(), queryArgs...).Scan(&totalCount)
	if err != nil {
		return 0, fmt.Errorf("failed to count posts in multiple tables: %w", err)
	}

	if !totalCount.Valid {
		return 0, nil
	}

	return int(totalCount.Int64), nil
}

// CreateTable 创建帖子表
func (r *postRepository) CreateTable(guildID, tableName string) error {
	if err := r.validateTableName(tableName); err != nil {
		return err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS "%s" (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			author TEXT NOT NULL,
			author_id TEXT NOT NULL,
			content TEXT,
			tags TEXT,
			message_count INTEGER DEFAULT 0,
			timestamp INTEGER NOT NULL,
			cover_image_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`, tableName)

	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// 创建索引
	indexQueries := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_author_id ON "%s"(author_id)`, tableName, tableName),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_timestamp ON "%s"(timestamp)`, tableName, tableName),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_tags ON "%s"(tags)`, tableName, tableName),
	}

	for _, indexQuery := range indexQueries {
		if _, err := db.Exec(indexQuery); err != nil {
			log.Printf("Failed to create index: %v", err)
		}
	}

	return nil
}

// DropTable 删除帖子表
func (r *postRepository) DropTable(guildID, tableName string) error {
	if err := r.validateTableName(tableName); err != nil {
		return err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild database: %w", err)
	}

	query := fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName)

	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}

	return nil
}

// TableExists 检查表是否存在
func (r *postRepository) TableExists(guildID, tableName string) (bool, error) {
	if err := r.validateTableName(tableName); err != nil {
		return false, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return false, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?`

	var name string
	err = db.QueryRow(query, tableName).Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check table existence: %w", err)
	}

	return true, nil
}

// GetTableNames 获取所有表名
func (r *postRepository) GetTableNames(guildID string) ([]string, error) {
	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}

	query := `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get table names: %w", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			log.Printf("Failed to scan table name: %v", err)
			continue
		}
		tableNames = append(tableNames, tableName)
	}

	return tableNames, nil
}

// GetRandomFromMultipleTables 从多个表中获取随机帖子
func (r *postRepository) GetRandomFromMultipleTables(guildID string, tableNames []string, count int) ([]model.Post, error) {
	if len(tableNames) == 0 || count <= 0 {
		return []model.Post{}, nil
	}

	// 验证所有表名
	for _, tableName := range tableNames {
		if err := r.validateTableName(tableName); err != nil {
			return nil, err
		}
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}

	// 构建安全的 UNION 查询
	var queryBuilder strings.Builder
	queryBuilder.WriteString("SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM (")
	
	for i, tableName := range tableNames {
		if i > 0 {
			queryBuilder.WriteString(" UNION ALL ")
		}
		queryBuilder.WriteString(fmt.Sprintf(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "%s"`, tableName))
	}
	
	queryBuilder.WriteString(") ORDER BY RANDOM() LIMIT ?")

	rows, err := db.Query(queryBuilder.String(), count)
	if err != nil {
		return nil, fmt.Errorf("failed to query random posts from multiple tables: %w", err)
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(
			&post.ID, &post.Title, &post.Author, &post.AuthorID,
			&post.Content, &post.Tags, &post.MessageCount,
			&post.Timestamp, &post.CoverImageURL,
		); err != nil {
			log.Printf("Failed to scan post row: %v", err)
			continue
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// GetRandomFromMultipleTablesByTag 从多个表中按标签获取随机帖子
func (r *postRepository) GetRandomFromMultipleTablesByTag(guildID string, tableNames []string, tagID string, count int, excludeTags []string) ([]model.Post, error) {
	if len(tableNames) == 0 || count <= 0 {
		return []model.Post{}, nil
	}

	// 验证所有表名
	for _, tableName := range tableNames {
		if err := r.validateTableName(tableName); err != nil {
			return nil, err
		}
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}

	// 构建安全的 UNION 查询
	var queryBuilder strings.Builder
	var queryArgs []interface{}
	
	queryBuilder.WriteString("SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM (")
	
	for i, tableName := range tableNames {
		if i > 0 {
			queryBuilder.WriteString(" UNION ALL ")
		}
		queryBuilder.WriteString(fmt.Sprintf(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "%s"`, tableName))
		
		// 添加条件
		if tagID != "" || len(excludeTags) > 0 {
			queryBuilder.WriteString(" WHERE ")
			conditions := []string{}
			
			if tagID != "" {
				conditions = append(conditions, "tags LIKE ?")
				queryArgs = append(queryArgs, "%"+tagID+"%")
			}
			
			for _, excludeTag := range excludeTags {
				conditions = append(conditions, "tags NOT LIKE ?")
				queryArgs = append(queryArgs, "%"+excludeTag+"%")
			}
			
			queryBuilder.WriteString(strings.Join(conditions, " AND "))
		}
	}
	
	queryBuilder.WriteString(") ORDER BY RANDOM() LIMIT ?")
	queryArgs = append(queryArgs, count)

	rows, err := db.Query(queryBuilder.String(), queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query random posts from multiple tables by tag: %w", err)
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(
			&post.ID, &post.Title, &post.Author, &post.AuthorID,
			&post.Content, &post.Tags, &post.MessageCount,
			&post.Timestamp, &post.CoverImageURL,
		); err != nil {
			log.Printf("Failed to scan post row: %v", err)
			continue
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// GetRandomFromAllTables 从所有表中获取随机帖子
func (r *postRepository) GetRandomFromAllTables(guildID string, count int) ([]model.Post, error) {
	if count <= 0 {
		return []model.Post{}, nil
	}

	// 获取所有表名
	tableNames, err := r.GetTableNames(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get table names: %w", err)
	}

	return r.GetRandomFromMultipleTables(guildID, tableNames, count)
}

// GetRandomFromAllTablesByTag 从所有表中按标签获取随机帖子
func (r *postRepository) GetRandomFromAllTablesByTag(guildID string, tagID string, count int, excludeTags []string) ([]model.Post, error) {
	if count <= 0 {
		return []model.Post{}, nil
	}

	// 获取所有表名
	tableNames, err := r.GetTableNames(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get table names: %w", err)
	}

	return r.GetRandomFromMultipleTablesByTag(guildID, tableNames, tagID, count, excludeTags)
}
