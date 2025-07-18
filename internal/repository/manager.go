package repository

import (
	"database/sql"
	"discord-bot/internal/database"
	"fmt"
)

// repositoryManager 仓库管理器实现
type repositoryManager struct {
	dbService *database.Service

	// 仓库实例缓存
	postRepo        PostRepository
	userRepo        UserRepository
	punishmentRepo  PunishmentRepository
	guildRepo       GuildRepository
	timedTaskRepo   TimedTaskRepository
	leaderboardRepo LeaderboardRepository
}

// NewRepositoryManager 创建新的仓库管理器
func NewRepositoryManager(dbService *database.Service) RepositoryManager {
	return &repositoryManager{
		dbService: dbService,
	}
}

// PostRepository 获取帖子仓库实例
func (m *repositoryManager) PostRepository() PostRepository {
	if m.postRepo == nil {
		m.postRepo = NewPostRepository(m.dbService)
	}
	return m.postRepo
}

// UserRepository 获取用户仓库实例
func (m *repositoryManager) UserRepository() UserRepository {
	if m.userRepo == nil {
		m.userRepo = NewUserRepository(m.dbService)
	}
	return m.userRepo
}

// PunishmentRepository 获取惩罚仓库实例
func (m *repositoryManager) PunishmentRepository() PunishmentRepository {
	if m.punishmentRepo == nil {
		m.punishmentRepo = NewPunishmentRepository(m.dbService)
	}
	return m.punishmentRepo
}

// GuildRepository 获取服务器仓库实例
func (m *repositoryManager) GuildRepository() GuildRepository {
	if m.guildRepo == nil {
		m.guildRepo = NewGuildRepository(m.dbService)
	}
	return m.guildRepo
}

// TimedTaskRepository 获取定时任务仓库实例
func (m *repositoryManager) TimedTaskRepository() TimedTaskRepository {
	if m.timedTaskRepo == nil {
		m.timedTaskRepo = NewTimedTaskRepository(m.dbService)
	}
	return m.timedTaskRepo
}

// LeaderboardRepository 获取排行榜仓库实例
func (m *repositoryManager) LeaderboardRepository() LeaderboardRepository {
	if m.leaderboardRepo == nil {
		m.leaderboardRepo = NewLeaderboardRepository(m.dbService)
	}
	return m.leaderboardRepo
}

// BeginTransaction 开始事务
func (m *repositoryManager) BeginTransaction() (Transaction, error) {
	return NewTransaction(m.dbService)
}

// Close 关闭仓库管理器
func (m *repositoryManager) Close() error {
	return m.dbService.Close()
}

// Ping 测试数据库连接
func (m *repositoryManager) Ping() error {
	return m.dbService.PingAll()
}

// GetStats 获取连接池统计信息
func (m *repositoryManager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// 获取连接池统计
	dbStats := m.dbService.GetStats()
	stats["database"] = dbStats

	// 可以添加其他统计信息
	stats["repository_count"] = m.getRepositoryCount()

	return stats
}

// getRepositoryCount 获取已初始化的仓库数量
func (m *repositoryManager) getRepositoryCount() int {
	count := 0
	if m.postRepo != nil {
		count++
	}
	if m.userRepo != nil {
		count++
	}
	if m.punishmentRepo != nil {
		count++
	}
	if m.guildRepo != nil {
		count++
	}
	if m.timedTaskRepo != nil {
		count++
	}
	if m.leaderboardRepo != nil {
		count++
	}
	return count
}

// transaction 事务实现
type transaction struct {
	tx        *sql.Tx
	dbService *database.Service

	// 事务中的仓库实例
	postRepo        PostRepository
	userRepo        UserRepository
	punishmentRepo  PunishmentRepository
	guildRepo       GuildRepository
	timedTaskRepo   TimedTaskRepository
	leaderboardRepo LeaderboardRepository
}

