package database

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// PoolConfig 数据库连接池配置
type PoolConfig struct {
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// DefaultPoolConfig 默认连接池配置
var DefaultPoolConfig = PoolConfig{
	MaxOpenConns:    25,
	MaxIdleConns:    5,
	ConnMaxLifetime: 1 * time.Hour,
	ConnMaxIdleTime: 10 * time.Minute,
}

// Pool 数据库连接池
type Pool struct {
	connections map[string]*sql.DB
	config      PoolConfig
	mu          sync.RWMutex
}

// NewPool 创建新的数据库连接池
func NewPool(config PoolConfig) *Pool {
	return &Pool{
		connections: make(map[string]*sql.DB),
		config:      config,
	}
}

// NewPoolWithDefault 使用默认配置创建数据库连接池
func NewPoolWithDefault() *Pool {
	return NewPool(DefaultPoolConfig)
}

// GetConnection 获取数据库连接
func (p *Pool) GetConnection(dbPath string) (*sql.DB, error) {
	p.mu.RLock()
	if db, exists := p.connections[dbPath]; exists {
		p.mu.RUnlock()
		return db, nil
	}
	p.mu.RUnlock()

	// 需要创建新连接
	p.mu.Lock()
	defer p.mu.Unlock()

	// 双重检查，防止并发创建
	if db, exists := p.connections[dbPath]; exists {
		return db, nil
	}

	// 创建新的数据库连接
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}

	// 配置连接池参数
	db.SetMaxOpenConns(p.config.MaxOpenConns)
	db.SetMaxIdleConns(p.config.MaxIdleConns)
	db.SetConnMaxLifetime(p.config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(p.config.ConnMaxIdleTime)

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database %s: %w", dbPath, err)
	}

	p.connections[dbPath] = db
	log.Printf("Created new database connection pool for: %s", dbPath)
	return db, nil
}

// GetGuildDB 获取服务器数据库连接
func (p *Pool) GetGuildDB(guildID string) (*sql.DB, error) {
	dbPath := fmt.Sprintf("./data/%s.db", guildID)
	return p.GetConnection(dbPath)
}

// GetUserDB 获取用户数据库连接
func (p *Pool) GetUserDB() (*sql.DB, error) {
	return p.GetConnection("./data/user.db")
}

// GetGuildsDB 获取服务器配置数据库连接
func (p *Pool) GetGuildsDB() (*sql.DB, error) {
	return p.GetConnection("./data/guilds.db")
}

// GetKickUserDB 获取踢人用户数据库连接
func (p *Pool) GetKickUserDB() (*sql.DB, error) {
	return p.GetConnection("./data/kick_user.db")
}

// GetTimedTasksDB 获取定时任务数据库连接
func (p *Pool) GetTimedTasksDB() (*sql.DB, error) {
	return p.GetConnection("./data/timed_tasks.db")
}

// GetNewPostDB 获取新帖子数据库连接
func (p *Pool) GetNewPostDB(guildID string) (*sql.DB, error) {
	dbPath := fmt.Sprintf("./data/new_post/%s.db", guildID)
	return p.GetConnection(dbPath)
}

// Close 关闭所有数据库连接
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errors []error
	for dbPath, db := range p.connections {
		if err := db.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close database %s: %w", dbPath, err))
		} else {
			log.Printf("Closed database connection: %s", dbPath)
		}
	}

	// 清空连接映射
	p.connections = make(map[string]*sql.DB)

	if len(errors) > 0 {
		return fmt.Errorf("errors closing databases: %v", errors)
	}

	return nil
}

// GetStats 获取连接池统计信息
func (p *Pool) GetStats() map[string]sql.DBStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]sql.DBStats)
	for dbPath, db := range p.connections {
		stats[dbPath] = db.Stats()
	}
	return stats
}

// LogStats 记录连接池统计信息
func (p *Pool) LogStats() {
	stats := p.GetStats()
	for dbPath, stat := range stats {
		log.Printf("DB[%s] - OpenConnections: %d, InUse: %d, Idle: %d",
			dbPath, stat.OpenConnections, stat.InUse, stat.Idle)
	}
}

// PingAll 测试所有数据库连接
func (p *Pool) PingAll() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for dbPath, db := range p.connections {
		if err := db.Ping(); err != nil {
			return fmt.Errorf("failed to ping database %s: %w", dbPath, err)
		}
	}
	return nil
}