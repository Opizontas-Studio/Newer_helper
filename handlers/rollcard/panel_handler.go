package rollcard

import (
	"discord-bot/bot"
	"discord-bot/model"
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

	// 检查是否已存在面板，如果存在先删除旧的
	if existingPanel, exists := utils.GetPersistentPanel(i.GuildID, i.ChannelID); exists {
		if err := s.ChannelMessageDelete(i.ChannelID, existingPanel.MessageID); err != nil {
			log.Printf("Failed to delete existing panel message %s: %v", existingPanel.MessageID, err)
		}
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

	message, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})

	if err != nil {
		log.Printf("Error sending persistent roll panel: %v", err)
		utils.SendEphemeralResponse(s, i, "创建面板时发生错误 ")
		return
	}

	// 保存面板信息到JSON文件
	if err := utils.SavePersistentPanel(i.GuildID, i.ChannelID, message.ID, title, description, scope); err != nil {
		log.Printf("Error saving persistent panel info: %v", err)
		// 面板已创建，但保存失败，给用户提示
		utils.SendEphemeralResponse(s, i, "✅ 快速抽卡面板已成功创建！（但持久化保存失败，面板可能无法自动刷新）")
		return
	}

	utils.SendEphemeralResponse(s, i, "✅ 快速抽卡面板已成功创建！")
}

// HandlePersistentPanelRefresh 处理持久化面板的刷新
func HandlePersistentPanelRefresh(s *discordgo.Session, m *discordgo.MessageCreate, b *bot.Bot) {
	// 不处理私聊消息
	if m.GuildID == "" {
		return
	}

	// 检查该频道是否有持久化面板
	panelInfo, exists := utils.GetPersistentPanel(m.GuildID, m.ChannelID)
	if !exists {
		return
	}

	// 如果消息是bot自己发的，检查是否是面板消息
	if m.Author.ID == s.State.User.ID {
		// 检查embed标题是否匹配，如果匹配则跳过刷新（避免无限循环）
		if len(m.Embeds) > 0 && m.Embeds[0].Title == panelInfo.Title {
			return
		}
		// 如果是bot的其他消息，也需要刷新面板（让面板保持在底部）
	}
	refreshPersistentPanel(s, m.GuildID, m.ChannelID, panelInfo, b)
}

// refreshPersistentPanel 刷新持久化面板
func refreshPersistentPanel(s *discordgo.Session, guildID, channelID string, panelInfo *model.PersistentPanelInfo, b *bot.Bot) {
	// 删除旧的面板消息
	if err := s.ChannelMessageDelete(channelID, panelInfo.MessageID); err != nil {
		log.Printf("Failed to delete old panel message %s: %v", panelInfo.MessageID, err)
	}

	// 重新创建面板组件
	var components []discordgo.MessageComponent

	if panelInfo.Scope == "global" {
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
		// 检查服务器是否配置了抽卡功能
		if _, ok := b.GetConfig().RollCardConfigs[guildID]; !ok {
			log.Printf("Guild %s is not configured for rollcard, removing persistent panel", guildID)
			utils.DeletePersistentPanel(guildID, channelID)
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
		Title:       panelInfo.Title,
		Description: panelInfo.Description,
		Color:       0x5865F2, // Discord Blurple
	}

	// 发送新的面板消息
	message, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})

	if err != nil {
		log.Printf("Error refreshing persistent roll panel: %v", err)
		return
	}

	// 更新面板消息ID
	if err := utils.UpdatePanelMessageID(guildID, channelID, message.ID); err != nil {
		log.Printf("Error updating panel message ID: %v", err)
	}
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
