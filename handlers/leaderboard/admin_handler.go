package leaderboard

import (
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type IBot interface {
	GetConfig() *model.Config
}

func HandleAdsBoardAdminCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b IBot) {
	// Defer the response
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Error sending deferred response: %v", err)
		return
	}

	// Run the logic in a goroutine
	go func() {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		action := optionMap["action"].StringValue()
		var input string
		if opt, ok := optionMap["input"]; ok {
			input = opt.StringValue()
		}

		db, err := database.InitDB("data/guilds.db")
		if err != nil {
			log.Printf("Failed to connect to database: %v", err)
			utils.EditErrorResponse(s, i, "数据库连接失败")
			return
		}
		defer db.Close()

		switch action {
		case "add":
			if input == "" {
				utils.EditErrorResponse(s, i, "请输入包含广告内容的消息链接")
				return
			}

			parsedMessages, err := utils.ParseMessageLinks(s, input)
			if err != nil {
				utils.EditErrorResponse(s, i, "解析消息链接失败: "+err.Error())
				return
			}
			if len(parsedMessages) == 0 {
				utils.EditErrorResponse(s, i, "没有找到有效的消息链接")
				return
			}

			// We only use the first message found
			msg := parsedMessages[0]
			var adContent string
			var imageURL string

			// Check for attachments first
			if len(msg.Attachments) > 0 {
				imageURL = msg.Attachments[0].URL
			}

			if len(msg.Embeds) > 0 {
				// If there are embeds, serialize the first one to JSON
				embedData, err := json.Marshal(msg.Embeds[0])
				if err != nil {
					log.Printf("Failed to marshal embed: %v", err)
					utils.EditErrorResponse(s, i, "序列化 Embed 内容失败")
					return
				}
				adContent = string(embedData)
			} else {
				// Otherwise, use the plain text content
				adContent = msg.Content
			}

			if adContent == "" && imageURL == "" {
				utils.EditErrorResponse(s, i, "消息内容和图片均为空，无法添加为广告")
				return
			}

			err = database.AddLeaderboardAd(db, i.GuildID, adContent, imageURL)
			if err != nil {
				log.Printf("Failed to add leaderboard ad: %v", err)
				utils.EditErrorResponse(s, i, "添加广告失败")
				return
			}
			content := "广告已成功添加！"
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &content,
			})

		case "delete":
			if input == "" {
				utils.EditErrorResponse(s, i, "请输入要删除的广告 ID")
				return
			}
			adID, err := strconv.Atoi(input)
			if err != nil {
				utils.EditErrorResponse(s, i, "无效的广告 ID")
				return
			}
			err = database.DeleteLeaderboardAd(db, adID, i.GuildID)
			if err != nil {
				log.Printf("Failed to delete leaderboard ad: %v", err)
				utils.EditErrorResponse(s, i, "删除广告失败")
				return
			}
			content := fmt.Sprintf("广告 #%d 已成功删除！", adID)
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &content,
			})

		case "list":
			ads, err := database.ListLeaderboardAds(db, i.GuildID)
			if err != nil {
				log.Printf("Failed to list leaderboard ads: %v", err)
				utils.EditErrorResponse(s, i, "获取广告列表失败")
				return
			}
			if len(ads) == 0 {
				content := "当前没有广告。"
				s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &content,
				})
				return
			}

			var builder strings.Builder
			builder.WriteString("以下是当前的广告列表：\n")
			for _, ad := range ads {
				status := "✅ 已启用"
				if !ad.Enabled {
					status = "❌ 已禁用"
				}
				// Truncate content for display
				displayContent := ad.Content
				if len(displayContent) > 50 {
					displayContent = displayContent[:50] + "..."
				}
				builder.WriteString(fmt.Sprintf("`%d`: %s (%s)\n", ad.ID, displayContent, status))
			}
			content := builder.String()
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &content,
			})

		case "toggle":
			if input == "" {
				utils.EditErrorResponse(s, i, "请输入要切换状态的广告 ID")
				return
			}
			adID, err := strconv.Atoi(input)
			if err != nil {
				utils.EditErrorResponse(s, i, "无效的广告 ID")
				return
			}

			ads, err := database.ListLeaderboardAds(db, i.GuildID)
			if err != nil {
				utils.EditErrorResponse(s, i, "无法获取广告信息")
				return
			}

			var currentAd *model.LeaderboardAd
			for i := range ads {
				if ads[i].ID == adID {
					currentAd = &ads[i]
					break
				}
			}

			if currentAd == nil {
				utils.EditErrorResponse(s, i, "找不到指定的广告 ID")
				return
			}

			err = database.ToggleLeaderboardAd(db, adID, i.GuildID, !currentAd.Enabled)
			if err != nil {
				log.Printf("Failed to toggle leaderboard ad: %v", err)
				utils.EditErrorResponse(s, i, "切换广告状态失败")
				return
			}

			newState := "启用"
			if currentAd.Enabled { // Note: we check the state *before* toggling
				newState = "禁用"
			}
			content := fmt.Sprintf("广告 #%d 的状态已成功切换为 **%s**！", adID, newState)
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &content,
			})
		}
	}()
}
