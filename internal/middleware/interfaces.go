package middleware

import (
	"discord-bot/model"
	"discord-bot/utils"

	"github.com/bwmarrin/discordgo"
)

// CommandContext 命令执行上下文
type CommandContext struct {
	Session     *discordgo.Session
	Interaction *discordgo.InteractionCreate
	GuildID     string
	UserID      string
	Config      *model.Config
}

// HandlerFunc 命令处理函数类型
type HandlerFunc func(ctx *CommandContext)

// Middleware 中间件接口
type Middleware interface {
	// Process 处理中间件逻辑
	Process(ctx *CommandContext, next HandlerFunc) error
}

// MiddlewareFunc 中间件函数类型
type MiddlewareFunc func(ctx *CommandContext, next HandlerFunc) error

// Process 实现Middleware接口
func (f MiddlewareFunc) Process(ctx *CommandContext, next HandlerFunc) error {
	return f(ctx, next)
}

// PermissionRequirement 权限要求配置
type PermissionRequirement struct {
	RequiredLevel   string
	AllowGuests     bool
	CustomValidator func(ctx *CommandContext) bool
}

// PermissionMiddleware 权限验证中间件接口
type PermissionMiddleware interface {
	Middleware
	// WithPermission 设置权限要求
	WithPermission(req PermissionRequirement) PermissionMiddleware
}

// LoggingMiddleware 日志中间件接口
type LoggingMiddleware interface {
	Middleware
	// WithLogger 设置日志记录器
	WithLogger(logger Logger) LoggingMiddleware
}

// Logger 日志接口
type Logger interface {
	Info(msg string, fields ...map[string]interface{})
	Error(msg string, err error, fields ...map[string]interface{})
	Debug(msg string, fields ...map[string]interface{})
}

// Chain 中间件链
type Chain struct {
	middlewares []Middleware
}

// NewChain 创建新的中间件链
func NewChain(middlewares ...Middleware) *Chain {
	return &Chain{
		middlewares: middlewares,
	}
}

// Use 添加中间件到链
func (c *Chain) Use(middleware Middleware) *Chain {
	c.middlewares = append(c.middlewares, middleware)
	return c
}

// Execute 执行中间件链
func (c *Chain) Execute(ctx *CommandContext, handler HandlerFunc) error {
	if len(c.middlewares) == 0 {
		handler(ctx)
		return nil
	}

	return c.executeMiddleware(0, ctx, handler)
}

// executeMiddleware 递归执行中间件
func (c *Chain) executeMiddleware(index int, ctx *CommandContext, handler HandlerFunc) error {
	if index >= len(c.middlewares) {
		handler(ctx)
		return nil
	}

	return c.middlewares[index].Process(ctx, func(ctx *CommandContext) {
		c.executeMiddleware(index+1, ctx, handler)
	})
}

// CommandHandler 包装了中间件的命令处理器
type CommandHandler struct {
	chain   *Chain
	handler HandlerFunc
}

// NewCommandHandler 创建新的命令处理器
func NewCommandHandler(handler HandlerFunc, middlewares ...Middleware) *CommandHandler {
	return &CommandHandler{
		chain:   NewChain(middlewares...),
		handler: handler,
	}
}

// Handle 处理命令
func (h *CommandHandler) Handle(s *discordgo.Session, i *discordgo.InteractionCreate, config *model.Config) {
	ctx := &CommandContext{
		Session:     s,
		Interaction: i,
		GuildID:     i.GuildID,
		UserID:      i.Member.User.ID,
		Config:      config,
	}

	if err := h.chain.Execute(ctx, h.handler); err != nil {
		// 处理错误，发送错误响应
		utils.SendErrorResponse(s, i, "命令执行失败")
	}
}
