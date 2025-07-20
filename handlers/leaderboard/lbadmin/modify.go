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

func HandleModifyAd(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB, b model.BotConfigProvider, adIDStr, input string) {
	if adIDStr == "" || input == "" {
		utils.SendFollowUpError(s, i.Interaction, "请输入要修改的广告 ID 和包含新内容的消息链接")
		return
	}

	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "无效的广告 ID")
		return
	}

	parsedMessages, err := utils.ParseMessageLinks(s, input)
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "解析消息链接失败: "+err.Error())
		return
	}
	if len(parsedMessages) == 0 {
		utils.SendFollowUpError(s, i.Interaction, "没有找到有效的消息链接")
		return
	}

	msg := parsedMessages[0]
	var adContent string
	var imageURL string

	if len(msg.Attachments) > 0 {
		imageURL = msg.Attachments[0].URL
	}

	if len(msg.Embeds) > 0 {
		embedData, err := json.Marshal(msg.Embeds[0])
		if err != nil {
			log.Printf("Failed to marshal embed: %v", err)
			utils.SendFollowUpError(s, i.Interaction, "序列化 Embed 内容失败")
			return
		}
		adContent = string(embedData)
	} else {
		adContent = msg.Content
	}

	if adContent == "" && imageURL == "" {
		utils.SendFollowUpError(s, i.Interaction, "消息内容和图片均为空，无法作为广告内容")
		return
	}

	err = database.UpdateLeaderboardAd(db, adID, i.GuildID, adContent, imageURL)
	if err != nil {
		log.Printf("Failed to update leaderboard ad: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "修改广告失败")
		return
	}
	utils.SendFollowUp(s, i.Interaction, fmt.Sprintf("广告 #%d 已成功修改！", adID))
	if b.GetConfig().LogChannelID != "" {
		go utils.LogInfo(s, b.GetConfig().LogChannelID, "广告管理", "修改广告", fmt.Sprintf("广告ID: %d\n操作人: %s", adID, i.Member.User.Username))
	}
}
