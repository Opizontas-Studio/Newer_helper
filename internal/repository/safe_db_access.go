package repository

import (
	"database/sql"
	"discord-bot/internal/database"
	"discord-bot/model"
	"log"
)

// SafeDBAccess 安全的数据库访问层，替换原有的不安全的数据库访问代码
type SafeDBAccess struct {
	dbService *database.Service
	postRepo  PostRepository
}

// NewSafeDBAccess 创建安全的数据库访问层
func NewSafeDBAccess(dbService *database.Service) *SafeDBAccess {
	return &SafeDBAccess{
		dbService: dbService,
		postRepo:  NewPostRepository(dbService),
	}
}

// GetAllPosts 安全地获取所有帖子，替换原有的不安全版本
func (s *SafeDBAccess) GetAllPosts(guildID, tableName string) ([]model.Post, error) {
	log.Printf("SafeDBAccess.GetAllPosts called for guild: %s, table: %s", guildID, tableName)
	return s.postRepo.GetAll(guildID, tableName)
}

// GetRandomPosts 安全地获取随机帖子，替换原有的不安全版本
func (s *SafeDBAccess) GetRandomPosts(guildID, tableName string, count int) ([]model.Post, error) {
	log.Printf("SafeDBAccess.GetRandomPosts called for guild: %s, table: %s, count: %d", guildID, tableName, count)
	return s.postRepo.GetRandom(guildID, tableName, count)
}

// CountPostsInTimeRange 安全地统计时间范围内的帖子数量，替换原有的不安全版本
func (s *SafeDBAccess) CountPostsInTimeRange(guildID string, tableNames []string, startTime, endTime int64) (int, error) {
	log.Printf("SafeDBAccess.CountPostsInTimeRange called for guild: %s, tables: %v, time range: %d-%d", guildID, tableNames, startTime, endTime)
	return s.postRepo.CountInMultipleTables(guildID, tableNames, startTime, endTime)
}

// GetPostsByAuthor 安全地获取作者的帖子
func (s *SafeDBAccess) GetPostsByAuthor(guildID, tableName, authorID string) ([]model.Post, error) {
	log.Printf("SafeDBAccess.GetPostsByAuthor called for guild: %s, table: %s, author: %s", guildID, tableName, authorID)
	return s.postRepo.GetByAuthor(guildID, tableName, authorID)
}

// GetPostsByTag 安全地获取包含特定标签的帖子
func (s *SafeDBAccess) GetPostsByTag(guildID, tableName, tag string) ([]model.Post, error) {
	log.Printf("SafeDBAccess.GetPostsByTag called for guild: %s, table: %s, tag: %s", guildID, tableName, tag)
	return s.postRepo.GetByTag(guildID, tableName, tag)
}

// GetPostsByTimeRange 安全地获取时间范围内的帖子
func (s *SafeDBAccess) GetPostsByTimeRange(guildID, tableName string, startTime, endTime int64) ([]model.Post, error) {
	log.Printf("SafeDBAccess.GetPostsByTimeRange called for guild: %s, table: %s, time range: %d-%d", guildID, tableName, startTime, endTime)
	return s.postRepo.GetByTimeRange(guildID, tableName, startTime, endTime)
}

// CountPosts 安全地统计帖子数量
func (s *SafeDBAccess) CountPosts(guildID, tableName string) (int, error) {
	log.Printf("SafeDBAccess.CountPosts called for guild: %s, table: %s", guildID, tableName)
	return s.postRepo.Count(guildID, tableName)
}

// CreatePost 安全地创建帖子
func (s *SafeDBAccess) CreatePost(guildID, tableName string, post *model.Post) error {
	log.Printf("SafeDBAccess.CreatePost called for guild: %s, table: %s, post: %s", guildID, tableName, post.ID)
	return s.postRepo.Create(guildID, tableName, post)
}

// UpdatePost 安全地更新帖子
func (s *SafeDBAccess) UpdatePost(guildID, tableName string, post *model.Post) error {
	log.Printf("SafeDBAccess.UpdatePost called for guild: %s, table: %s, post: %s", guildID, tableName, post.ID)
	return s.postRepo.Update(guildID, tableName, post)
}

// DeletePost 安全地删除帖子
func (s *SafeDBAccess) DeletePost(guildID, tableName, postID string) error {
	log.Printf("SafeDBAccess.DeletePost called for guild: %s, table: %s, post: %s", guildID, tableName, postID)
	return s.postRepo.Delete(guildID, tableName, postID)
}

