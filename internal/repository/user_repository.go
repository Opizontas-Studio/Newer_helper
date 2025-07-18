package repository

import (
	"database/sql"
	"discord-bot/internal/database"
	"discord-bot/model"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// userRepository 用户数据访问实现
type userRepository struct {
	dbService *database.Service
}

// NewUserRepository 创建新的用户仓库实例
func NewUserRepository(dbService *database.Service) UserRepository {
	return &userRepository{
		dbService: dbService,
	}
}

// GetByID 获取用户信息
func (r *userRepository) GetByID(userID string) (*model.User, error) {
	db, err := r.dbService.GetPool().GetUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get user database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var user model.User
	query := "SELECT * FROM users WHERE id = ?"
	err = sqlxDB.Get(&user, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return &user, nil
}

// Create 创建新用户
func (r *userRepository) Create(user *model.User) error {
	db, err := r.dbService.GetPool().GetUserDB()
	if err != nil {
		return fmt.Errorf("failed to get user database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := `INSERT INTO users (id, username, discriminator, avatar_url, created_at)
	          VALUES (:id, :username, :discriminator, :avatar_url, :created_at)`
	_, err = sqlxDB.NamedExec(query, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// Update 更新用户信息
func (r *userRepository) Update(user *model.User) error {
	db, err := r.dbService.GetPool().GetUserDB()
	if err != nil {
		return fmt.Errorf("failed to get user database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := `UPDATE users SET
				username = :username,
				discriminator = :discriminator,
				avatar_url = :avatar_url
			  WHERE id = :id`
	result, err := sqlxDB.NamedExec(query, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("user not found for update")
	}
	return nil
}

// Delete 删除用户
func (r *userRepository) Delete(userID string) error {
	db, err := r.dbService.GetPool().GetUserDB()
	if err != nil {
		return fmt.Errorf("failed to get user database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := "DELETE FROM users WHERE id = ?"
	result, err := sqlxDB.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("user not found for delete")
	}
	return nil
}

// GetUserStats 获取用户统计信息
func (r *userRepository) GetUserStats(userID, guildID string) (*model.UserStats, error) {
	db, err := r.dbService.GetPool().GetUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get user database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var stats model.UserStats
	query := "SELECT * FROM user_stats WHERE user_id = ? AND guild_id = ?"
	err = sqlxDB.Get(&stats, query, userID, guildID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &model.UserStats{UserID: userID, GuildID: guildID}, nil // Return empty stats if not found
		}
		return nil, fmt.Errorf("failed to get user stats: %w", err)
	}
	return &stats, nil
}

// UpdateUserStats 更新用户统计信息
func (r *userRepository) UpdateUserStats(userID, guildID string, stats *model.UserStats) error {
	db, err := r.dbService.GetPool().GetUserDB()
	if err != nil {
		return fmt.Errorf("failed to get user database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := `
		INSERT INTO user_stats (user_id, guild_id, roll_count, post_count, last_roll_time, last_post_time)
		VALUES (:user_id, :guild_id, :roll_count, :post_count, :last_roll_time, :last_post_time)
		ON CONFLICT(user_id, guild_id) DO UPDATE SET
			roll_count = excluded.roll_count,
			post_count = excluded.post_count,
			last_roll_time = excluded.last_roll_time,
			last_post_time = excluded.last_post_time;
	`
	_, err = sqlxDB.NamedExec(query, stats)
	if err != nil {
		return fmt.Errorf("failed to update user stats: %w", err)
	}
	return nil
}

// GetUserPreferences 获取用户偏好
func (r *userRepository) GetUserPreferences(userID, guildID string) ([]string, error) {
	db, err := r.dbService.GetPool().GetUserDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get user database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := `SELECT preferred_pools FROM user_preferences WHERE user_id = ? AND guild_id = ?`
	var preferredPoolsJSON string
	err = sqlxDB.Get(&preferredPoolsJSON, query, userID, guildID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []string{}, nil // No preferences set is not an error
		}
		return nil, fmt.Errorf("failed to get user preferences: %w", err)
	}

	if preferredPoolsJSON == "" {
		return []string{}, nil
	}

	var preferredPools []string
	if err := json.Unmarshal([]byte(preferredPoolsJSON), &preferredPools); err != nil {
		return nil, fmt.Errorf("failed to unmarshal preferred pools: %w", err)
	}

	return preferredPools, nil
}

// SetUserPreferences 设置用户偏好
func (r *userRepository) SetUserPreferences(userID, guildID string, preferences []string) error {
	db, err := r.dbService.GetPool().GetUserDB()
	if err != nil {
		return fmt.Errorf("failed to get user database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	preferencesJSON, err := json.Marshal(preferences)
	if err != nil {
		return fmt.Errorf("failed to marshal preferred pools: %w", err)
	}

	query := `
		INSERT INTO user_preferences (user_id, guild_id, preferred_pools)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, guild_id) DO UPDATE SET
		preferred_pools = excluded.preferred_pools;
	`
	_, err = sqlxDB.Exec(query, userID, guildID, string(preferencesJSON))
	if err != nil {
		return fmt.Errorf("failed to set user preferences: %w", err)
	}

	return nil
}

// GetTotalUserCount 获取总用户数
func (r *userRepository) GetTotalUserCount() (int, error) {
	db, err := r.dbService.GetPool().GetUserDB()
	if err != nil {
		return 0, fmt.Errorf("failed to get user database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var count int
	err = sqlxDB.Get(&count, "SELECT COUNT(*) FROM users")
	if err != nil {
		return 0, fmt.Errorf("failed to get total user count: %w", err)
	}
	return count, nil
}

// GetActiveUserCount 获取活跃用户数
func (r *userRepository) GetActiveUserCount(guildID string, since time.Time) (int, error) {
	db, err := r.dbService.GetPool().GetUserDB()
	if err != nil {
		return 0, fmt.Errorf("failed to get user database: %w", err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var count int
	query := `
		SELECT COUNT(DISTINCT user_id)
		FROM user_stats
		WHERE guild_id = ? AND (last_roll_time >= ? OR last_post_time >= ?)`
	err = sqlxDB.Get(&count, query, guildID, since, since)
	if err != nil {
		return 0, fmt.Errorf("failed to get active user count: %w", err)
	}
	return count, nil
}

// transactionalUserRepository 事务版本的用户数据访问实现
type transactionalUserRepository struct {
	*userRepository
	tx *sqlx.Tx
}

// NewTransactionalUserRepository 创建新的事务性用户仓库实例
func NewTransactionalUserRepository(tx *sqlx.Tx, dbService *database.Service) UserRepository {
	return &transactionalUserRepository{
		userRepository: &userRepository{
			dbService: dbService,
		},
		tx: tx,
	}
}

// GetUserPreferences 获取用户偏好 (事务)
func (r *transactionalUserRepository) GetUserPreferences(userID, guildID string) ([]string, error) {
	query := `SELECT preferred_pools FROM user_preferences WHERE user_id = ? AND guild_id = ?`
	var preferredPoolsJSON string
	err := r.tx.Get(&preferredPoolsJSON, query, userID, guildID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []string{}, nil // No preferences set is not an error
		}
		return nil, fmt.Errorf("failed to get user preferences in transaction: %w", err)
	}

	if preferredPoolsJSON == "" {
		return []string{}, nil
	}

	var preferredPools []string
	if err := json.Unmarshal([]byte(preferredPoolsJSON), &preferredPools); err != nil {
		return nil, fmt.Errorf("failed to unmarshal preferred pools in transaction: %w", err)
	}

	return preferredPools, nil
}

// SetUserPreferences 设置用户偏好 (事务)
func (r *transactionalUserRepository) SetUserPreferences(userID, guildID string, preferences []string) error {
	preferencesJSON, err := json.Marshal(preferences)
	if err != nil {
		return fmt.Errorf("failed to marshal preferred pools in transaction: %w", err)
	}

	query := `
		INSERT INTO user_preferences (user_id, guild_id, preferred_pools)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, guild_id) DO UPDATE SET
		preferred_pools = excluded.preferred_pools;
	`
	_, err = r.tx.Exec(query, userID, guildID, string(preferencesJSON))
	if err != nil {
		return fmt.Errorf("failed to set user preferences in transaction: %w", err)
	}

	return nil
}
