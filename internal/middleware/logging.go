package middleware

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

// loggingMiddleware 日志中间件实现
type loggingMiddleware struct {
	logger Logger
}

// NewLoggingMiddleware 创建日志中间件
func NewLoggingMiddleware(logger Logger) LoggingMiddleware {
	return &loggingMiddleware{
		logger: logger,
	}
}

// WithLogger 设置日志记录器
func (m *loggingMiddleware) WithLogger(logger Logger) LoggingMiddleware {
	m.logger = logger
	return m
}

// Process 处理日志记录
func (m *loggingMiddleware) Process(ctx *CommandContext, next HandlerFunc) error {
	start := time.Now()

	// 记录命令开始执行
	m.logger.Info("命令开始执行", map[string]interface{}{
		"command":    ctx.Interaction.ApplicationCommandData().Name,
		"guild_id":   ctx.GuildID,
		"user_id":    ctx.UserID,
		"start_time": start,
	})

	// 执行下一个中间件或处理器
	next(ctx)

	// 记录命令执行完成
	duration := time.Since(start)
	m.logger.Info("命令执行完成", map[string]interface{}{
		"command":  ctx.Interaction.ApplicationCommandData().Name,
		"guild_id": ctx.GuildID,
		"user_id":  ctx.UserID,
		"duration": duration,
	})

	return nil
}

// defaultLogger 默认日志实现
type defaultLogger struct{}

// NewDefaultLogger 创建默认日志记录器
func NewDefaultLogger() Logger {
	return &defaultLogger{}
}

// Info 记录信息日志
func (l *defaultLogger) Info(msg string, fields ...map[string]interface{}) {
	logMsg := fmt.Sprintf("[INFO] %s", msg)
	if len(fields) > 0 {
		logMsg += fmt.Sprintf(" %+v", fields[0])
	}
	log.Println(logMsg)
}

// Error 记录错误日志
func (l *defaultLogger) Error(msg string, err error, fields ...map[string]interface{}) {
	logMsg := fmt.Sprintf("[ERROR] %s: %v", msg, err)
	if len(fields) > 0 {
		logMsg += fmt.Sprintf(" %+v", fields[0])
	}
	log.Println(logMsg)
}

// Debug 记录调试日志
func (l *defaultLogger) Debug(msg string, fields ...map[string]interface{}) {
	logMsg := fmt.Sprintf("[DEBUG] %s", msg)
	if len(fields) > 0 {
		logMsg += fmt.Sprintf(" %+v", fields[0])
	}
	log.Println(logMsg)
}

// ErrorHandlingMiddleware 错误处理中间件
type ErrorHandlingMiddleware struct {
	logger Logger
}

// NewErrorHandlingMiddleware 创建错误处理中间件
func NewErrorHandlingMiddleware(logger Logger) *ErrorHandlingMiddleware {
	return &ErrorHandlingMiddleware{
		logger: logger,
	}
}

// Process 处理错误
func (m *ErrorHandlingMiddleware) Process(ctx *CommandContext, next HandlerFunc) error {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("命令执行发生 panic", fmt.Errorf("%v", r), map[string]interface{}{
				"command":  ctx.Interaction.ApplicationCommandData().Name,
				"guild_id": ctx.GuildID,
				"user_id":  ctx.UserID,
			})

			// 发送错误响应
			ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "⚠️ 命令执行时发生错误，请稍后重试",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}
	}()

	next(ctx)
	return nil
}

// CooldownMiddleware 冷却时间中间件
type CooldownMiddleware struct {
	cooldownService CooldownService
}

// CooldownService 冷却服务接口
type CooldownService interface {
	SetCooldown(key string, duration time.Duration)
	IsOnCooldown(key string) bool
	GetCooldownRemaining(key string) time.Duration
}

// NewCooldownMiddleware 创建冷却时间中间件
func NewCooldownMiddleware(cooldownService CooldownService) *CooldownMiddleware {
	return &CooldownMiddleware{
		cooldownService: cooldownService,
	}
}

// Process 处理冷却检查
func (m *CooldownMiddleware) Process(ctx *CommandContext, next HandlerFunc) error {
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

	// 设置冷却时间（根据命令类型设置不同的冷却时间）
	cooldownDuration := m.getCooldownDuration(commandName)
	if cooldownDuration > 0 {
		m.cooldownService.SetCooldown(cooldownKey, cooldownDuration)
	}

	next(ctx)
	return nil
}

// getCooldownDuration 获取命令的冷却时间
func (m *CooldownMiddleware) getCooldownDuration(commandName string) time.Duration {
	cooldownMap := map[string]time.Duration{
		"rollcard":       3 * time.Second,
		"punish":         5 * time.Second,
		"preset-message": 2 * time.Second,
		"start-scan":     30 * time.Second,
	}

	if duration, exists := cooldownMap[commandName]; exists {
		return duration
	}

	return 1 * time.Second // 默认冷却时间
}
