package lbadmin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"

	"github.com/bwmarrin/discordgo"
)

func HandleAddAd(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB, b model.BotConfigProvider, input string) {
	if input == "" {
		utils.SendFollowUpError(s, i.Interaction, "请输入包含广告内容的消息链接")
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
			utils.SendFollowUpError(s, i.Interaction, "序列化 Embed 内容失败")
			return
		}
		adContent = string(embedData)
	} else {
		// Otherwise, use the plain text content
		adContent = msg.Content
	}

	if adContent == "" && imageURL == "" {
		utils.SendFollowUpError(s, i.Interaction, "消息内容和图片均为空，无法添加为广告")
		return
	}

	err = database.AddLeaderboardAd(db, i.GuildID, adContent, imageURL)
	if err != nil {
		log.Printf("Failed to add leaderboard ad: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "添加广告失败")
		return
	}
	utils.SendFollowUp(s, i.Interaction, "广告已成功添加！")
	if b.GetConfig().LogChannelID != "" {
		go utils.LogInfo(s, b.GetConfig().LogChannelID, "广告管理", "添加广告", fmt.Sprintf("操作人: %s", i.Member.User.Username))
	}
}
