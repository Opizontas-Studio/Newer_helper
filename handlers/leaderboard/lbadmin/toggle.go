package lbadmin

import (
	"database/sql"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

func HandleToggleAd(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB, b model.BotConfigProvider, adIDStr string) {
	if adIDStr == "" {
		utils.SendFollowUpError(s, i.Interaction, "请输入要切换状态的广告 ID")
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

	err = database.ToggleLeaderboardAd(db, adID, i.GuildID, !currentAd.Enabled)
	if err != nil {
		log.Printf("Failed to toggle leaderboard ad: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "切换广告状态失败")
		return
	}

	newState := "启用"
	if currentAd.Enabled { // Note: we check the state *before* toggling
		newState = "禁用"
	}
	utils.SendFollowUp(s, i.Interaction, fmt.Sprintf("广告 #%d 的状态已成功切换为 **%s**！", adID, newState))
	if b.GetConfig().LogChannelID != "" {
		go utils.LogInfo(s, b.GetConfig().LogChannelID, "广告管理", "切换广告状态", fmt.Sprintf("广告ID: %d\n新状态: %s\n操作人: %s", adID, newState, i.Member.User.Username))
	}
}
