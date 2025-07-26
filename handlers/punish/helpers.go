package punish

import (
	"discord-bot/model"
	"discord-bot/utils/database"
	"time"

	"discord-bot/utils"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

// Evidence holds the content and attachments of a message.
type Evidence struct {
	Content     string   `json:"content"`
	Attachments []string `json:"attachments"`
}

// This file will contain helper functions for the punish command logic.

// ParsedOptions holds the parsed options from the punish command interaction.
type ParsedOptions struct {
	TargetUser   *discordgo.User
	Reason       string
	MessageLinks string
	OptionMap    map[string]*discordgo.ApplicationCommandInteractionDataOption
}

// parsePunishOptions extracts and returns the command options from the interaction.
func parsePunishOptions(s *discordgo.Session, i *discordgo.InteractionCreate) ParsedOptions {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var messageLinks string
	if messageLinksOpt, ok := optionMap["message_links"]; ok {
		messageLinks = messageLinksOpt.StringValue()
	}

	var reason string
	if reasonOpt, ok := optionMap["reason"]; ok {
		reason = reasonOpt.StringValue()
	}

	return ParsedOptions{
		TargetUser:   optionMap["user"].UserValue(s),
		Reason:       reason,
		MessageLinks: messageLinks,
		OptionMap:    optionMap,
	}
}

// processEvidence fetches messages from links, downloads attachments, and returns serialized evidence data.
func processEvidence(s *discordgo.Session, messageLinks string, targetUser *discordgo.User) (string, []Evidence, error) {
	if messageLinks == "" {
		return "[]", nil, nil
	}

	links := strings.Fields(messageLinks)
	linkRegex := regexp.MustCompile(`https://discord.com/channels/(\d+)/(\d+)/(\d+)`)
	var allEvidence []Evidence

	for _, link := range links {
		matches := linkRegex.FindStringSubmatch(link)
		if len(matches) != 4 {
			log.Printf("Invalid Discord message link format: %s", link)
			continue
		}
		_, channelID, messageID := matches[1], matches[2], matches[3]

		msg, err := s.ChannelMessage(channelID, messageID)
		if err != nil {
			log.Printf("Failed to fetch message %s: %v", messageID, err)
			continue
		}

		var downloadedAttachments []string
		for _, attachment := range msg.Attachments {
			fileName := fmt.Sprintf("%s-%s", attachment.ID, attachment.Filename)
			filePath, err := utils.DownloadFile(attachment.URL, filepath.Join("data", "evidence", targetUser.ID), fileName)
			if err != nil {
				log.Printf("Failed to download attachment %s: %v", attachment.URL, err)
				continue
			}
			downloadedAttachments = append(downloadedAttachments, filePath)
		}

		allEvidence = append(allEvidence, Evidence{
			Content:     msg.Content,
			Attachments: downloadedAttachments,
		})
	}

	evidenceJSON, err := json.Marshal(allEvidence)
	if err != nil {
		return "", nil, fmt.Errorf("failed to serialize evidence: %w", err)
	}

	return string(evidenceJSON), allEvidence, nil
}

// isUserWhitelisted checks if a user has a role that is on the whitelist.
func isUserWhitelisted(targetMember *discordgo.Member, configEntry model.KickConfigEntry) bool {
	for _, whitelistRole := range configEntry.WhitelistRoleID {
		for _, userRole := range targetMember.Roles {
			if userRole == whitelistRole {
				return true
			}
		}
	}
	return false
}

// removePunishmentRoles removes specified roles from a user.
func removePunishmentRoles(s *discordgo.Session, guildID, userID string, roleIDs []string) {
	for _, roleID := range roleIDs {
		err := s.GuildMemberRoleRemove(guildID, userID, roleID)
		if err != nil {
			log.Printf("Failed to remove role %s from user %s: %v", roleID, userID, err)
		}
	}
}

// addTimedRoles adds roles to a user and schedules their removal.
func addTimedRoles(s *discordgo.Session, i *discordgo.InteractionCreate, kickConfig *model.KickConfig, configEntry model.KickConfigEntry, targetUser *discordgo.User) error {
	if configEntry.Timeout.AddRoleTimeoutTime == "" {
		// Fallback to just adding roles without scheduling removal
		for _, roleID := range configEntry.Timeout.AddRole {
			err := s.GuildMemberRoleAdd(i.GuildID, targetUser.ID, roleID)
			if err != nil {
				log.Printf("Failed to add role %s to user %s: %v", roleID, targetUser.ID, err)
			}
		}
		return nil
	}

	removalDuration, err := utils.ParseDuration(configEntry.Timeout.AddRoleTimeoutTime)
	if err != nil {
		return fmt.Errorf("error parsing add_role_timeout_time: %w", err)
	}

	removeAt := time.Now().Add(removalDuration)
	timedTaskDB, err := database.InitTimedTaskDB(kickConfig.InitConfig.DBPath)
	if err != nil {
		return fmt.Errorf("failed to connect to timed task DB for scheduling: %w", err)
	}
	defer timedTaskDB.Close()

	for _, roleID := range configEntry.Timeout.AddRole {
		err := s.GuildMemberRoleAdd(i.GuildID, targetUser.ID, roleID)
		if err != nil {
			log.Printf("Failed to add role %s to user %s: %v", roleID, targetUser.ID, err)
			continue // Continue to next role even if one fails
		}

		task := model.TimedTask{
			GuildID:  i.GuildID,
			UserID:   targetUser.ID,
			RoleID:   roleID,
			RemoveAt: removeAt,
		}
		if err := database.AddTimedTask(timedTaskDB, task); err != nil {
			log.Printf("Failed to schedule role removal for user %s, role %s: %v", targetUser.ID, roleID, err)
		}
	}
	return nil
}

// applyTimeoutIfRequired checks punishment history and applies timeout if conditions are met.
func applyTimeoutIfRequired(s *discordgo.Session, i *discordgo.InteractionCreate, db *sqlx.DB, kickConfig *model.KickConfig, configEntry model.KickConfigEntry, targetUser *discordgo.User) (bool, string, error) {
	if configEntry.Timeout.Frequency <= 0 || configEntry.Timeout.Time == "" {
		return false, "", nil
	}

	duration, err := utils.ParseDuration(configEntry.Timeout.Time)
	if err != nil {
		return false, "", fmt.Errorf("error parsing timeout duration: %w", err)
	}

	since := time.Now().Add(-duration)
	recentHistory, err := database.GetPunishmentRecordsByUserID(db, targetUser.ID, &since)
	if err != nil {
		return false, "", fmt.Errorf("error fetching recent punishment history: %w", err)
	}

	var currentGuildRecentHistory []model.PunishmentRecord
	for _, rec := range recentHistory {
		if rec.GuildID == i.GuildID {
			currentGuildRecentHistory = append(currentGuildRecentHistory, rec)
		}
	}

	if len(currentGuildRecentHistory) < configEntry.Timeout.Frequency {
		return false, "", nil
	}

	// Apply timeout
	if configEntry.Timeout.TimeoutTime == "" {
		return false, "", nil
	}

	timeoutDuration, err := utils.ParseDuration(configEntry.Timeout.TimeoutTime)
	if err != nil {
		return false, "", fmt.Errorf("error parsing timeout_time: %w", err)
	}

	timeoutUntil := time.Now().Add(timeoutDuration)
	err = s.GuildMemberTimeout(i.GuildID, targetUser.ID, &timeoutUntil)
	if err != nil {
		return false, "", fmt.Errorf("failed to timeout user %s: %w", targetUser.ID, err)
	}

	log.Printf("Successfully timed out user %s for %s", targetUser.ID, configEntry.Timeout.TimeoutTime)

	// Add roles and schedule their removal
	if err := addTimedRoles(s, i, kickConfig, configEntry, targetUser); err != nil {
		log.Printf("Failed to add timed roles: %v", err) // Log error but don't block timeout
	}

	return true, configEntry.Timeout.TimeoutTime, nil
}

// addPunishmentRecord adds a new punishment record to the database and returns the new record's ID.
func addPunishmentRecord(db *sqlx.DB, i *discordgo.InteractionCreate, targetUser *discordgo.User, reason, evidenceJSON string) (int64, error) {
	record := model.PunishmentRecord{
		MessageID:    i.ID,
		AdminID:      i.Member.User.ID,
		UserID:       targetUser.ID,
		UserUsername: targetUser.Username,
		Reason:       reason,
		GuildID:      i.GuildID,
		Timestamp:    time.Now().Unix(),
		Evidence:     evidenceJSON,
	}
	return database.AddPunishmentRecord(db, record)
}

// getPunishmentHistory retrieves and categorizes punishment records for a user.
func getPunishmentHistory(db *sqlx.DB, userID, currentGuildID string) ([]model.PunishmentRecord, map[string][]model.PunishmentRecord, error) {
	history, err := database.GetPunishmentRecordsByUserID(db, userID, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching punishment history: %w", err)
	}

	currentGuildHistory := []model.PunishmentRecord{}
	otherGuildsHistory := make(map[string][]model.PunishmentRecord)

	for _, rec := range history {
		if rec.GuildID == currentGuildID {
			currentGuildHistory = append(currentGuildHistory, rec)
		} else {
			otherGuildsHistory[rec.GuildID] = append(otherGuildsHistory[rec.GuildID], rec)
		}
	}

	return currentGuildHistory, otherGuildsHistory, nil
}

// buildPunishmentEmbed creates the rich embed message for the punishment announcement.
func buildPunishmentEmbed(i *discordgo.InteractionCreate, targetUser *discordgo.User, reason string, allEvidence []Evidence, currentGuildHistory []model.PunishmentRecord, otherGuildsHistory map[string][]model.PunishmentRecord, kickConfig *model.KickConfig, timeoutApplied bool, timeoutDurationStr string, punishmentID int64) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: "用户惩罚",
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: targetUser.AvatarURL(""),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "用户",
				Value: targetUser.Mention(),
			},
			{
				Name:  "原因",
				Value: reason,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("由 %s 操作 | 处罚ID: %d", i.Member.User.Username, punishmentID),
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Color:     0xff0000,
	}

	if len(allEvidence) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "已存档证据",
			Value: fmt.Sprintf("已保存 %d 条消息作为证据。", len(allEvidence)),
		})
	}

	if len(currentGuildHistory) > 0 {
		var historyValue string
		for _, rec := range currentGuildHistory {
			entry := fmt.Sprintf("操作人: <@%s>, 原因: %s\n", rec.AdminID, rec.Reason)
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
			var guildName string
			if guildConfig, exists := kickConfig.Data[guildID]; exists {
				guildName = guildConfig.Name
			} else {
				guildName = "未知服务器"
			}
			otherGuildsValue += fmt.Sprintf("在'%s'存在 %d 条处罚记录\n", guildName, len(records))
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "其他服务器处罚记录",
			Value: otherGuildsValue,
		})
	}

	if timeoutApplied {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "自动禁言",
			Value: fmt.Sprintf("该用户已被自动禁言，时长为: %s", timeoutDurationStr),
		})
	}

	if configEntry, ok := kickConfig.Data[i.GuildID]; ok && configEntry.Timeout.Frequency <= 0 {
		embed.Footer.Text += "\n此服务器已禁用自动禁言"
	}

	return embed
}

