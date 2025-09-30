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
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "action",
			Description:  "处罚类型 (默认: re-answer)",
			Required:     false,
			Autocomplete: true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "处罚原因 (留空使用默认原因)",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "message_links",
			Description: "作为证据的消息链接，多个链接请用空格分开",
			Required:    false,
		},
	},
}

var (
	PunishSearch = &discordgo.ApplicationCommand{
		Name:        "punish_search",
		Description: "搜索处罚记录",
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.ChineseCN: "搜索处罚",
			discordgo.ChineseTW: "搜索處罰",
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.ChineseCN: "根据不同标准搜索处罚记录",
			discordgo.ChineseTW: "根據不同標準搜索處罰記錄",
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
		},
	}

	PunishRevoke = &discordgo.ApplicationCommand{
		Name:        "punish_revoke",
		Description: "撤销一个处罚",
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.ChineseCN: "撤销处罚",
			discordgo.ChineseTW: "撤銷處罰",
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.ChineseCN: "通过处罚ID撤销一个处罚",
			discordgo.ChineseTW: "通過處罰ID撤銷一個處罰",
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "punishment_id",
				Description: "要撤销的处罚ID",
				Required:    true,
			},
		},
	}

	PunishDelete = &discordgo.ApplicationCommand{
		Name:        "punish_delete",
		Description: "删除一个处罚记录",
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.ChineseCN: "删除处罚",
			discordgo.ChineseTW: "刪除處罰",
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.ChineseCN: "通过处罚ID删除一个处罚记录",
			discordgo.ChineseTW: "通過處罰ID刪除一個處罰記錄",
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "punishment_id",
				Description: "要删除的处罚ID",
				Required:    true,
			},
		},
	}

	PunishPrintEvidence = &discordgo.ApplicationCommand{
		Name:        "punish_print_evidence",
		Description: "打印一个处罚的证据",
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.ChineseCN: "打印证据",
			discordgo.ChineseTW: "打印證據",
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.ChineseCN: "通过处罚ID打印一个处罚的证据",
			discordgo.ChineseTW: "通過處罰ID打印一個處罰的證據",
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "punishment_id",
				Description: "要打印证据的处罚ID",
				Required:    true,
			},
		},
	}
)

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
				{Name: "立即刷新 (refresh)", Value: "refresh"},
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

var ResetPunishCooldown = &discordgo.ApplicationCommand{
	Name:        "reset_punish_cooldown",
	Description: "Reset all punishment cooldowns",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "重置处罚冷却",
		discordgo.ChineseTW: "重置處罰冷卻",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "重置所有用户的处罚冷却时间",
		discordgo.ChineseTW: "重置所有用戶的處罰冷卻時間",
	},
}
