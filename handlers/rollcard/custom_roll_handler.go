package rollcard

import (
	"discord-bot/bot"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// HandleCustomRoll is the entry point for "单抽" and "五连抽" buttons.
func HandleCustomRoll(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, customID string) {
	parts := strings.Split(customID, ":")
	if len(parts) != 2 {
		sendEphemeralResponse(s, i, "无效的按钮ID ")
		return
	}
	count, err := strconv.Atoi(parts[1])
	if err != nil {
		sendEphemeralResponse(s, i, "无效的抽卡数量 ")
		return
	}

	userID := i.Member.User.ID
	guildID := i.GuildID
	preferredPools, err := database.GetUserPreferredPools(userID, guildID)
	if err != nil {
		log.Printf("Error getting user preferred pools for %s in guild %s: %v", userID, guildID, err)
		sendEphemeralResponse(s, i, "获取用户偏好时出错。")
		return
	}

	if len(preferredPools) == 0 {
		// No preferences set, show the selection menu.
		SendPoolSelectionMenu(s, i, b)
		return
	}

	// User has preferences, roll with them.
	rollCard(s, i, b, preferredPools, "", count, []string{})
}

// HandleEditPools shows the pool selection menu to the user.
func HandleEditPools(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	SendPoolSelectionMenu(s, i, b)
}

// SendPoolSelectionMenu sends a message with a multi-select menu for the user to choose their preferred pools.
func SendPoolSelectionMenu(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	guildID := i.GuildID
	rollCardGuildConfig, ok := b.Config.RollCardConfigs[guildID]
	if !ok {
		sendEphemeralResponse(s, i, "此服务器未配置抽卡功能 ")
		return
	}

	var options []discordgo.SelectMenuOption
	for _, poolName := range rollCardGuildConfig.DataBaseTableNameMapping {
		options = append(options, discordgo.SelectMenuOption{
			Label: poolName,
			Value: poolName,
		})
	}

	if len(options) == 0 {
		sendEphemeralResponse(s, i, "没有可用的卡池")
		return
	}

	// Check for existing preferences to set as default
	currentPrefs, err := database.GetUserPreferredPools(i.Member.User.ID, i.GuildID)
	if err != nil {
		log.Printf("Error getting user preferred pools for default values in guild %s: %v", i.GuildID, err)
		// Continue without defaults
	}

	for i, opt := range options {
		for _, pref := range currentPrefs {
			if opt.Value == pref {
				options[i].Default = true
				break
			}
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:       "自定义您的抽卡体验",
		Description: "请从下面的菜单中选择您偏好的一个或多个抽卡池 \n您的选择将被保存，并用于“我的卡池”抽卡 ",
		Color:       0x7289DA, // Discord Blurple
		Footer: &discordgo.MessageEmbedFooter{
			Text: "您的设置将被自动保存",
		},
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							CustomID:    "select_pools_menu",
							Placeholder: "点击选择卡池",
							MinValues:   &[]int{1}[0], // Pointer to int 1
							MaxValues:   len(options),
							Options:     options,
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Error sending pool selection menu: %v", err)
	}
}

// HandlePoolSelectionResponse handles the user's response from the pool selection menu.
func HandlePoolSelectionResponse(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	data := i.MessageComponentData()
	userID := i.Member.User.ID
	guildID := i.GuildID
	selectedPools := data.Values

	if err := database.SetUserPreferredPools(userID, guildID, selectedPools); err != nil {
		log.Printf("Error setting user preferred pools for %s in guild %s: %v", userID, guildID, err)
		sendEphemeralResponse(s, i, "保存您的偏好时发生错误。")
		return
	}

	// Respond to the interaction to confirm the selection has been saved.
	// We update the original message.
	embed := &discordgo.MessageEmbed{
		Title:       "✅ 偏好已保存",
		Description: fmt.Sprintf("您新的偏好卡池为: **%s** \n\n现在您可以直接使用“单抽”或“五连抽”按钮了！", strings.Join(selectedPools, ", ")),
		Color:       0x57F287, // Green
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{}, // Remove the select menu
		},
	})
	if err != nil {
		log.Printf("Error updating message after pool selection: %v", err)
	}
}