// sendPrivatePunishmentNotification sends a private message to the punished user.
func sendPrivatePunishmentNotification(s *discordgo.Session, i *discordgo.InteractionCreate, targetUser *discordgo.User, reason string, timeoutApplied bool, timeoutDurationStr string) {
	var description string
	guild, err := s.Guild(i.GuildID)
	if err != nil {
		log.Printf("Failed to get guild details: %v", err)
		// Fallback to a generic guild name
		guild = &discordgo.Guild{Name: "一个服务器"}
	}

	if timeoutApplied {
		description = fmt.Sprintf("您在 **%s** 服务器因 **%s** 被禁言，时长为 **%s**。", guild.Name, reason, timeoutDurationStr)
	} else {
		description = fmt.Sprintf("您在 **%s** 服务器因 **%s** 被处罚。", guild.Name, reason)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "处罚通知",
		Description: description,
		Color:       0xff0000, // Red
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	utils.SendPrivateEmbedMessage(s, targetUser.ID, embed)
}

// sendResponseMessages sends the final confirmation and public embed messages.
func sendResponseMessages(s *discordgo.Session, i *discordgo.InteractionCreate, targetUser *discordgo.User, embed *discordgo.MessageEmbed, timeoutApplied bool, timeoutDurationStr string, reason string) *discordgo.Message {
	responseMessage := "✅ 惩罚指令已成功执行。"
	if timeoutApplied {
		timeoutMessage := fmt.Sprintf("用户 %s 已被禁言，时长为 %s。", targetUser.Username, timeoutDurationStr)
		responseMessage = fmt.Sprintf("%s\n%s", responseMessage, timeoutMessage)
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &responseMessage,
	})

	// Send private notification
	sendPrivatePunishmentNotification(s, i, targetUser, reason, timeoutApplied, timeoutDurationStr)

	punishmentMessage, err := s.ChannelMessageSendEmbed(i.ChannelID, embed)
	if err != nil {
		log.Printf("Failed to send punishment embed to channel %s: %v", i.ChannelID, err)
		return nil
	}
	return punishmentMessage
}

