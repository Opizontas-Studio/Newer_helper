package defs

import "github.com/bwmarrin/discordgo"

var minCount = 1.0

var Rollcard = &discordgo.ApplicationCommand{
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
}

var SetupRollPanel = &discordgo.ApplicationCommand{
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
}
