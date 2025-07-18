package services

import (
	"database/sql"
	"discord-bot/model"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// DiscordService 管理Discord会话
type DiscordService interface {
	// GetSession 获取Discord会话
	GetSession() *discordgo.Session
	// Connect 连接到Discord
	Connect() error
	// Close 关闭连接
	Close() error
}

// CommandService 管理命令注册和处理
type CommandService interface {
	// RegisterCommands 注册命令到Discord
	RegisterCommands(guildID string, commands []*discordgo.ApplicationCommand) error
	// RefreshCommands 刷新特定服务器的命令
	RefreshCommands(guildID string) error
	// GetRegisteredCommands 获取已注册的命令
	GetRegisteredCommands() []*discordgo.ApplicationCommand
	// SetCommandHandlers 设置命令处理器
	SetCommandHandlers(handlers map[string]func(*discordgo.Session, *discordgo.InteractionCreate))
}

// CooldownService 管理冷却时间
type CooldownService interface {
	// SetCooldown 设置冷却时间
	SetCooldown(key string, duration time.Duration)
	// IsOnCooldown 检查是否在冷却期
	IsOnCooldown(key string) bool
	// GetCooldownRemaining 获取剩余冷却时间
	GetCooldownRemaining(key string) time.Duration
	// CleanupExpired 清理过期的冷却记录
	CleanupExpired()
}

// SchedulerService 管理定时任务
type SchedulerService interface {
	// AddJob 添加定时任务
	AddJob(name string, interval time.Duration, job func())
	// RemoveJob 移除定时任务
	RemoveJob(name string)
	// Start 启动调度器
	Start()
	// Stop 停止调度器
	Stop()
}

// ConfigService 管理配置
type ConfigService interface {
	// GetConfig 获取配置
	GetConfig() *model.Config
	// ReloadConfig 重新加载配置
	ReloadConfig() error
	// LoadConfigFromDB 从数据库加载配置
	LoadConfigFromDB(db *sql.DB) error
}

// LeaderboardService 管理排行榜
type LeaderboardService interface {
	// UpdateLeaderboard 更新排行榜
	UpdateLeaderboard(guildID string)
	// UpdateAllLeaderboards 更新所有排行榜
	UpdateAllLeaderboards()
}

// ApplicationService 应用程序主服务
type ApplicationService interface {
	// Start 启动应用程序
	Start() error
	// Stop 停止应用程序
	Stop() error
	// GetContainer 获取服务容器
	GetContainer() Container
}

// Container 服务容器接口
type Container interface {
	// Register 注册服务
	Register(name string, service interface{})
	// Get 获取服务
	Get(name string) (interface{}, error)
	// GetTyped 获取指定类型的服务
	GetTyped(name string, target interface{}) error
}

// ServiceDependencies 服务依赖结构
type ServiceDependencies struct {
	Config      *model.Config
	Database    *sql.DB
	Discord     DiscordService
	Command     CommandService
	Cooldown    CooldownService
	Scheduler   SchedulerService
	Leaderboard LeaderboardService
}

// CooldownEntry 冷却条目
type CooldownEntry struct {
	ExpireTime time.Time
	mutex      sync.RWMutex
}

// IsExpired 检查是否过期
func (c *CooldownEntry) IsExpired() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return time.Now().After(c.ExpireTime)
}

// GetRemainingTime 获取剩余时间
func (c *CooldownEntry) GetRemainingTime() time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	remaining := c.ExpireTime.Sub(time.Now())
	if remaining < 0 {
		return 0
	}
	return remaining
}
