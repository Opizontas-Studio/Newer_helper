package database

import (
	"database/sql"
	"fmt"
	"log"
)

// Service 数据库服务
type Service struct {
	pool *Pool
}

// NewService 创建新的数据库服务
func NewService(config PoolConfig) *Service {
	return &Service{
		pool: NewPool(config),
	}
}

// NewServiceWithDefault 使用默认配置创建数据库服务
func NewServiceWithDefault() *Service {
	return &Service{
		pool: NewPoolWithDefault(),
	}
}

// GetPool 获取连接池
func (s *Service) GetPool() *Pool {
	return s.pool
}

// InitializeDatabase 初始化数据库，创建必要的表
func (s *Service) InitializeDatabase() error {
	// 初始化服务器配置数据库
	guildsDB, err := s.pool.GetGuildsDB()
	if err != nil {
		return fmt.Errorf("failed to get guilds database: %w", err)
	}

	if err := s.createGuildTables(guildsDB); err != nil {
		return fmt.Errorf("failed to create guild tables: %w", err)
	}

	// 初始化用户数据库
	userDB, err := s.pool.GetUserDB()
	if err != nil {
		return fmt.Errorf("failed to get user database: %w", err)
	}

	if err := s.createUserTables(userDB); err != nil {
		return fmt.Errorf("failed to create user tables: %w", err)
	}

	// 初始化定时任务数据库
	timedTasksDB, err := s.pool.GetTimedTasksDB()
	if err != nil {
		return fmt.Errorf("failed to get timed tasks database: %w", err)
	}

	if err := s.createTimedTaskTables(timedTasksDB); err != nil {
		return fmt.Errorf("failed to create timed task tables: %w", err)
	}

	log.Println("Database initialization completed successfully")
	return nil
}

// createGuildTables 创建服务器相关表
func (s *Service) createGuildTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS servers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			admin_role_ids TEXT,
			user_role_ids TEXT,
			preset_messages TEXT,
			top_channels TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS preset_messages (
			id TEXT PRIMARY KEY,
			guild_id TEXT NOT NULL,
			name TEXT NOT NULL,
			value TEXT NOT NULL,
			description TEXT,
			type TEXT DEFAULT 'text',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (guild_id) REFERENCES servers(id)
		)`,
		`CREATE TABLE IF NOT EXISTS top_channels (
			id TEXT PRIMARY KEY,
			guild_id TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			message_limit INTEGER DEFAULT 10,
			excluded_message_ids TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (guild_id) REFERENCES servers(id)
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %s, error: %w", query, err)
		}
	}

	return nil
}

// createUserTables 创建用户相关表
func (s *Service) createUserTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			discriminator TEXT,
			global_name TEXT,
			avatar_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_statistics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			guild_id TEXT NOT NULL,
			total_posts INTEGER DEFAULT 0,
			total_rolls INTEGER DEFAULT 0,
			last_activity DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, guild_id)
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %s, error: %w", query, err)
		}
	}

	return nil
}

// createTimedTaskTables 创建定时任务相关表
func (s *Service) createTimedTaskTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS timed_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guild_id TEXT NOT NULL,
			task_type TEXT NOT NULL,
			task_data TEXT,
			schedule_time DATETIME NOT NULL,
			executed BOOLEAN DEFAULT FALSE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_timed_tasks_guild_id ON timed_tasks(guild_id)`,
		`CREATE INDEX IF NOT EXISTS idx_timed_tasks_schedule_time ON timed_tasks(schedule_time)`,
		`CREATE INDEX IF NOT EXISTS idx_timed_tasks_executed ON timed_tasks(executed)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %s, error: %w", query, err)
		}
	}

	return nil
}

// Close 关闭数据库服务
func (s *Service) Close() error {
	return s.pool.Close()
}

// GetStats 获取连接池统计信息
func (s *Service) GetStats() map[string]sql.DBStats {
	return s.pool.GetStats()
}

// LogStats 记录连接池统计信息
func (s *Service) LogStats() {
	s.pool.LogStats()
}

// PingAll 测试所有数据库连接
func (s *Service) PingAll() error {
	return s.pool.PingAll()
}