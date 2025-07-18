package repository

import (
	"database/sql"
	"discord-bot/internal/database"
	"discord-bot/model"
	"fmt"
	"log"
	"strings"

	"github.com/jmoiron/sqlx"
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

// validateTableName 验证表名安全性（基础）
func (r *postRepository) validateTableName(tableName string) error {
	if len(tableName) == 0 || len(tableName) > 64 {
		return fmt.Errorf("table name length must be between 1 and 64 characters")
	}
	for _, char := range tableName {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_') {
			return fmt.Errorf("invalid character in table name: %s. Only letters, numbers, and underscores are allowed", tableName)
		}
	}
	return nil
}

// validateTableNameWithWhitelist 使用白名单验证表名，防止SQL注入
func (r *postRepository) validateTableNameWithWhitelist(guildID, tableName string) error {
	if err := r.validateTableName(tableName); err != nil {
		return err
	}

	// 白名单验证
	validTableNames, err := r.GetTableNames(guildID)
	if err != nil {
		return fmt.Errorf("could not get table names for validation: %w", err)
	}

	for _, validName := range validTableNames {
		if tableName == validName {
			return nil // 表名在白名单中
		}
	}

	return fmt.Errorf("table name '%s' is not in the whitelist of allowed table names", tableName)
}

// GetByID 通过ID获取帖子
func (r *postRepository) GetByID(guildID, tableName, postID string) (*model.Post, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return nil, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		WHERE id = ?`, tableName)

	var post model.Post
	err = sqlxDB.Get(&post, query, postID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("failed to get post by id with sqlx: %w", err)
	}

	return &post, nil
}

// GetAll 获取所有帖子
func (r *postRepository) GetAll(guildID, tableName string) ([]model.Post, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return nil, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		ORDER BY timestamp DESC`, tableName)

	var posts []model.Post
	err = sqlxDB.Select(&posts, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all posts with sqlx: %w", err)
	}

	return posts, nil
}

// GetRandom 获取随机帖子
func (r *postRepository) GetRandom(guildID, tableName string, count int) ([]model.Post, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return nil, err
	}
	if count <= 0 {
		return []model.Post{}, nil
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		ORDER BY RANDOM() 
		LIMIT ?`, tableName)

	var posts []model.Post
	err = sqlxDB.Select(&posts, query, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get random posts with sqlx: %w", err)
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
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`
		INSERT INTO "%s" (id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url)
		VALUES (:id, :title, :author, :author_id, :content, :tags, :message_count, :timestamp, :cover_image_url)`, tableName)

	_, err = sqlxDB.NamedExec(query, post)
	if err != nil {
		return fmt.Errorf("failed to create post with sqlx: %w", err)
	}

	return nil
}

// Update 更新帖子
func (r *postRepository) Update(guildID, tableName string, post *model.Post) error {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`
		UPDATE "%s" 
		SET title = :title, author = :author, author_id = :author_id, content = :content, 
		    tags = :tags, message_count = :message_count, timestamp = :timestamp, 
		    cover_image_url = :cover_image_url
		WHERE id = :id`, tableName)

	result, err := sqlxDB.NamedExec(query, post)
	if err != nil {
		return fmt.Errorf("failed to update post with sqlx: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("post not found for update: %s", post.ID)
	}

	return nil
}

// Delete 删除帖子
func (r *postRepository) Delete(guildID, tableName, postID string) error {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`DELETE FROM "%s" WHERE id = ?`, tableName)

	result, err := sqlxDB.Exec(query, postID)
	if err != nil {
		return fmt.Errorf("failed to delete post with sqlx: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("post not found for delete: %s", postID)
	}

	return nil
}

// GetByAuthor 通过作者ID获取帖子
func (r *postRepository) GetByAuthor(guildID, tableName, authorID string) ([]model.Post, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return nil, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		WHERE author_id = ? 
		ORDER BY timestamp DESC`, tableName)

	var posts []model.Post
	err = sqlxDB.Select(&posts, query, authorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts by author with sqlx: %w", err)
	}

	return posts, nil
}

