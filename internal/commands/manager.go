package commands

import (
	"discord-bot/internal/middleware"
	"discord-bot/internal/services"
	"discord-bot/model"
	"log"

	"github.com/bwmarrin/discordgo"
)

// Manager 命令管理器
type Manager struct {
	middlewareFactory *middleware.Factory
	commands          map[string]*middleware.CommandHandler
	session           *discordgo.Session
	config            *model.Config
}

// NewManager 创建命令管理器
func NewManager(session *discordgo.Session, config *model.Config, cooldownService services.CooldownService) *Manager {
	logger := middleware.NewDefaultLogger()
	factory := middleware.NewFactory(logger, cooldownService)
	
	return &Manager{
		middlewareFactory: factory,
		commands:          make(map[string]*middleware.CommandHandler),
		session:           session,
		config:            config,
	}
}

// RegisterCommand 注册命令
func (m *Manager) RegisterCommand(name string, handler *middleware.CommandHandler) {
	m.commands[name] = handler
}

// RegisterAdminCommand 注册管理员命令
func (m *Manager) RegisterAdminCommand(name string, handler middleware.HandlerFunc) {
	cmdHandler := middleware.NewCommandBuilder(m.middlewareFactory).
		WithErrorHandling().
		WithLogging().
		WithPermission(middleware.RequireAdmin()).
		Build(handler)
	m.RegisterCommand(name, cmdHandler)
}

// RegisterSuperAdminCommand 注册超级管理员命令
func (m *Manager) RegisterSuperAdminCommand(name string, handler middleware.HandlerFunc) {
	cmdHandler := middleware.NewCommandBuilder(m.middlewareFactory).
		WithErrorHandling().
		WithLogging().
		WithPermission(middleware.RequireSuperAdmin()).
		Build(handler)
	m.RegisterCommand(name, cmdHandler)
}

// RegisterDeveloperCommand 注册开发者命令
func (m *Manager) RegisterDeveloperCommand(name string, handler middleware.HandlerFunc) {
	cmdHandler := middleware.NewCommandBuilder(m.middlewareFactory).
		WithErrorHandling().
		WithLogging().
		WithPermission(middleware.RequireDeveloper()).
		Build(handler)
	m.RegisterCommand(name, cmdHandler)
}

// RegisterUserCommand 注册用户命令
func (m *Manager) RegisterUserCommand(name string, handler middleware.HandlerFunc) {
	cmdHandler := middleware.NewCommandBuilder(m.middlewareFactory).
		WithErrorHandling().
		WithLogging().
		WithCooldown().
		WithPermission(middleware.RequireUser()).
		Build(handler)
	m.RegisterCommand(name, cmdHandler)
}

// RegisterGuestCommand 注册访客命令
func (m *Manager) RegisterGuestCommand(name string, handler middleware.HandlerFunc) {
	cmdHandler := middleware.NewCommandBuilder(m.middlewareFactory).
		WithErrorHandling().
		WithLogging().
		WithCooldown().
		WithPermission(middleware.AllowGuests()).
		Build(handler)
	m.RegisterCommand(name, cmdHandler)
}

// RegisterCustomCommand 注册自定义命令
func (m *Manager) RegisterCustomCommand(name string, builder *middleware.CommandBuilder, handler middleware.HandlerFunc) {
	cmdHandler := builder.Build(handler)
	m.RegisterCommand(name, cmdHandler)
}

// HandleInteraction 处理交互
func (m *Manager) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	commandName := i.ApplicationCommandData().Name
	
	handler, exists := m.commands[commandName]
	if !exists {
		log.Printf("未找到命令处理器: %s", commandName)
		return
	}

	handler.Handle(s, i, m.config)
}

// GetRegisteredCommands 获取已注册的命令
func (m *Manager) GetRegisteredCommands() []string {
	commands := make([]string, 0, len(m.commands))
	for name := range m.commands {
		commands = append(commands, name)
	}
	return commands
}

// GetMiddlewareFactory 获取中间件工厂
func (m *Manager) GetMiddlewareFactory() *middleware.Factory {
	return m.middlewareFactory
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *model.Config) {
	m.config = config
}