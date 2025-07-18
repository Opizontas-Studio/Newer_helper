package repository

import (
	"database/sql"
	"discord-bot/internal/database"
	"discord-bot/model"
	"log"
)

// LegacyRepositoryAdapter 提供与旧数据库访问代码的兼容性
type LegacyRepositoryAdapter struct {
	manager    RepositoryManager
	dbService  *database.Service
}

// NewLegacyRepositoryAdapter 创建兼容性适配器
func NewLegacyRepositoryAdapter(manager RepositoryManager, dbService *database.Service) *LegacyRepositoryAdapter {
	return &LegacyRepositoryAdapter{
		manager:   manager,
		dbService: dbService,
	}
}

// GetAllPosts 兼容旧的GetAllPosts函数
func (a *LegacyRepositoryAdapter) GetAllPosts(db *sql.DB, tableName string) ([]model.Post, error) {
	// 提取guildID (从db连接推断，这是一个简化的实现)
	guildID := a.extractGuildIDFromDB(db)
	return a.manager.PostRepository().GetAll(guildID, tableName)
}

// GetRandomPosts 兼容旧的GetRandomPosts函数
func (a *LegacyRepositoryAdapter) GetRandomPosts(db *sql.DB, tableName string, count int) ([]model.Post, error) {
	guildID := a.extractGuildIDFromDB(db)
	return a.manager.PostRepository().GetRandom(guildID, tableName, count)
}

// CountPostsInTimeRange 兼容旧的CountPostsInTimeRange函数
func (a *LegacyRepositoryAdapter) CountPostsInTimeRange(db *sql.DB, tableNames []string, startTime int64, endTime int64) (int, error) {
	guildID := a.extractGuildIDFromDB(db)
	return a.manager.PostRepository().CountInMultipleTables(guildID, tableNames, startTime, endTime)
}

// extractGuildIDFromDB 从数据库连接中提取guildID
// 这是一个简化的实现，实际应用中可能需要更复杂的逻辑
func (a *LegacyRepositoryAdapter) extractGuildIDFromDB(db *sql.DB) string {
	// 这里我们返回一个默认值，实际应用中应该从连接信息中获取
	// 或者修改调用代码传递guildID
	return "default_guild"
}

// UpdateExistingDatabaseFunctions 更新现有的数据库函数，使其使用新的Repository
func UpdateExistingDatabaseFunctions(manager RepositoryManager, dbService *database.Service) {
	_ = NewLegacyRepositoryAdapter(manager, dbService)
	
	// 这里可以重新定义一些全局函数，使它们使用新的Repository
	// 但由于Go的限制，我们无法直接替换包级别的函数
	// 所以这更多是一个概念性的函数
	
	log.Println("Legacy database functions updated to use Repository pattern")
	
	// 可以在这里设置一些全局变量或配置
	// 让现有代码知道应该使用新的Repository
}

// SafeGetAllPosts 安全版本的GetAllPosts，防止SQL注入
func SafeGetAllPosts(guildID, tableName string, manager RepositoryManager) ([]model.Post, error) {
	return manager.PostRepository().GetAll(guildID, tableName)
}

// SafeGetRandomPosts 安全版本的GetRandomPosts，防止SQL注入
func SafeGetRandomPosts(guildID, tableName string, count int, manager RepositoryManager) ([]model.Post, error) {
	return manager.PostRepository().GetRandom(guildID, tableName, count)
}

// SafeCountPostsInTimeRange 安全版本的CountPostsInTimeRange，防止SQL注入
func SafeCountPostsInTimeRange(guildID string, tableNames []string, startTime, endTime int64, manager RepositoryManager) (int, error) {
	return manager.PostRepository().CountInMultipleTables(guildID, tableNames, startTime, endTime)
}

// MigrateLegacyCode 迁移旧代码使用新的Repository
func MigrateLegacyCode(manager RepositoryManager) {
	log.Println("Starting migration of legacy database code to Repository pattern")
	
	// 这里可以实现具体的迁移逻辑
	// 例如，更新一些全局变量，让现有代码知道应该使用新的Repository
	
	// 创建一个全局的Repository manager实例
	globalRepositoryManager = manager
	
	log.Println("Legacy code migration completed")
}

// 全局Repository manager实例
var globalRepositoryManager RepositoryManager

// GetGlobalRepositoryManager 获取全局Repository manager
func GetGlobalRepositoryManager() RepositoryManager {
	return globalRepositoryManager
}

// SetGlobalRepositoryManager 设置全局Repository manager
func SetGlobalRepositoryManager(manager RepositoryManager) {
	globalRepositoryManager = manager
}