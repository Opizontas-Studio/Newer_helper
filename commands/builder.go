package commands

import (
	"discord-bot/model"

	"github.com/bwmarrin/discordgo"
)

func GenerateCommands(_ *model.ServerConfig) []*discordgo.ApplicationCommand {

	return []*discordgo.ApplicationCommand{
		{
			Name:        "punish",
			Description: "Remove user roles and record punishment",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "答题处罚",
				discordgo.ChineseTW: "答題處罰",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "移除用户的身份组并记录处罚",
				discordgo.ChineseTW: "移除用戶的身份組並記錄處罰",
			},
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "要处罚的用户",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "处罚原因",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "message_links",
					Description: "作为证据的消息链接，多个链接请用空格分开",
					Required:    false,
				},
			},
		},
		{
			Name:        "preset-message",
			Description: "Send preset message and mention a member",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "预设",
				discordgo.ChineseTW: "預設",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "发送预设消息并提及一位成员",
				discordgo.ChineseTW: "發送預設消息並提及一位成員",
			},
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "id",
					Description:  "要发送的预设消息 ID ",
					Required:     true,
					Autocomplete: true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "要提及的用户 ",
					Required:    false,
				},
			},
		},
		{
			Name:        "preset-message_upd",
			Description: "Parse and create preset from message links",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "更新预设",
				discordgo.ChineseTW: "更新預設",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "从消息链接中解析和创建预设",
				discordgo.ChineseTW: "從消息鏈接中解析和創建預設",
			},
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "messagelinks",
					Description: "要解析的消息链接 ",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "为预设指定一个自定义名称 ",
					Required:    true,
				},
			},
		},
		{
			Name:        "preset-message_admin",
			Description: "Manage preset messages",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "管理预设",
				discordgo.ChineseTW: "管理預設",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "管理预设消息",
				discordgo.ChineseTW: "管理預設消息",
			},
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "id",
					Description:  "要管理的预设 ID",
					Required:     true,
					Autocomplete: true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "要执行的操作",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "重命名", Value: "rename"},
						{Name: "删除", Value: "del"},
						{Name: "覆盖", Value: "overwrite"},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "input",
					Description: "重命名或覆盖的新内容",
					Required:    false,
				},
			},
		},
		{
			Name:        "rollcard",
			Description: "Draw cards from specified card pool",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "抽卡",
				discordgo.ChineseTW: "抽卡",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "从指定卡池中抽取卡片",
				discordgo.ChineseTW: "從指定卡池中抽取卡片",
			},
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "pool",
					Description:  "要抽卡的卡池",
					Autocomplete: true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "count",
					Description: "要抽取的数量 (1-8，默认为 1)",
					Required:    false,
					MinValue:    &minCount,
					MaxValue:    8,
				},
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "tag",
					Description:  "使用标签进行抽卡",
					Required:     false,
					Autocomplete: true,
				},
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "exclude_tag1",
					Description:  "排除不想看到的tag",
					Required:     false,
					Autocomplete: true,
				},
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "exclude_tag2",
					Description:  "排除不想看到的tag",
					Required:     false,
					Autocomplete: true,
				},
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "exclude_tag3",
					Description:  "排除不想看到的tag",
					Required:     false,
					Autocomplete: true,
				},
			},
		},
		{
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
		},
		{
			Name:        "new-cards",
			Description: "Display latest card leaderboard",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "最新卡片",
				discordgo.ChineseTW: "最新卡片",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "显示最新的卡片榜单",
				discordgo.ChineseTW: "顯示最新的卡片榜單",
			},
		},
		{
			Name:        "setup-roll-panel",
			Description: "Create or update a quick roll panel visible to everyone",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "设置抽卡面板",
				discordgo.ChineseTW: "設置抽卡面板",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "创建或更新一个对所有人可见的快速抽卡面板",
				discordgo.ChineseTW: "創建或更新一個對所有人可見的快速抽卡面板",
			},
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "title",
					Description: "面板的标题",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "description",
					Description: "面板的描述信息",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "scope",
					Description: "面板的作用域 (默认为当前服务器)",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "当前服务器 (server)", Value: "server"},
						{Name: "全局 (global)", Value: "global"},
					},
				},
			},
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
			Name:        "punish_admin",
			Description: "Manage punishment records",
			NameLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "处罚管理",
				discordgo.ChineseTW: "處罰管理",
			},
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.ChineseCN: "管理处罚记录",
				discordgo.ChineseTW: "管理處罰記錄",
			},
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "search_by",
					Description: "选择搜索方式",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "处罚ID", Value: "punishment_id"},
						{Name: "被处罚者ID", Value: "punished_user_id"},
						{Name: "处罚者ID", Value: "punisher_id"},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "input",
					Description: "输入要搜索的ID",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "要执行的操作 (仅限按处罚ID搜索时)",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "撤销处罚", Value: "revoke"},
						{Name: "删除记录", Value: "delete"},
					},
				},
			},
		},
	}
}

var minCount = 1.0
