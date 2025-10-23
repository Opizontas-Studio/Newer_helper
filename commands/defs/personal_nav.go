package defs

import "github.com/bwmarrin/discordgo"

var PersonalNavigation = &discordgo.ApplicationCommand{
	Name:        "personal-nav",
	Description: "Manage your personal navigation posts",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "个人导航",
		discordgo.ChineseTW: "個人導航",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "管理你的个人作品导航",
		discordgo.ChineseTW: "管理你的個人作品導航",
	},
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "action",
			Description: "Action to perform",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{
					Name:  "创建导航 (create)",
					Value: "create",
				},
				{
					Name:  "删除导航 (delete)",
					Value: "delete",
				},
				{
					Name:  "刷新导航 (refresh)",
					Value: "refresh",
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "input",
			Description: "Optional message ID to operate on",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "userid",
			Description: "Developer only: specify a user ID to act on their behalf",
			Required:    false,
		},
	},
}
