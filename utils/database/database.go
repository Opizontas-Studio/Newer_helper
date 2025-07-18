package database

import (
	"database/sql"
	"discord-bot/internal/database"
	"discord-bot/model"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// 全局数据库服务实例
var (
	dbService *database.Service
	legacyAdapter *database.LegacyDatabaseAdapter
)

// Initialize 初始化数据库系统
func Initialize() error {
	dbService = database.NewServiceWithDefault()
	legacyAdapter = database.NewLegacyDatabaseAdapter(dbService)

	// 初始化数据库结构
	if err := dbService.InitializeDatabase(); err != nil {
		return err
	}

	log.Println("Database system initialized successfully")
	return nil
}

// InitDB 兼容旧的数据库初始化函数
func InitDB(filepath string) (*sql.DB, error) {
	if dbService == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return legacyAdapter.InitDB(filepath)
}

// 注意：CreateGuildTables, InitUserDB, LoadConfigFromDB 已在其他文件中定义
// 这里提供新的连接池版本，通过不同的函数名避免冲突

// CreateGuildTablesV2 创建服务器表（新版本，使用连接池）
func CreateGuildTablesV2(db *sql.DB) error {
	if legacyAdapter == nil {
		if err := Initialize(); err != nil {
			return err
		}
	}
	return legacyAdapter.CreateGuildTables(db)
}

// InitUserDBV2 初始化用户数据库（新版本，使用连接池）
func InitUserDBV2() (*sql.DB, error) {
	if legacyAdapter == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return legacyAdapter.InitUserDB()
}

// LoadConfigFromDBV2 从数据库加载配置（新版本，使用连接池）
func LoadConfigFromDBV2(db *sql.DB, cfg *model.Config) error {
	if legacyAdapter == nil {
		if err := Initialize(); err != nil {
			return err
		}
	}
	return legacyAdapter.LoadConfigFromDB(db, cfg)
}

// GetService 获取数据库服务实例
func GetService() *database.Service {
	return dbService
}

// GetLegacyAdapter 获取兼容性适配器
func GetLegacyAdapter() *database.LegacyDatabaseAdapter {
	return legacyAdapter
}

// GetDBSize 获取数据库文件大小
func GetDBSize(filepath string) (int64, error) {
	fi, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// Close 关闭数据库服务
func Close() error {
	if dbService != nil {
		return dbService.Close()
	}
	return nil
}

// GetGuildDB 获取服务器数据库连接（新增便捷方法）
func GetGuildDB(guildID string) (*sql.DB, error) {
	if legacyAdapter == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return legacyAdapter.GetGuildDatabase(guildID)
}

// GetUserDB 获取用户数据库连接（新增便捷方法）
func GetUserDB() (*sql.DB, error) {
	if legacyAdapter == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return legacyAdapter.GetUserDatabase()
}

// GetKickUserDB 获取踢人用户数据库连接（新增便捷方法）
func GetKickUserDB() (*sql.DB, error) {
	if legacyAdapter == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return legacyAdapter.GetKickUserDatabase()
}

// GetTimedTasksDB 获取定时任务数据库连接（新增便捷方法）
func GetTimedTasksDB() (*sql.DB, error) {
	if legacyAdapter == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return legacyAdapter.GetTimedTasksDatabase()
}

// GetNewPostDB 获取新帖子数据库连接（新增便捷方法）
func GetNewPostDB(guildID string) (*sql.DB, error) {
	if legacyAdapter == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return legacyAdapter.GetNewPostDatabase(guildID)
}
