package middleware

import (
	"discord-bot/internal/services"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Factory 中间件工厂
type Factory struct {
	logger          Logger
	cooldownService services.CooldownService
}

// NewFactory 创建中间件工厂
func NewFactory(logger Logger, cooldownService services.CooldownService) *Factory {
	return &Factory{
		logger:          logger,
		cooldownService: cooldownService,
	}
}

// CreateStandardChain 创建标准中间件链
func (f *Factory) CreateStandardChain() *Chain {
	return NewChain(
		NewErrorHandlingMiddleware(f.logger),
		NewLoggingMiddleware(f.logger),
		NewCooldownMiddleware(f.cooldownService),
	)
}

// CreateAdminChain 创建管理员命令中间件链
func (f *Factory) CreateAdminChain() *Chain {
	return NewChain(
		NewErrorHandlingMiddleware(f.logger),
		NewLoggingMiddleware(f.logger),
		RequireAdmin(),
	)
}

// CreateSuperAdminChain 创建超级管理员命令中间件链
func (f *Factory) CreateSuperAdminChain() *Chain {
	return NewChain(
		NewErrorHandlingMiddleware(f.logger),
		NewLoggingMiddleware(f.logger),
		RequireSuperAdmin(),
	)
}

// CreateDeveloperChain 创建开发者命令中间件链
func (f *Factory) CreateDeveloperChain() *Chain {
	return NewChain(
		NewErrorHandlingMiddleware(f.logger),
		NewLoggingMiddleware(f.logger),
		RequireDeveloper(),
	)
}

// CreateUserChain 创建用户命令中间件链
func (f *Factory) CreateUserChain() *Chain {
	return NewChain(
		NewErrorHandlingMiddleware(f.logger),
		NewLoggingMiddleware(f.logger),
		NewCooldownMiddleware(f.cooldownService),
		RequireUser(),
	)
}

// CreateGuestChain 创建访客命令中间件链
func (f *Factory) CreateGuestChain() *Chain {
	return NewChain(
		NewErrorHandlingMiddleware(f.logger),
		NewLoggingMiddleware(f.logger),
		NewCooldownMiddleware(f.cooldownService),
		AllowGuests(),
	)
}

// CommandBuilder 命令构建器
type CommandBuilder struct {
	factory *Factory
	chain   *Chain
}

// NewCommandBuilder 创建命令构建器
func NewCommandBuilder(factory *Factory) *CommandBuilder {
	return &CommandBuilder{
		factory: factory,
		chain:   NewChain(),
	}
}

// WithErrorHandling 添加错误处理
func (b *CommandBuilder) WithErrorHandling() *CommandBuilder {
	b.chain.Use(NewErrorHandlingMiddleware(b.factory.logger))
	return b
}

// WithLogging 添加日志记录
func (b *CommandBuilder) WithLogging() *CommandBuilder {
	b.chain.Use(NewLoggingMiddleware(b.factory.logger))
	return b
}

// WithCooldown 添加冷却时间
func (b *CommandBuilder) WithCooldown() *CommandBuilder {
	b.chain.Use(NewCooldownMiddleware(b.factory.cooldownService))
	return b
}

// WithCustomCooldown 添加自定义冷却时间
func (b *CommandBuilder) WithCustomCooldown(duration time.Duration) *CommandBuilder {
	middleware := &CustomCooldownMiddleware{
		cooldownService: b.factory.cooldownService,
		duration:        duration,
	}
	b.chain.Use(middleware)
	return b
}

// WithPermission 添加权限检查
func (b *CommandBuilder) WithPermission(middleware PermissionMiddleware) *CommandBuilder {
	b.chain.Use(middleware)
	return b
}

// WithCustomMiddleware 添加自定义中间件
func (b *CommandBuilder) WithCustomMiddleware(middleware Middleware) *CommandBuilder {
	b.chain.Use(middleware)
	return b
}

// Build 构建命令处理器
func (b *CommandBuilder) Build(handler HandlerFunc) *CommandHandler {
	return &CommandHandler{
		chain:   b.chain,
		handler: handler,
	}
}

// CustomCooldownMiddleware 自定义冷却时间中间件
type CustomCooldownMiddleware struct {
	cooldownService services.CooldownService
	duration        time.Duration
}

// Process 处理自定义冷却检查
func (m *CustomCooldownMiddleware) Process(ctx *CommandContext, next HandlerFunc) error {
	commandName := ctx.Interaction.ApplicationCommandData().Name
	cooldownKey := fmt.Sprintf("%s:%s:%s", commandName, ctx.GuildID, ctx.UserID)

	// 检查是否在冷却期
	if m.cooldownService.IsOnCooldown(cooldownKey) {
		remaining := m.cooldownService.GetCooldownRemaining(cooldownKey)
		
		response := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("⏱️ 命令正在冷却中，请等待 %.1f 秒后再试", remaining.Seconds()),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}
		
		return ctx.Session.InteractionRespond(ctx.Interaction.Interaction, response)
	}

	// 设置自定义冷却时间
	m.cooldownService.SetCooldown(cooldownKey, m.duration)

	next(ctx)
	return nil
}

// PresetMiddleware 预设中间件配置
type PresetMiddleware struct {
	factory *Factory
}

// NewPresetMiddleware 创建预设中间件
func NewPresetMiddleware(factory *Factory) *PresetMiddleware {
	return &PresetMiddleware{factory: factory}
}

// Standard 标准中间件配置
func (p *PresetMiddleware) Standard() *CommandBuilder {
	return NewCommandBuilder(p.factory).
		WithErrorHandling().
		WithLogging().
		WithCooldown()
}

// Admin 管理员中间件配置
func (p *PresetMiddleware) Admin() *CommandBuilder {
	return NewCommandBuilder(p.factory).
		WithErrorHandling().
		WithLogging().
		WithPermission(RequireAdmin())
}

// SuperAdmin 超级管理员中间件配置
func (p *PresetMiddleware) SuperAdmin() *CommandBuilder {
	return NewCommandBuilder(p.factory).
		WithErrorHandling().
		WithLogging().
		WithPermission(RequireSuperAdmin())
}

// Developer 开发者中间件配置
func (p *PresetMiddleware) Developer() *CommandBuilder {
	return NewCommandBuilder(p.factory).
		WithErrorHandling().
		WithLogging().
		WithPermission(RequireDeveloper())
}

// User 用户中间件配置
func (p *PresetMiddleware) User() *CommandBuilder {
	return NewCommandBuilder(p.factory).
		WithErrorHandling().
		WithLogging().
		WithCooldown().
		WithPermission(RequireUser())
}

// Guest 访客中间件配置
func (p *PresetMiddleware) Guest() *CommandBuilder {
	return NewCommandBuilder(p.factory).
		WithErrorHandling().
		WithLogging().
		WithCooldown().
		WithPermission(AllowGuests())
}