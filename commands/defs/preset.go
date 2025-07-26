package defs

import "github.com/bwmarrin/discordgo"

var PresetMessage = &discordgo.ApplicationCommand{
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
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "message_link",
			Description: "要回复的消息链接",
			Required:    false,
		},
	},
}

var PresetMessageUpd = &discordgo.ApplicationCommand{
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
}

var PresetMessageAdmin = &discordgo.ApplicationCommand{
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
}

var SearchPresetByMessage = &discordgo.ApplicationCommand{
	Name: "以消息文本搜索预设",
	Type: discordgo.MessageApplicationCommand,
}
