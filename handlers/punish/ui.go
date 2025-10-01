package punish

import (
	"discord-bot/model"
	"discord-bot/utils"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// buildPunishmentEmbedNew creates the rich embed message for the punishment announcement using new config.
func buildPunishmentEmbedNew(i *discordgo.InteractionCreate, targetUser *discordgo.User, actionConfig *model.ActionConfig, reason string, allEvidence []Evidence, currentGuildHistory []model.PunishmentRecord, otherGuildsHistory map[string][]model.PunishmentRecord, timeoutApplied bool, timeoutDurationStr string, punishmentID int64, punishLevel *model.PunishLevel) *discordgo.MessageEmbed {
	// Get display name, fallback to type if actionConfig is nil
	displayName := "未知处罚"
	if actionConfig != nil {
		displayName = actionConfig.Name
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("%s 处罚", displayName),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: targetUser.AvatarURL(""),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "用户",
				Value: targetUser.Mention(),
			},
			{
				Name:  "处罚类型",
				Value: displayName,
			},
			{
				Name:  "原因",
				Value: reason,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Color:     getEmbedColor(punishLevel),
	}

	if punishmentID == -1 {
		embed.Description = "用户进行了一次自我处罚，本次操作不会被记录。"
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("由 %s 操作", i.Member.User.Username),
		}
	} else {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("由 %s 操作 | 处罚ID: %d", i.Member.User.Username, punishmentID),
		}
	}

	if len(allEvidence) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "已存档证据",
			Value: fmt.Sprintf("已保存 %d 条消息作为证据。", len(allEvidence)),
		})
	}

	if len(currentGuildHistory) > 1 {
		var historyValue string
		for _, rec := range currentGuildHistory {
			entry := fmt.Sprintf("操作人: <@%s>, 类型: %s, 原因: %s\n", rec.AdminID, rec.ActionType, rec.Reason)
			if rec.PunishmentID == punishmentID {
				entry = fmt.Sprintf("\n**> 本次处罚** %s", entry)
			}
			historyValue += entry
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "历史处罚记录",
			Value: historyValue,
		})
	}

	if len(otherGuildsHistory) > 0 {
		var otherGuildsValue string
		for guildID, records := range otherGuildsHistory {
			otherGuildsValue += fmt.Sprintf("在服务器 %s 存在 %d 条处罚记录\n", guildID, len(records))
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "其他服务器处罚记录",
			Value: otherGuildsValue,
		})
	}

	if timeoutApplied {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "处罚措施",
			Value: fmt.Sprintf("该用户已被处罚: %s", timeoutDurationStr),
		})
	}

	return embed
}

// buildSoftPunishmentEmbed creates a softer embed message using the description macro from config.
// This embed is sent to private messages and command execution channel.
func buildSoftPunishmentEmbed(targetUser *discordgo.User, punishLevel *model.PunishLevel, reason string, timeoutDurationStr string, adminUsername string, punishmentID int64) *discordgo.MessageEmbed {
	// Replace macros in description
	description := punishLevel.Description
	description = utils.ReplaceMacro(description, "${user}", targetUser.Mention())
	description = utils.ReplaceMacro(description, "${reason}", reason)
	description = utils.ReplaceMacro(description, "${timeout}", timeoutDurationStr)
	description = utils.ReplaceMacro(description, "${add_role_timeout_time}", punishLevel.AddRoleTimeoutTime)

	embed := &discordgo.MessageEmbed{
		Description: description,
		Color:       getEmbedColor(punishLevel),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("由 %s 操作 | 处罚ID: %d", adminUsername, punishmentID),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return embed
}

// getEmbedColor determines the embed color based on the punishment level configuration.
// Returns the configured color from ed_color field, or default red if not available.
func getEmbedColor(punishLevel *model.PunishLevel) int {
	if punishLevel != nil && punishLevel.EdColor != "" {
		return utils.ParseHexColor(punishLevel.EdColor)
	}
	return 0xff0000 // Default red color
}