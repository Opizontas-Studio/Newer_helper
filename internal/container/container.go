package container

import (
	"database/sql"
	"discord-bot/internal/commands"
	"discord-bot/internal/services"
	"discord-bot/model"
	"fmt"
	"reflect"
	"sync"
)

// Container 是依赖注入容器
type Container struct {
	services map[string]interface{}
	mu       sync.RWMutex
}

// New 创建一个新的容器
func New() *Container {
	return &Container{
		services: make(map[string]interface{}),
	}
}

// Register 注册服务到容器
func (c *Container) Register(name string, service interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = service
}

// Get 从容器获取服务
func (c *Container) Get(name string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	service, exists := c.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}
	return service, nil
}

// GetTyped 获取指定类型的服务
func (c *Container) GetTyped(name string, target interface{}) error {
	service, err := c.Get(name)
	if err != nil {
		return err
	}
	
	// 使用反射将服务赋值给目标
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}
	
	serviceValue := reflect.ValueOf(service)
	if !serviceValue.Type().AssignableTo(targetValue.Elem().Type()) {
		return fmt.Errorf("service type %T not assignable to target type %T", service, target)
	}
	
	targetValue.Elem().Set(serviceValue)
	return nil
}

// ServiceBuilder 构建服务容器的构建器
type ServiceBuilder struct {
	container *Container
}

// NewServiceBuilder 创建服务构建器
func NewServiceBuilder() *ServiceBuilder {
	return &ServiceBuilder{
		container: New(),
	}
}

// Build 构建完整的服务容器
func (b *ServiceBuilder) Build(cfg *model.Config, db *sql.DB) (*Container, error) {
	// 注册核心依赖
	b.container.Register("config", cfg)
	b.container.Register("database", db)
	
	// 创建并注册Discord服务
	discordService, err := services.NewDiscordService(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord service: %w", err)
	}
	b.container.Register("discord", discordService)
	
	// 创建并注册冷却服务
	cooldownService := services.NewCooldownService()
	b.container.Register("cooldown", cooldownService)
	
	// 创建并注册调度服务
	schedulerService := services.NewSchedulerService()
	b.container.Register("scheduler", schedulerService)
	
	// 创建并注册命令服务
	commandService, err := services.NewCommandService(discordService, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create command service: %w", err)
	}
	b.container.Register("command", commandService)
	
	// 创建并注册命令管理器
	commandManager := commands.NewManager(discordService.GetSession(), cfg, cooldownService)
	b.container.Register("command_manager", commandManager)
	
	return b.container, nil
}

// GetContainer 获取容器实例
func (b *ServiceBuilder) GetContainer() *Container {
	return b.container
}