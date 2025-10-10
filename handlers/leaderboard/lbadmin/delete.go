package lbadmin

import (
	"database/sql"
	"fmt"
	"log"
	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

func HandleDeleteAd(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB, b model.BotConfigProvider, adIDStr string) {
	if adIDStr == "" {
		utils.SendFollowUpError(s, i.Interaction, "请输入要删除的广告 ID")
		return
	}
	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "无效的广告 ID")
		return
	}
	err = database.DeleteLeaderboardAd(db, adID, i.GuildID)
	if err != nil {
		log.Printf("Failed to delete leaderboard ad: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "删除广告失败")
		return
	}
	utils.SendFollowUp(s, i.Interaction, fmt.Sprintf("广告 #%d 已成功删除！", adID))
	if b.GetConfig().LogChannelID != "" {
		go utils.LogInfo(s, b.GetConfig().LogChannelID, "广告管理", "删除广告", fmt.Sprintf("广告ID: %d\n操作人: %s", adID, i.Member.User.Username))
	}
}
