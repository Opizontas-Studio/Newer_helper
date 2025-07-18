package bot

import (
	"database/sql"
	"discord-bot/commands"
	"discord-bot/config"
	"discord-bot/handlers/leaderboard"
	"discord-bot/internal/services"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Bot 重构后的Bot结构，移除上帝对象特征
type Bot struct {
	// 核心服务依赖
	discord     services.DiscordService
	command     services.CommandService
	scheduler   services.SchedulerService
	cooldown    services.CooldownService
	
	// 基础配置和数据库
	config      atomic.Value // *model.Config
	database    *sql.DB
	
	// 扫描相关状态
	ActiveScanCount int
	done            chan struct{}
	
	// 向后兼容的方法
	commandHandlers map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
	mu              sync.RWMutex
}

func (b *Bot) GetConfig() *model.Config {
	return b.config.Load().(*model.Config)
}

func (b *Bot) GetDB() *sql.DB {
	return b.database
}

func (b *Bot) GetSession() *discordgo.Session {
	return b.discord.GetSession()
}

// 服务访问器
func (b *Bot) GetDiscordService() services.DiscordService {
	return b.discord
}

func (b *Bot) GetCommandService() services.CommandService {
	return b.command
}

func (b *Bot) GetSchedulerService() services.SchedulerService {
	return b.scheduler
}

func (b *Bot) GetCooldownService() services.CooldownService {
	return b.cooldown
}

// 向后兼容的方法
func (b *Bot) GetPresetCooldowns() map[string]time.Time {
	// 为向后兼容，我们返回一个空map
	// 新的冷却逻辑通过CooldownService处理
	return make(map[string]time.Time)
}

func (b *Bot) GetCooldownMutex() *sync.Mutex {
	// 为向后兼容，返回一个虚拟的mutex
	// 新的冷却逻辑通过CooldownService处理
	return &sync.Mutex{}
}

// NewBot 创建新的Bot实例，使用依赖注入
func NewBot(discord interface{}, command interface{}, scheduler interface{}, cooldown interface{}, cfg *model.Config, db *sql.DB) (*Bot, error) {
	// 类型断言
	discordSvc, ok := discord.(services.DiscordService)
	if !ok {
		return nil, fmt.Errorf("discord service type assertion failed")
	}
	
	commandSvc, ok := command.(services.CommandService)
	if !ok {
		return nil, fmt.Errorf("command service type assertion failed")
	}
	
	schedulerSvc, ok := scheduler.(services.SchedulerService)
	if !ok {
		return nil, fmt.Errorf("scheduler service type assertion failed")
	}
	
	cooldownSvc, ok := cooldown.(services.CooldownService)
	if !ok {
		return nil, fmt.Errorf("cooldown service type assertion failed")
	}
	
	b := &Bot{
		discord:         discordSvc,
		command:         commandSvc,
		scheduler:       schedulerSvc,
		cooldown:        cooldownSvc,
		database:        db,
		ActiveScanCount: 0,
		done:            make(chan struct{}),
		commandHandlers: make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)),
	}
	b.config.Store(cfg)
	return b, nil
}

// New 保持向后兼容的构造函数
func New(cfg *model.Config, db *sql.DB) (*Bot, error) {
	// 创建服务
	discord, err := services.NewDiscordService(cfg.BotToken)
	if err != nil {
		return nil, err
	}
	
	command, err := services.NewCommandService(discord, cfg)
	if err != nil {
		return nil, err
	}
	
	scheduler := services.NewSchedulerService()
	cooldown := services.NewCooldownService()
	
	return NewBot(discord, command, scheduler, cooldown, cfg, db)
}

func (b *Bot) Close() {
	log.Println("Gracefully shutting down.")
	close(b.done) // Signal all goroutines to stop

	// 停止调度器
	if b.scheduler != nil {
		b.scheduler.Stop()
	}
	
	// 关闭Discord连接
	if b.discord != nil {
		b.discord.Close()
	}
}

func (b *Bot) UpdateLeaderboard() {
	states, err := utils.LoadLeaderboardState()
	if err != nil {
		log.Printf("Error loading leaderboard state for update: %v", err)
		return
	}

	var wg sync.WaitGroup
	workerLimit := 5 // Limit to 5 concurrent workers
	guard := make(chan struct{}, workerLimit)

	for guildID := range states {
		wg.Add(1)
		guard <- struct{}{} // Acquire a worker slot

		go func(guildID string) {
			defer func() {
				<-guard // Release the worker slot
				wg.Done()
			}()
			log.Printf("Updating leaderboard for guild: %s", guildID)
			leaderboard.UpdateLeaderboard(b, guildID)
		}(guildID)
	}

	wg.Wait()
}

func (b *Bot) RefreshCommands(guildID string) {
	if err := b.command.RefreshCommands(guildID); err != nil {
		log.Printf("Failed to refresh commands for guild %s: %v", guildID, err)
	}
}

func (b *Bot) CleanupCooldowns() {
	b.cooldown.CleanupExpired()
}

func (b *Bot) ReloadConfig() error {
	log.Println("Reloading configuration...")
	newCfg, err := config.Load()
	if err != nil {
		log.Printf("Error reloading config: %v", err)
		return err
	}

	// Load server-specific configs from DB
	if err := database.LoadConfigFromDB(b.database, newCfg); err != nil {
		log.Printf("Error loading config from database during reload: %v", err)
		return err
	}

	b.config.Store(newCfg)
	log.Println("Configuration reloaded successfully.")

	log.Println("Refreshing commands for all guilds...")
	for _, serverCfg := range newCfg.ServerConfigs {
		go b.RefreshCommands(serverCfg.GuildID)
	}

	return nil
}
