package lbadmin

import (
	"database/sql"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

func HandlePrintAd(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB, b model.BotConfigProvider, adIDStr string) {
	if adIDStr == "" {
		utils.SendFollowUpError(s, i.Interaction, "请输入要打印的广告 ID")
		return
	}
	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "无效的广告 ID")
		return
	}

	ads, err := database.ListLeaderboardAds(db, i.GuildID)
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "无法获取广告信息")
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
		utils.SendFollowUpError(s, i.Interaction, "找不到指定的广告 ID")
		return
	}

	var embed discordgo.MessageEmbed
	err = json.Unmarshal([]byte(currentAd.Content), &embed)
	if err == nil {
		// It's an embed
		if currentAd.ImageURL != "" && embed.Image == nil {
			embed.Image = &discordgo.MessageEmbedImage{URL: currentAd.ImageURL}
		}
		_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{&embed},
		})
		if err != nil {
			log.Printf("Error sending embed followup: %v", err)
			utils.SendFollowUpError(s, i.Interaction, "发送广告内容失败")
		}
	} else {
		// It's plain text
		content := currentAd.Content
		if currentAd.ImageURL != "" {
			content += "\n" + currentAd.ImageURL
		}
		utils.SendFollowUp(s, i.Interaction, content)
	}
	if b.GetConfig().LogChannelID != "" {
		go utils.LogInfo(s, b.GetConfig().LogChannelID, "广告管理", "打印广告", fmt.Sprintf("广告ID: %d\n操作人: %s", adID, i.Member.User.Username))
	}
}
