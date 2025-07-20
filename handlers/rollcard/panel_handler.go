package rollcard

import (
	"discord-bot/bot"
	"discord-bot/utils"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func HandleSetupRollPanel(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	title := "快速抽卡面板"
	if opt, ok := optionMap["title"]; ok {
		title = opt.StringValue()
	}

	description := "点击下方按钮，快速抽取你的卡片！"
	if opt, ok := optionMap["description"]; ok {
		description = opt.StringValue()
	}

	scope := "server"
	if opt, ok := optionMap["scope"]; ok {
		scope = opt.StringValue()
	}

	var components []discordgo.MessageComponent

	if scope == "global" {
		components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "全局单抽",
						Style:    discordgo.PrimaryButton,
						CustomID: "global_roll:1",
					},
					discordgo.Button{
						Label:    "全局五连抽",
						Style:    discordgo.PrimaryButton,
						CustomID: "global_roll:5",
					},
				},
			},
		}
	} else {
		guildID := i.GuildID
		if _, ok := b.GetConfig().RollCardConfigs[guildID]; !ok {
			utils.SendEphemeralResponse(s, i, "This server is not configured for rollcard.")
			return
		}
		components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "单抽 (我的卡池)",
						Style:    discordgo.PrimaryButton,
						CustomID: "custom_roll:1",
					},
					discordgo.Button{
						Label:    "五连抽 (我的卡池)",
						Style:    discordgo.PrimaryButton,
						CustomID: "custom_roll:5",
					},
					discordgo.Button{
						Label:    "修改我的卡池",
						Style:    discordgo.SecondaryButton,
						CustomID: "edit_my_pools",
					},
					discordgo.Button{
						Label:    "全区抽卡",
						Style:    discordgo.SecondaryButton,
						CustomID: "persistent_roll:all-server-roll",
					},
				},
			},
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       0x5865F2, // Discord Blurple
	}

	_, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})

	if err != nil {
		log.Printf("Error sending persistent roll panel: %v", err)
		utils.SendEphemeralResponse(s, i, "创建面板时发生错误 ")
		return
	}

	utils.SendEphemeralResponse(s, i, "✅ 快速抽卡面板已成功创建！")
}

func HandlePersistentRoll(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, customID string) {
	parts := strings.SplitN(customID, ":", 2)
	if len(parts) != 2 {
		log.Printf("Invalid customID format for persistent roll: %s", customID)
		return
	}
	poolName := parts[1]
	rollCard(s, i, b, []string{poolName}, "", 1, []string{})
}
