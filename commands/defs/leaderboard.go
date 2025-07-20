package defs

import "github.com/bwmarrin/discordgo"

var NewCards = &discordgo.ApplicationCommand{
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
}

var AdsBoardAdmin = &discordgo.ApplicationCommand{
	Name:        "ads_board_admin",
	Description: "Manage leaderboard advertisements",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "广告板管理",
		discordgo.ChineseTW: "廣告板管理",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "管理排行榜广告",
		discordgo.ChineseTW: "管理排行榜廣告",
	},
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "action",
			Description: "Action to perform",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "添加广告 (add)", Value: "add"},
				{Name: "删除广告 (delete)", Value: "delete"},
				{Name: "列出广告 (list)", Value: "list"},
				{Name: "切换状态 (toggle)", Value: "toggle"},
				{Name: "修改广告 (modify)", Value: "modify"},
				{Name: "打印广告 (print)", Value: "print"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "input",
			Description: "Content for 'add' or 'modify'",
			Required:    false,
		},
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "ad_id",
			Description:  "Ad ID for 'delete'/'toggle'/'modify'/'print'",
			Autocomplete: true,
			Required:     false,
		},
	},
}