// GetByTag 通过标签获取帖子
func (r *postRepository) GetByTag(guildID, tableName, tag string) ([]model.Post, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return nil, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		WHERE tags LIKE ? 
		ORDER BY timestamp DESC`, tableName)

	var posts []model.Post
	err = sqlxDB.Select(&posts, query, "%"+tag+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to get posts by tag with sqlx: %w", err)
	}

	return posts, nil
}

// GetByTimeRange 通过时间范围获取帖子
func (r *postRepository) GetByTimeRange(guildID, tableName string, startTime, endTime int64) ([]model.Post, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return nil, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`
		SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url 
		FROM "%s" 
		WHERE timestamp >= ? AND timestamp < ? 
		ORDER BY timestamp DESC`, tableName)

	var posts []model.Post
	err = sqlxDB.Select(&posts, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts by time range with sqlx: %w", err)
	}

	return posts, nil
}

// Count 统计帖子数量
func (r *postRepository) Count(guildID, tableName string) (int, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return 0, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, tableName)

	var count int
	err = sqlxDB.Get(&count, query)
	if err != nil {
		return 0, fmt.Errorf("failed to count posts with sqlx: %w", err)
	}

	return count, nil
}

// CountByAuthor 统计作者的帖子数量
func (r *postRepository) CountByAuthor(guildID, tableName, authorID string) (int, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return 0, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE author_id = ?`, tableName)

	var count int
	err = sqlxDB.Get(&count, query, authorID)
	if err != nil {
		return 0, fmt.Errorf("failed to count posts by author with sqlx: %w", err)
	}

	return count, nil
}

// CountByTag 统计包含特定标签的帖子数量
func (r *postRepository) CountByTag(guildID, tableName, tag string) (int, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return 0, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE tags LIKE ?`, tableName)

	var count int
	err = sqlxDB.Get(&count, query, "%"+tag+"%")
	if err != nil {
		return 0, fmt.Errorf("failed to count posts by tag with sqlx: %w", err)
	}

	return count, nil
}

// CountByTimeRange 统计时间范围内的帖子数量
func (r *postRepository) CountByTimeRange(guildID, tableName string, startTime, endTime int64) (int, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return 0, err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE timestamp >= ? AND timestamp < ?`, tableName)

	var count int
	err = sqlxDB.Get(&count, query, startTime, endTime)
	if err != nil {
		return 0, fmt.Errorf("failed to count posts by time range with sqlx: %w", err)
	}

	return count, nil
}

// CountInMultipleTables 统计多个表中的帖子数量
func (r *postRepository) CountInMultipleTables(guildID string, tableNames []string, startTime, endTime int64) (int, error) {
	if len(tableNames) == 0 {
		return 0, nil
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var totalCount int
	for _, tableName := range tableNames {
		if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
			return 0, err
		}
		query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE timestamp >= ? AND timestamp < ?`, tableName)
		var count int
		err := sqlxDB.Get(&count, query, startTime, endTime)
		if err != nil {
			log.Printf("failed to count posts in table %s: %v. Skipping.", tableName, err)
			continue
		}
		totalCount += count
	}

	return totalCount, nil
}

// --- Missing method implementations ---

// GetTableNames 获取所有表名
func (r *postRepository) GetTableNames(guildID string) ([]string, error) {
	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	rows, err := sqlxDB.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';")
	if err != nil {
		return nil, fmt.Errorf("failed to query table names: %w", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tableNames = append(tableNames, name)
	}
	return tableNames, nil
}

// GetRandomFromAllTables 从所有表中随机获取帖子
func (r *postRepository) GetRandomFromAllTables(guildID string, count int) ([]model.Post, error) {
	tableNames, err := r.GetTableNames(guildID)
	if err != nil {
		return nil, err
	}
	return r.GetRandomFromMultipleTables(guildID, tableNames, count)
}

// GetRandomFromAllTablesByTag 从所有表中按标签随机获取帖子
func (r *postRepository) GetRandomFromAllTablesByTag(guildID string, tagID string, count int, excludeTags []string) ([]model.Post, error) {
	tableNames, err := r.GetTableNames(guildID)
	if err != nil {
		return nil, err
	}
	return r.GetRandomFromMultipleTablesByTag(guildID, tableNames, tagID, count, excludeTags)
}

// GetRandomFromMultipleTables 从多个表中随机获取帖子
func (r *postRepository) GetRandomFromMultipleTables(guildID string, tableNames []string, count int) ([]model.Post, error) {
	if len(tableNames) == 0 || count <= 0 {
		return []model.Post{}, nil
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var queries []string
	for _, tableName := range tableNames {
		if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
			return nil, err
		}
		queries = append(queries, fmt.Sprintf(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "%s"`, tableName))
	}
	fullQuery := strings.Join(queries, " UNION ALL ")
	finalQuery := fmt.Sprintf("SELECT * FROM (%s) ORDER BY RANDOM() LIMIT ?", fullQuery)

	var posts []model.Post
	err = sqlxDB.Select(&posts, finalQuery, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get random posts from multiple tables with sqlx: %w", err)
	}
	return posts, nil
}

