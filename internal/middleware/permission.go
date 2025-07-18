package middleware

import (
	"discord-bot/utils"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// permissionMiddleware 权限验证中间件实现
type permissionMiddleware struct {
	requirement PermissionRequirement
}

// NewPermissionMiddleware 创建权限验证中间件
func NewPermissionMiddleware(req PermissionRequirement) PermissionMiddleware {
	return &permissionMiddleware{
		requirement: req,
	}
}

// WithPermission 设置权限要求
func (m *permissionMiddleware) WithPermission(req PermissionRequirement) PermissionMiddleware {
	m.requirement = req
	return m
}

// Process 处理权限验证
func (m *permissionMiddleware) Process(ctx *CommandContext, next HandlerFunc) error {
	// 检查服务器配置
	serverConfig, ok := ctx.Config.ServerConfigs[ctx.GuildID]
	if !ok {
		log.Printf("未找到服务器配置，Guild ID: %s", ctx.GuildID)
		return m.sendPermissionError(ctx, "服务器配置未找到")
	}

	// 获取用户权限级别
	permissionLevel := utils.CheckPermission(
		ctx.Interaction.Member.Roles,
		ctx.UserID,
		serverConfig.AdminRoleIDs,
		serverConfig.UserRoleIDs,
		ctx.Config.DeveloperUserIDs,
		ctx.Config.SuperAdminRoleIDs,
	)

	// 检查自定义验证器
	if m.requirement.CustomValidator != nil {
		if !m.requirement.CustomValidator(ctx) {
			return m.sendPermissionError(ctx, "自定义权限验证失败")
		}
	}

	// 检查权限级别
	if !m.isPermissionSufficient(permissionLevel) {
		return m.sendPermissionError(ctx, "权限不足")
	}

	// 权限验证通过，继续执行
	next(ctx)
	return nil
}

// isPermissionSufficient 检查权限是否足够
func (m *permissionMiddleware) isPermissionSufficient(level string) bool {
	// 开发者权限可以访问所有命令
	if level == utils.DeveloperPermission {
		return true
	}

	// 超级管理员权限
	if level == utils.SuperAdminPermission && (m.requirement.RequiredLevel == utils.SuperAdminPermission || m.requirement.RequiredLevel == utils.AdminPermission || m.requirement.RequiredLevel == utils.UserPermission || m.requirement.RequiredLevel == utils.GuestPermission) {
		return true
	}

	// 管理员权限
	if level == utils.AdminPermission && (m.requirement.RequiredLevel == utils.AdminPermission || m.requirement.RequiredLevel == utils.UserPermission || m.requirement.RequiredLevel == utils.GuestPermission) {
		return true
	}

	// 用户权限
	if level == utils.UserPermission && (m.requirement.RequiredLevel == utils.UserPermission || m.requirement.RequiredLevel == utils.GuestPermission) {
		return true
	}

	// 访客权限
	if level == utils.GuestPermission && m.requirement.AllowGuests {
		return true
	}

	return false
}

// sendPermissionError 发送权限错误响应
func (m *permissionMiddleware) sendPermissionError(ctx *CommandContext, message string) error {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("❌ %s", message),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}

	return ctx.Session.InteractionRespond(ctx.Interaction.Interaction, response)
}

// 便利函数，用于快速创建常见的权限要求

// RequireAdmin 要求管理员权限
func RequireAdmin() PermissionMiddleware {
	return NewPermissionMiddleware(PermissionRequirement{
		RequiredLevel: utils.AdminPermission,
		AllowGuests:   false,
	})
}

// RequireUser 要求用户权限
func RequireUser() PermissionMiddleware {
	return NewPermissionMiddleware(PermissionRequirement{
		RequiredLevel: utils.UserPermission,
		AllowGuests:   false,
	})
}

// RequireSuperAdmin 要求超级管理员权限
func RequireSuperAdmin() PermissionMiddleware {
	return NewPermissionMiddleware(PermissionRequirement{
		RequiredLevel: utils.SuperAdminPermission,
		AllowGuests:   false,
	})
}

// RequireDeveloper 要求开发者权限
func RequireDeveloper() PermissionMiddleware {
	return NewPermissionMiddleware(PermissionRequirement{
		RequiredLevel: utils.DeveloperPermission,
		AllowGuests:   false,
	})
}

// AllowGuests 允许访客访问
func AllowGuests() PermissionMiddleware {
	return NewPermissionMiddleware(PermissionRequirement{
		RequiredLevel: utils.GuestPermission,
		AllowGuests:   true,
	})
}

// WithCustomValidator 添加自定义验证器
func WithCustomValidator(validator func(ctx *CommandContext) bool) PermissionMiddleware {
	return NewPermissionMiddleware(PermissionRequirement{
		RequiredLevel:   utils.GuestPermission,
		AllowGuests:     true,
		CustomValidator: validator,
	})
}