// logPunishment sends a detailed log message to the configured log channel.
func logPunishment(s *discordgo.Session, i *discordgo.InteractionCreate, configEntry model.KickConfigEntry, targetUser *discordgo.User, messageLinks string, punishmentMessage *discordgo.Message, timeoutApplied bool, timeoutDurationStr string) {
	if configEntry.LogChannelID == "" {
		return
	}

	var logDetails strings.Builder
	logDetails.WriteString(fmt.Sprintf("执行人: %s (`%s`)\n", i.Member.User.Username, i.Member.User.ID))
	logDetails.WriteString(fmt.Sprintf("被处罚用户: %s (`%s`)\n", targetUser.Username, targetUser.ID))

	timeoutStatus := "否"
	if timeoutApplied {
		timeoutStatus = fmt.Sprintf("是 (时长: %s)", timeoutDurationStr)
	}
	logDetails.WriteString(fmt.Sprintf("是否禁言: %s\n", timeoutStatus))

	if messageLinks != "" {
		logDetails.WriteString(fmt.Sprintf("证据链接: %s\n", messageLinks))
	}

	if punishmentMessage != nil {
		punishmentLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, i.ChannelID, punishmentMessage.ID)
		logDetails.WriteString(fmt.Sprintf("处罚消息链接: %s\n", punishmentLink))
	}

	err := utils.LogInfo(s, configEntry.LogChannelID, "处罚模块", "执行处罚", logDetails.String())
	if err != nil {
		log.Printf("Failed to send punish log: %v", err)
	}
}
