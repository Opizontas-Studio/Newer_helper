package defs

import "github.com/bwmarrin/discordgo"

var ManageAutoTrigger = &discordgo.ApplicationCommand{
	Name:        "manage-auto-trigger",
	Description: "Manage auto-triggers for keywords and presets.",
	NameLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "管理自动触发",
	},
	DescriptionLocalizations: &map[discordgo.Locale]string{
		discordgo.ChineseCN: "管理关键词和预设的自动触发",
	},
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "action",
			Description: "The action to perform.",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "绑定关键词 (Bind Keyword)", Value: "bind_keyword"},
				{Name: "解绑关键词 (Unbind Keyword)", Value: "unbind_keyword"},
				// {Name: "添加频道监听 (Add Channel)", Value: "add_channel"},
				// {Name: "移除频道监听 (Remove Channel)", Value: "remove_channel"},
				{Name: "列出配置 (List Config)", Value: "list_config"},
				{Name: "复写关键词 (Overwrite Keyword)", Value: "overwrite_keyword"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "keyword",
			Description: "The keyword to bind.",
			Required:    false,
		},
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "id",
			Description:  "The ID of the preset to trigger.",
			Required:     false,
			Autocomplete: true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionChannel,
			Name:        "channel",
			Description: "The channel to listen on.",
			Required:    false,
			ChannelTypes: []discordgo.ChannelType{
				discordgo.ChannelTypeGuildText,
			},
		},
	},
}
