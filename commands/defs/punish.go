package defs

import "github.com/bwmarrin/discordgo"

var Punish = &discordgo.ApplicationCommand{
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
}

var PunishAdmin = &discordgo.ApplicationCommand{
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
				{Name: "禁言数据库ID", Value: "mute_db_id"},
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
			Description: "要执行的操作 (仅限按 处罚ID/禁言数据库ID 搜索时)",
			Required:    false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "撤销处罚", Value: "revoke"},
				{Name: "删除记录", Value: "delete"},
				{Name: "打印证据", Value: "print_evidence"},
			},
		},
	},
}

var QuickPunish = &discordgo.ApplicationCommand{
	Name: "快速处罚",
	Type: discordgo.MessageApplicationCommand,
}

var DailyPunishmentStats = &discordgo.ApplicationCommand{
	Name:        "daily_punishment_stats",
	Description: "Manage and display daily punishment statistics",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "每日处罚数",
		discordgo.ChineseTW: "每日處罰數",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "管理并显示每日处罚数和管理员击杀榜单",
		discordgo.ChineseTW: "管理並顯示每日處罰數和管理員擊殺榜單",
	},
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "action",
			Description: "Action to perform",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "注册频道 (register)", Value: "register"},
				{Name: "删除频道 (delete)", Value: "delete"},
				{Name: "设置服务器 (set_server)", Value: "set_server"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "input",
			Description: "输入ID (频道ID或服务器ID)",
			Required:    false,
		},
	},
}
