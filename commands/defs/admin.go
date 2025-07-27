package defs

import "github.com/bwmarrin/discordgo"

var StartScan = &discordgo.ApplicationCommand{
	Name:        "start-scan",
	Description: "Manually start scanning",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "开始扫描",
		discordgo.ChineseTW: "開始掃描",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "手动启动扫描",
		discordgo.ChineseTW: "手動啟動掃描",
	},
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "mode",
			Description: "扫描模式 (默认为 active)",
			Required:    false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "活跃 (active)", Value: "active"},
				{Name: "全区 (full)", Value: "full"},
				{Name: "清理频道 (clean)", Value: "clean"},
			},
		},
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "guild",
			Description:  "要扫描的特定服务器 (默认为全部)",
			Required:     false,
			Autocomplete: true,
		},
	},
}

var SystemInfo = &discordgo.ApplicationCommand{
	Name:        "system-info",
	Description: "Display bot and system status information",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "系统信息",
		discordgo.ChineseTW: "系統信息",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "显示机器人和系统的状态信息",
		discordgo.ChineseTW: "顯示機器人和系統的狀態信息",
	},
}

var ReloadConfig = &discordgo.ApplicationCommand{
	Name:        "reload-config",
	Description: "Reload bot configuration file (developers only)",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "重载配置",
		discordgo.ChineseTW: "重載配置",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "重新加载机器人配置文件 (仅限开发者)",
		discordgo.ChineseTW: "重新加載機器人配置文件 (僅限開發者)",
	},
}

var NewPostPushAdmin = &discordgo.ApplicationCommand{
	Name:        "new-post-push_admin",
	Description: "Manage new post push configuration",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "推送管理",
		discordgo.ChineseTW: "推送管理",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "管理新帖子推送的配置",
		discordgo.ChineseTW: "管理新帖子推送的配置",
	},
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "action",
			Description: "要执行的操作",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "添加推送频道 (add_channel)", Value: "add_channel"},
				{Name: "移除推送频道 (remove_channel)", Value: "remove_channel"},
				{Name: "添加白名单消息 (add_whitelist)", Value: "add_whitelist"},
				{Name: "移除白名单消息 (remove_whitelist)", Value: "remove_whitelist"},
				{Name: "查看当前配置 (list_config)", Value: "list_config"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "input",
			Description: "输入值 (频道ID或消息ID)",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionChannel,
			Name:        "channel",
			Description: "目标频道 (用于白名单)",
			Required:    false,
			ChannelTypes: []discordgo.ChannelType{
				discordgo.ChannelTypeGuildText,
			},
		},
	},
}

var RegisterTopChannel = &discordgo.ApplicationCommand{
	Name:        "register-top-channel",
	Description: "Register a channel for auto-topping and cleaning.",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "注册回顶频道",
		discordgo.ChineseTW: "註冊回頂頻道",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "注册一个频道，用于自动回顶和消息清理",
		discordgo.ChineseTW: "註冊一個頻道，用於自動回頂和消息清理",
	},
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionChannel,
			Name:        "channel",
			Description: "要注册的频道",
			Required:    true,
			ChannelTypes: []discordgo.ChannelType{
				discordgo.ChannelTypeGuildText,
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "limit",
			Description: "频道内保留的最大消息数量",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "exclude-ids",
			Description: "要排除的消息ID，多个请用逗号隔开",
			Required:    false,
		},
	},
}

var GuildsAdmin = &discordgo.ApplicationCommand{
	Name:        "guilds_admin",
	Description: "Manage guild configurations",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "服务器管理",
		discordgo.ChineseTW: "伺服器管理",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "管理服务器配置",
		discordgo.ChineseTW: "管理伺服器配置",
	},
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "action",
			Description: "Action to perform",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "激活服务器 (activate)", Value: "activate"},
				{Name: "禁用服务器 (deactivate)", Value: "deactivate"},
				{Name: "新增服务器 (add_guild)", Value: "add_guild"},
				{Name: "新增管理员 (add_admin)", Value: "add_admin"},
				{Name: "新增用户 (add_user)", Value: "add_user"},
				{Name: "移除管理员 (remove_admin)", Value: "remove_admin"},
				{Name: "移除用户 (remove_user)", Value: "remove_user"},
				{Name: "列出配置 (list_config)", Value: "list_config"},
			},
		},
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "guild",
			Description:  "Target guild",
			Required:     true,
			Autocomplete: true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionRole,
			Name:        "role",
			Description: "Target role (for admin/user actions)",
			Required:    false,
		},
	},
}