// NewTransaction 创建新的事务
func NewTransaction(dbService *database.Service) (Transaction, error) {
	// 这里简化实现，实际应该选择合适的数据库连接
	// 由于SQLite的限制，我们选择主要的guilds数据库进行事务
	db, err := dbService.GetPool().GetGuildsDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database for transaction: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &transaction{
		tx:        tx,
		dbService: dbService,
	}, nil
}

// Commit 提交事务
func (t *transaction) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback 回滚事务
func (t *transaction) Rollback() error {
	if err := t.tx.Rollback(); err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}

// PostRepository 获取事务中的帖子仓库实例
func (t *transaction) PostRepository() PostRepository {
	if t.postRepo == nil {
		t.postRepo = NewTransactionalPostRepository(t.tx, t.dbService)
	}
	return t.postRepo
}

// UserRepository 获取事务中的用户仓库实例
func (t *transaction) UserRepository() UserRepository {
	if t.userRepo == nil {
		t.userRepo = NewTransactionalUserRepository(t.tx, t.dbService)
	}
	return t.userRepo
}

// PunishmentRepository 获取事务中的惩罚仓库实例
func (t *transaction) PunishmentRepository() PunishmentRepository {
	if t.punishmentRepo == nil {
		t.punishmentRepo = NewTransactionalPunishmentRepository(t.tx, t.dbService)
	}
	return t.punishmentRepo
}

// GuildRepository 获取事务中的服务器仓库实例
func (t *transaction) GuildRepository() GuildRepository {
	if t.guildRepo == nil {
		t.guildRepo = NewTransactionalGuildRepository(t.tx, t.dbService)
	}
	return t.guildRepo
}

// TimedTaskRepository 获取事务中的定时任务仓库实例
func (t *transaction) TimedTaskRepository() TimedTaskRepository {
	if t.timedTaskRepo == nil {
		t.timedTaskRepo = NewTransactionalTimedTaskRepository(t.tx, t.dbService)
	}
	return t.timedTaskRepo
}

// LeaderboardRepository 获取事务中的排行榜仓库实例
func (t *transaction) LeaderboardRepository() LeaderboardRepository {
	if t.leaderboardRepo == nil {
		t.leaderboardRepo = NewTransactionalLeaderboardRepository(t.tx, t.dbService)
	}
	return t.leaderboardRepo
}

// 仓库构造函数 - 当前只实现了PostRepository，其他返回nil
func NewUserRepository(dbService *database.Service) UserRepository {
	return nil // 暂未实现
}

func NewPunishmentRepository(dbService *database.Service) PunishmentRepository {
	return nil // 暂未实现
}

func NewGuildRepository(dbService *database.Service) GuildRepository {
	return nil // 暂未实现
}

func NewTimedTaskRepository(dbService *database.Service) TimedTaskRepository {
	return nil // 暂未实现
}

func NewLeaderboardRepository(dbService *database.Service) LeaderboardRepository {
	return nil // 暂未实现
}

// 事务版本的仓库构造函数 - 暂未实现
func NewTransactionalPostRepository(tx *sql.Tx, dbService *database.Service) PostRepository {
	return nil // 暂未实现
}

func NewTransactionalUserRepository(tx *sql.Tx, dbService *database.Service) UserRepository {
	return nil // 暂未实现
}

func NewTransactionalPunishmentRepository(tx *sql.Tx, dbService *database.Service) PunishmentRepository {
	return nil // 暂未实现
}

func NewTransactionalGuildRepository(tx *sql.Tx, dbService *database.Service) GuildRepository {
	return nil // 暂未实现
}

func NewTransactionalTimedTaskRepository(tx *sql.Tx, dbService *database.Service) TimedTaskRepository {
	return nil // 暂未实现
}

func NewTransactionalLeaderboardRepository(tx *sql.Tx, dbService *database.Service) LeaderboardRepository {
	return nil // 暂未实现
}
