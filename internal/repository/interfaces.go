package repository

import (
	"discord-bot/model"
	"time"
)

// PostRepository 帖子数据访问接口
type PostRepository interface {
	// 基本CRUD操作
	GetByID(guildID, tableName, postID string) (*model.Post, error)
	GetAll(guildID, tableName string) ([]model.Post, error)
	GetRandom(guildID, tableName string, count int) ([]model.Post, error)
	Create(guildID, tableName string, post *model.Post) error
	Update(guildID, tableName string, post *model.Post) error
	Delete(guildID, tableName, postID string) error
	
	// 条件查询
	GetByAuthor(guildID, tableName, authorID string) ([]model.Post, error)
	GetByTag(guildID, tableName, tag string) ([]model.Post, error)
	GetByTimeRange(guildID, tableName string, startTime, endTime int64) ([]model.Post, error)
	
	// 统计功能
	Count(guildID, tableName string) (int, error)
	CountByAuthor(guildID, tableName, authorID string) (int, error)
	CountByTag(guildID, tableName, tag string) (int, error)
	CountByTimeRange(guildID, tableName string, startTime, endTime int64) (int, error)
	CountInMultipleTables(guildID string, tableNames []string, startTime, endTime int64) (int, error)
	
	// 表管理
	CreateTable(guildID, tableName string) error
	DropTable(guildID, tableName string) error
	TableExists(guildID, tableName string) (bool, error)
	GetTableNames(guildID string) ([]string, error)
}

// UserRepository 用户数据访问接口
type UserRepository interface {
	// 用户基本信息
	GetByID(userID string) (*model.User, error)
	Create(user *model.User) error
	Update(user *model.User) error
	Delete(userID string) error
	
	// 用户统计
	GetUserStats(userID, guildID string) (*model.UserStats, error)
	UpdateUserStats(userID, guildID string, stats *model.UserStats) error
	
	// 用户偏好
	GetUserPreferences(userID, guildID string) ([]string, error)
	SetUserPreferences(userID, guildID string, preferences []string) error
	
	// 用户统计
	GetTotalUserCount() (int, error)
	GetActiveUserCount(guildID string, since time.Time) (int, error)
}

// PunishmentRepository 惩罚系统数据访问接口
type PunishmentRepository interface {
	// 惩罚记录
	GetByID(punishmentID string) (*model.Punishment, error)
	GetByUserID(userID string) ([]model.Punishment, error)
	GetByGuildID(guildID string) ([]model.Punishment, error)
	Create(punishment *model.Punishment) error
	Update(punishment *model.Punishment) error
	Delete(punishmentID string) error
	
	// 统计查询
	CountByUser(userID string) (int, error)
	CountByGuild(guildID string) (int, error)
	GetRecentPunishments(guildID string, limit int) ([]model.Punishment, error)
}

// GuildRepository 服务器配置数据访问接口
type GuildRepository interface {
	// 服务器配置
	GetConfig(guildID string) (*model.ServerConfig, error)
	SaveConfig(guildID string, config *model.ServerConfig) error
	DeleteConfig(guildID string) error
	GetAllConfigs() (map[string]model.ServerConfig, error)
	
	// 预设消息
	GetPresetMessages(guildID string) ([]model.PresetMessage, error)
	AddPresetMessage(guildID string, message *model.PresetMessage) error
	UpdatePresetMessage(guildID string, message *model.PresetMessage) error
	DeletePresetMessage(guildID, messageID string) error
	
	// 回顶频道配置
	GetTopChannels(guildID string) (map[string]*model.TopChannelConfig, error)
	SaveTopChannelConfig(guildID string, config *model.TopChannelConfig) error
	DeleteTopChannelConfig(guildID, channelID string) error
}

// TimedTaskRepository 定时任务数据访问接口
type TimedTaskRepository interface {
	// 任务管理
	GetByID(taskID int) (*model.TimedTask, error)
	GetByGuildID(guildID string) ([]model.TimedTask, error)
	GetPendingTasks(before time.Time) ([]model.TimedTask, error)
	Create(task *model.TimedTask) error
	Update(task *model.TimedTask) error
	Delete(taskID int) error
	
	// 任务执行
	MarkAsExecuted(taskID int) error
	GetExecutedTasks(guildID string, limit int) ([]model.TimedTask, error)
	
	// 清理
	CleanupExecutedTasks(before time.Time) error
}

// LeaderboardRepository 排行榜数据访问接口
type LeaderboardRepository interface {
	// 排行榜数据
	GetTopPosters(guildID string, limit int) ([]model.LeaderboardEntry, error)
	GetTopRollers(guildID string, limit int) ([]model.LeaderboardEntry, error)
	GetUserRanking(guildID, userID string) (*model.UserRanking, error)
	
	// 统计更新
	UpdateUserPostCount(guildID, userID string, count int) error
	UpdateUserRollCount(guildID, userID string, count int) error
	
	// 缓存管理
	RefreshLeaderboard(guildID string) error
	GetLastUpdated(guildID string) (time.Time, error)
}

// RepositoryManager 仓库管理器接口
type RepositoryManager interface {
	// 获取各种仓库实例
	PostRepository() PostRepository
	UserRepository() UserRepository
	PunishmentRepository() PunishmentRepository
	GuildRepository() GuildRepository
	TimedTaskRepository() TimedTaskRepository
	LeaderboardRepository() LeaderboardRepository
	
	// 事务管理
	BeginTransaction() (Transaction, error)
	
	// 连接管理
	Close() error
	Ping() error
	GetStats() map[string]interface{}
}

// Transaction 事务接口
type Transaction interface {
	// 事务操作
	Commit() error
	Rollback() error
	
	// 在事务中获取仓库实例
	PostRepository() PostRepository
	UserRepository() UserRepository
	PunishmentRepository() PunishmentRepository
	GuildRepository() GuildRepository
	TimedTaskRepository() TimedTaskRepository
	LeaderboardRepository() LeaderboardRepository
}