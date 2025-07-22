package tasks

import (
	"database/sql"
	"discord-bot/model"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

func GeneratePunishmentStatsEmbed(dbx *sqlx.DB, targetGuildID string, duration time.Duration) (*discordgo.MessageEmbed, error) {
	since := time.Now().Add(-duration)
	stats, err := database.GetAdminPunishmentStats(dbx, targetGuildID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin punishment stats for guild %s: %v", targetGuildID, err)
	}

	total, err := database.GetTotalPunishmentCount(dbx, targetGuildID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get total punishment count for guild %s: %v", targetGuildID, err)
	}

	var sortedAdmins []string
	for adminID := range stats {
		sortedAdmins = append(sortedAdmins, adminID)
	}
	sort.Slice(sortedAdmins, func(i, j int) bool {
		return stats[sortedAdmins[i]] > stats[sortedAdmins[j]]
	})

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("### 过去 %s 内处罚统计\n", duration.String()))
	builder.WriteString(fmt.Sprintf("**总计: %d**\n\n", total))
	builder.WriteString("**管理员击杀榜:**\n")

	for i, adminID := range sortedAdmins {
		builder.WriteString(fmt.Sprintf("%d. <@%s>: %d\n", i+1, adminID, stats[adminID]))
	}

	embed := &discordgo.MessageEmbed{
		Title:       "处罚排行榜",
		Description: builder.String(),
		Timestamp:   time.Now().Format(time.RFC3339),
		Color:       0x00ff00,
	}
	return embed, nil
}

func UpdatePunishmentStats(s *discordgo.Session, db *sql.DB, config model.PunishmentStatsChannel, duration time.Duration) {
	dbx := sqlx.NewDb(db, "sqlite3")
	embed, err := GeneratePunishmentStatsEmbed(dbx, config.TargetGuildID, duration)
	if err != nil {
		log.Printf("Failed to generate punishment stats embed: %v", err)
		return
	}

	var msg *discordgo.Message
	if config.MessageID == "" {
		msg, err = s.ChannelMessageSendEmbed(config.ChannelID, embed)
		if err != nil {
			log.Printf("Failed to send punishment stats message to channel %s: %v", config.ChannelID, err)
			return
		}
		err = database.UpdatePunishmentStatsChannel(db, config.ChannelID, msg.ID)
		if err != nil {
			log.Printf("Failed to update punishment stats message ID for channel %s: %v", config.ChannelID, err)
		}
	} else {
		_, err = s.ChannelMessageEditEmbed(config.ChannelID, config.MessageID, embed)
		if err != nil {
			log.Printf("Failed to edit punishment stats message %s in channel %s: %v", config.MessageID, config.ChannelID, err)
		}
	}
}
