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
)

// Initialize 初始化数据库系统
func Initialize() error {
	dbService = database.NewServiceWithDefault()

	// 初始化数据库结构
	if err := dbService.InitializeDatabase(); err != nil {
		return err
	}

	log.Println("Database system initialized successfully")
	return nil
}

// InitDB 数据库初始化函数
func InitDB(filepath string) (*sql.DB, error) {
	if dbService == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return dbService.GetPool().GetConnection(filepath)
}

// CreateGuildTablesV2 创建服务器表（新版本，使用连接池）
func CreateGuildTablesV2(db *sql.DB) error {
	if dbService == nil {
		if err := Initialize(); err != nil {
			return err
		}
	}
	return dbService.InitializeDatabase()
}

// InitUserDBV2 初始化用户数据库（新版本，使用连接池）
func InitUserDBV2() (*sql.DB, error) {
	if dbService == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return dbService.GetPool().GetUserDB()
}

// LoadConfigFromDBV2 从数据库加载配置（新版本，使用连接池）
func LoadConfigFromDBV2(db *sql.DB, cfg *model.Config) error {
	// 这个函数现在直接返回nil，因为配置已经通过新系统加载
	return nil
}

// GetService 获取数据库服务实例
func GetService() *database.Service {
	return dbService
}

// GetPool 获取数据库连接池
func GetPool() *database.Pool {
	if dbService == nil {
		return nil
	}
	return dbService.GetPool()
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

// GetGuildDB 获取服务器数据库连接
func GetGuildDB(guildID string) (*sql.DB, error) {
	if dbService == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return dbService.GetPool().GetGuildDB(guildID)
}

// GetUserDB 获取用户数据库连接
func GetUserDB() (*sql.DB, error) {
	if dbService == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return dbService.GetPool().GetUserDB()
}

// GetKickUserDB 获取踢人用户数据库连接
func GetKickUserDB() (*sql.DB, error) {
	if dbService == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return dbService.GetPool().GetKickUserDB()
}

// GetTimedTasksDB 获取定时任务数据库连接
func GetTimedTasksDB() (*sql.DB, error) {
	if dbService == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return dbService.GetPool().GetTimedTasksDB()
}

// GetNewPostDB 获取新帖子数据库连接
func GetNewPostDB(guildID string) (*sql.DB, error) {
	if dbService == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}
	return dbService.GetPool().GetNewPostDB(guildID)
}
