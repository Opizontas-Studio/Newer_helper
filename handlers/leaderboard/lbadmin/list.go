package lbadmin

import (
	"database/sql"
	"fmt"
	"log"
	"newer_helper/utils"
	"newer_helper/utils/database"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func HandleListAds(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB) {
	ads, err := database.ListLeaderboardAds(db, i.GuildID)
	if err != nil {
		log.Printf("Failed to list leaderboard ads: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "获取广告列表失败")
		return
	}
	if len(ads) == 0 {
		utils.SendFollowUp(s, i.Interaction, "当前没有广告。")
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
	utils.SendFollowUp(s, i.Interaction, builder.String())
}
