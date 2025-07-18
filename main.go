//go:build !migrate

package main

import (
	"discord-bot/bot"
	"discord-bot/config"
	"discord-bot/handlers"
	"discord-bot/internal/container"
	"discord-bot/internal/repository"
	"discord-bot/utils/database"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	internal_config "discord-bot/internal/config"
)

func startPprofServer() {
	if os.Getenv("ENABLE_PPROF") == "true" {
		log.Println("Starting pprof server on :6060")
		go func() {
			if err := http.ListenAndServe(":6060", nil); err != nil {
				log.Printf("Failed to start pprof server: %v", err)
			}
		}()
	}
}

func main() {
	startPprofServer()

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// 初始化数据库
	db, err := database.InitDB("./data/guilds.db")
	if err != nil {
		log.Fatalf("Error setting up database: %v", err)
	}
	if err := database.CreateGuildTables(db); err != nil {
		log.Fatalf("Error creating guild tables: %v", err)
	}

	if _, err := database.InitUserDB(); err != nil {
		log.Fatalf("Error setting up user database: %v", err)
	}

	if err := database.LoadConfigFromDB(db, cfg); err != nil {
		log.Fatalf("Error loading config from database: %v", err)
	}

	// 使用依赖注入容器构建服务
	serviceBuilder := container.NewServiceBuilder()
	serviceContainer, err := serviceBuilder.Build(cfg, db)
	if err != nil {
		log.Fatalf("Error building service container: %v", err)
	}

	// 从容器获取服务并创建Bot
	discordService, err := serviceContainer.Get("discord")
	if err != nil {
		log.Fatalf("Error getting discord service: %v", err)
	}

	commandService, err := serviceContainer.Get("command")
	if err != nil {
		log.Fatalf("Error getting command service: %v", err)
	}

	schedulerService, err := serviceContainer.Get("scheduler")
	if err != nil {
		log.Fatalf("Error getting scheduler service: %v", err)
	}

	cooldownService, err := serviceContainer.Get("cooldown")
	if err != nil {
		log.Fatalf("Error getting cooldown service: %v", err)
	}

	configServiceInterface, err := serviceContainer.Get("config_service")
	if err != nil {
		log.Fatalf("Error getting config service: %v", err)
	}

	configService, ok := configServiceInterface.(*internal_config.Service)
	if !ok {
		log.Fatalf("Error: config service type assertion failed")
	}

	// 获取Repository管理器
	repoManagerInterface, err := serviceContainer.Get("repository_manager")
	if err != nil {
		log.Fatalf("Error getting repository manager: %v", err)
	}

	repoManager, ok := repoManagerInterface.(repository.RepositoryManager)
	if !ok {
		log.Fatalf("Error: repository manager type assertion failed")
	}

	// 创建Bot实例
	b, err := bot.NewBot(
		discordService,
		commandService,
		schedulerService,
		cooldownService,
		cfg,
		db,
		configService,
		repoManager,
	)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	// 注册中间件处理器（新系统）
	handlers.RegisterMiddlewareHandlers(b)
	
	// 注册其他处理器（非交互处理器）
	handlers.RegisterNonInteractionHandlers(b)

	// 运行Bot
	b.Run()

	defer b.Close()
}
