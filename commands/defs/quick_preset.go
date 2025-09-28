package defs

import "github.com/bwmarrin/discordgo"

var QuickPreset = &discordgo.ApplicationCommand{
	Name:        "quick-preset",
	Description: "Send, add, remove, or view your favorite presets.",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "快速预设",
		discordgo.ChineseTW: "快速預設",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "发送、添加、移除或查看您收藏的预设。",
		discordgo.ChineseTW: "發送、添加、移除或查看您收藏的預設。",
	},
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "slot",
			Description: "The preset slot ID (1, 2, or 3). Defaults to 1 if no action is specified.",
			NameLocalizations: map[discordgo.Locale]string{
				discordgo.ChineseCN: "预设槽位",
				discordgo.ChineseTW: "預設槽位",
			},
			DescriptionLocalizations: map[discordgo.Locale]string{
				discordgo.ChineseCN: "预设槽位 ID (1, 2, 或 3)。如果未指定操作，则默认为 1。",
				discordgo.ChineseTW: "預設槽位 ID (1, 2, 或 3)。如果未指定操作，則默認為 1。",
			},
			Required: false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "1", Value: 1},
				{Name: "2", Value: 2},
				{Name: "3", Value: 3},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "action",
			Description: "The action to perform.",
			NameLocalizations: map[discordgo.Locale]string{
				discordgo.ChineseCN: "操作",
				discordgo.ChineseTW: "操作",
			},
			DescriptionLocalizations: map[discordgo.Locale]string{
				discordgo.ChineseCN: "要执行的操作。",
				discordgo.ChineseTW: "要執行的操作。",
			},
			Required: false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "添加/替换", Value: "add"},
				{Name: "移除", Value: "remove"},
				{Name: "打印", Value: "show"},
			},
		},
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "preset_id",
			Description:  "The ID of the preset to add/replace. Required for 'add' action.",
			Autocomplete: true,
			NameLocalizations: map[discordgo.Locale]string{
				discordgo.ChineseCN: "预设id",
				discordgo.ChineseTW: "預設id",
			},
			DescriptionLocalizations: map[discordgo.Locale]string{
				discordgo.ChineseCN: "要添加/替换的预设的 ID。'添加' 操作需要此项。",
				discordgo.ChineseTW: "要添加/替換的預設的 ID。'添加' 操作需要此項。",
			},
			Required: false,
		},
	},
}

var QuickPresetReplyForAPP = &discordgo.ApplicationCommand{
	Name: "快速预设回复",
	Type: discordgo.MessageApplicationCommand,
}
