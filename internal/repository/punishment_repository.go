package repository

import (
	"database/sql"
	"discord-bot/internal/database"
	"discord-bot/model"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// punishmentRepository 惩罚数据访问实现
type punishmentRepository struct {
	dbService *database.Service
}

// NewPunishmentRepository 创建新的惩罚仓库实例
func NewPunishmentRepository(dbService *database.Service) PunishmentRepository {
	return &punishmentRepository{
		dbService: dbService,
	}
}

// GetByID 通过ID获取惩罚记录
func (r *punishmentRepository) GetByID(punishmentID int64) (*model.PunishmentRecord, error) {
	pool := r.dbService.GetPool()
	db, err := pool.GetKickUserDB()
	if err != nil {
		return nil, err
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var record model.PunishmentRecord
	query := "SELECT * FROM punishments WHERE punishment_id = ?"
	err = sqlxDB.Get(&record, query, punishmentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an application error
		}
		return nil, fmt.Errorf("failed to get punishment record by ID %d: %w", punishmentID, err)
	}
	return &record, nil
}

// GetByUserID 通过用户ID获取所有惩罚记录
func (r *punishmentRepository) GetByUserID(userID string) ([]model.PunishmentRecord, error) {
	return r.getPunishmentsByField("user_id", userID, nil)
}

// GetByUserIDSince 通过用户ID获取指定时间点之后的所有惩罚记录
func (r *punishmentRepository) GetByUserIDSince(userID string, since time.Time) ([]model.PunishmentRecord, error) {
	return r.getPunishmentsByField("user_id", userID, &since)
}

// GetByAdminID 通过管理员ID获取所有惩罚记录
func (r *punishmentRepository) GetByAdminID(adminID string) ([]model.PunishmentRecord, error) {
	return r.getPunishmentsByField("admin_id", adminID, nil)
}

// GetByGuildID 通过服务器ID获取所有惩罚记录
func (r *punishmentRepository) GetByGuildID(guildID string) ([]model.PunishmentRecord, error) {
	return r.getPunishmentsByField("guild_id", guildID, nil)
}

// GetLatestByUserID 获取用户在特定服务器的最新一笔惩罚记录
func (r *punishmentRepository) GetLatestByUserID(guildID, userID string) (*model.PunishmentRecord, error) {
	pool := r.dbService.GetPool()
	db, err := pool.GetKickUserDB()
	if err != nil {
		return nil, err
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var record model.PunishmentRecord
	query := "SELECT * FROM punishments WHERE guild_id = ? AND user_id = ? ORDER BY timestamp DESC LIMIT 1"
	err = sqlxDB.Get(&record, query, guildID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest punishment for user %s in guild %s: %w", userID, guildID, err)
	}
	return &record, nil
}

// Create 创建一个新的惩罚记录，并返回记录的ID
func (r *punishmentRepository) Create(punishment *model.PunishmentRecord) (int64, error) {
	pool := r.dbService.GetPool()
	db, err := pool.GetKickUserDB()
	if err != nil {
		return 0, err
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := `INSERT INTO punishments (message_id, admin_id, user_id, user_username, reason, guild_id, timestamp, evidence)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := sqlxDB.Exec(query,
		punishment.MessageID,
		punishment.AdminID,
		punishment.UserID,
		punishment.UserUsername,
		punishment.Reason,
		punishment.GuildID,
		punishment.Timestamp,
		punishment.Evidence,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create punishment record: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID for punishment record: %w", err)
	}

	return id, nil
}

// Update 更新惩罚记录
func (r *punishmentRepository) Update(punishment *model.PunishmentRecord) error {
	pool := r.dbService.GetPool()
	db, err := pool.GetKickUserDB()
	if err != nil {
		return err
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := `UPDATE punishments SET
				message_id = :message_id,
				admin_id = :admin_id,
				user_id = :user_id,
				user_username = :user_username,
				reason = :reason,
				guild_id = :guild_id,
				timestamp = :timestamp,
				evidence = :evidence
			  WHERE punishment_id = :punishment_id`

	_, err = sqlxDB.NamedExec(query, punishment)
	if err != nil {
		return fmt.Errorf("failed to update punishment record %d: %w", punishment.PunishmentID, err)
	}
	return nil
}

// Delete 删除惩罚记录
func (r *punishmentRepository) Delete(punishmentID int64) error {
	pool := r.dbService.GetPool()
	db, err := pool.GetKickUserDB()
	if err != nil {
		return err
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	query := "DELETE FROM punishments WHERE punishment_id = ?"
	_, err = sqlxDB.Exec(query, punishmentID)
	if err != nil {
		return fmt.Errorf("failed to delete punishment record %d: %w", punishmentID, err)
	}
	return nil
}

// CountByUser 统计用户的惩罚记录数量
func (r *punishmentRepository) CountByUser(userID string) (int, error) {
	return r.countByField("user_id", userID)
}

// CountByGuild 统计服务器的惩罚记录数量
func (r *punishmentRepository) CountByGuild(guildID string) (int, error) {
	return r.countByField("guild_id", guildID)
}

// GetRecentPunishments 获取最近的惩罚记录
func (r *punishmentRepository) GetRecentPunishments(guildID string, limit int) ([]model.PunishmentRecord, error) {
	pool := r.dbService.GetPool()
	db, err := pool.GetKickUserDB()
	if err != nil {
		return nil, err
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var records []model.PunishmentRecord
	query := "SELECT * FROM punishments WHERE guild_id = ? ORDER BY timestamp DESC LIMIT ?"
	err = sqlxDB.Select(&records, query, guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent punishments for guild %s: %w", guildID, err)
	}
	return records, nil
}

// getPunishmentsByField 是一个通用的辅助函数，用于根据不同的字段获取惩罚记录
func (r *punishmentRepository) getPunishmentsByField(field string, value interface{}, since *time.Time) ([]model.PunishmentRecord, error) {
	pool := r.dbService.GetPool()
	db, err := pool.GetKickUserDB()
	if err != nil {
		return nil, err
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var records []model.PunishmentRecord
	args := []interface{}{value}
	query := fmt.Sprintf("SELECT * FROM punishments WHERE %s = ?", field)

	if since != nil {
		query += " AND timestamp >= ?"
		args = append(args, since.Unix())
	}

	query += " ORDER BY timestamp DESC"

	err = sqlxDB.Select(&records, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get punishments by %s = %v: %w", field, value, err)
	}
	return records, nil
}

// countByField 是一个通用的辅助函数，用于根据不同的字段统计记录数量
func (r *punishmentRepository) countByField(field string, value interface{}) (int, error) {
	pool := r.dbService.GetPool()
	db, err := pool.GetKickUserDB()
	if err != nil {
		return 0, err
	}
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM punishments WHERE %s = ?", field)
	err = sqlxDB.Get(&count, query, value)
	if err != nil {
		return 0, fmt.Errorf("failed to count punishments by %s = %v: %w", field, value, err)
	}
	return count, nil
}

// PunishmentRepositoryImpl 公开的类型，用于在需要时进行类型断言，但应尽量避免使用
type PunishmentRepositoryImpl = punishmentRepository

// transactionalPunishmentRepository 事务版本的惩罚数据访问实现
type transactionalPunishmentRepository struct {
	*punishmentRepository
	tx *sqlx.Tx
}