// GetRandomFromMultipleTablesByTag 从多个表中按标签随机获取帖子
func (r *postRepository) GetRandomFromMultipleTablesByTag(guildID string, tableNames []string, tagID string, count int, excludeTags []string) ([]model.Post, error) {
	if len(tableNames) == 0 || count <= 0 {
		return []model.Post{}, nil
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var queries []string
	var args []interface{}
	for _, tableName := range tableNames {
		if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
			return nil, err
		}
		baseQuery := fmt.Sprintf(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "%s" WHERE tags LIKE ?`, tableName)
		args = append(args, "%"+tagID+"%")

		if len(excludeTags) > 0 {
			for _, exclude := range excludeTags {
				baseQuery += " AND tags NOT LIKE ?"
				args = append(args, "%"+exclude+"%")
			}
		}
		queries = append(queries, baseQuery)
	}

	fullQuery := strings.Join(queries, " UNION ALL ")
	finalQuery := fmt.Sprintf("SELECT * FROM (%s) ORDER BY RANDOM() LIMIT ?", fullQuery)
	args = append(args, count)

	var posts []model.Post
	err = sqlxDB.Select(&posts, finalQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get random posts by tag from multiple tables with sqlx: %w", err)
	}
	return posts, nil
}

// TableExists 检查表是否存在
func (r *postRepository) TableExists(guildID, tableName string) (bool, error) {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return false, err
	}
	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return false, fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	var name string
	err = sqlxDB.Get(&name, query, tableName)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if table exists: %w", err)
	}
	return true, nil
}

// CreateTable 创建一个新的帖子表
func (r *postRepository) CreateTable(guildID, tableName string) error {
	if err := r.validateTableName(tableName); err != nil {
		return err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS "%s" (
			id TEXT PRIMARY KEY,
			title TEXT,
			author TEXT,
			author_id TEXT,
			content TEXT,
			tags TEXT,
			message_count INTEGER,
			timestamp INTEGER,
			cover_image_url TEXT
		);`, tableName)

	_, err = sqlxDB.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}
	return nil
}

// DropTable 删除一个帖子表
func (r *postRepository) DropTable(guildID, tableName string) error {
	if err := r.validateTableNameWithWhitelist(guildID, tableName); err != nil {
		return err
	}

	db, err := r.dbService.GetPool().GetGuildDB(guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName)

	_, err = sqlxDB.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to drop table %s: %w", tableName, err)
	}
	return nil
}

// transactionalPostRepository 事务版本的帖子数据访问实现
type transactionalPostRepository struct {
	*postRepository
	tx *sqlx.Tx
}