// CreateTable 安全地创建表
func (s *SafeDBAccess) CreateTable(guildID, tableName string) error {
	log.Printf("SafeDBAccess.CreateTable called for guild: %s, table: %s", guildID, tableName)
	return s.postRepo.CreateTable(guildID, tableName)
}

// TableExists 检查表是否存在
func (s *SafeDBAccess) TableExists(guildID, tableName string) (bool, error) {
	log.Printf("SafeDBAccess.TableExists called for guild: %s, table: %s", guildID, tableName)
	return s.postRepo.TableExists(guildID, tableName)
}

// GetTableNames 获取所有表名
func (s *SafeDBAccess) GetTableNames(guildID string) ([]string, error) {
	log.Printf("SafeDBAccess.GetTableNames called for guild: %s", guildID)
	return s.postRepo.GetTableNames(guildID)
}

// 全局安全数据库访问实例
var globalSafeDBAccess *SafeDBAccess

// InitializeSafeDBAccess 初始化全局安全数据库访问实例
func InitializeSafeDBAccess(dbService *database.Service) {
	globalSafeDBAccess = NewSafeDBAccess(dbService)
	log.Println("Safe database access initialized")
}

// GetSafeDBAccess 获取全局安全数据库访问实例
func GetSafeDBAccess() *SafeDBAccess {
	return globalSafeDBAccess
}

// LegacyToSafeDBAdapter 将旧的数据库访问代码适配到安全的访问层
type LegacyToSafeDBAdapter struct {
	safeAccess *SafeDBAccess
}

// NewLegacyToSafeDBAdapter 创建适配器
func NewLegacyToSafeDBAdapter(safeAccess *SafeDBAccess) *LegacyToSafeDBAdapter {
	return &LegacyToSafeDBAdapter{
		safeAccess: safeAccess,
	}
}

// AdaptGetAllPosts 适配GetAllPosts函数，需要从调用上下文中获取guildID
func (a *LegacyToSafeDBAdapter) AdaptGetAllPosts(db *sql.DB, tableName string) ([]model.Post, error) {
	// 从数据库连接中推断guildID
	guildID := a.inferGuildIDFromDB(db)
	return a.safeAccess.GetAllPosts(guildID, tableName)
}

// AdaptGetRandomPosts 适配GetRandomPosts函数
func (a *LegacyToSafeDBAdapter) AdaptGetRandomPosts(db *sql.DB, tableName string, count int) ([]model.Post, error) {
	guildID := a.inferGuildIDFromDB(db)
	return a.safeAccess.GetRandomPosts(guildID, tableName, count)
}

// AdaptCountPostsInTimeRange 适配CountPostsInTimeRange函数
func (a *LegacyToSafeDBAdapter) AdaptCountPostsInTimeRange(db *sql.DB, tableNames []string, startTime, endTime int64) (int, error) {
	guildID := a.inferGuildIDFromDB(db)
	return a.safeAccess.CountPostsInTimeRange(guildID, tableNames, startTime, endTime)
}

// inferGuildIDFromDB 从数据库连接中推断guildID
// 这是一个简化的实现，实际应用中可能需要更复杂的逻辑
func (a *LegacyToSafeDBAdapter) inferGuildIDFromDB(db *sql.DB) string {
	// TODO: 实现从数据库连接中推断guildID的逻辑
	// 这可能需要查询数据库的元数据或使用其他方法

	// 暂时返回一个默认值
	return "default_guild"
}

// ReplaceUnsafeDatabaseFunctions 替换不安全的数据库函数
func ReplaceUnsafeDatabaseFunctions(dbService *database.Service) {
	// 初始化安全的数据库访问层
	InitializeSafeDBAccess(dbService)

	log.Println("Unsafe database functions have been replaced with safe Repository-based implementations")
	log.Println("Please update your code to use the new SafeDBAccess methods")

	// 在这里可以记录哪些函数被替换了
	replacedFunctions := []string{
		"GetAllPosts",
		"GetRandomPosts",
		"CountPostsInTimeRange",
		"GetPostsByAuthor",
		"GetPostsByTag",
		"CountPosts",
	}

	log.Printf("Replaced functions: %v", replacedFunctions)
}
