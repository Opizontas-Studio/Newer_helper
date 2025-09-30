package punish

import (
	"discord-bot/model"
	punishments_db "discord-bot/utils/database/punishments"
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
	Action       string
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

	var action string
	if actionOpt, ok := optionMap["action"]; ok {
		action = actionOpt.StringValue()
	}
	if action == "" {
		action = "re-answer" // Default action
	}

	return ParsedOptions{
		TargetUser:   optionMap["user"].UserValue(s),
		Action:       action,
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

// removePunishmentRoles removes specified roles from a user.
func removePunishmentRoles(s *discordgo.Session, guildID, userID string, roleIDs []string) {
	// 如果 roleIDs 包含 "0"，则不执行任何操作
	for _, roleID := range roleIDs {
		if roleID == "0" {
			return
		}
	}

	for _, roleID := range roleIDs {
		err := s.GuildMemberRoleRemove(guildID, userID, roleID)
		if err != nil {
			log.Printf("Failed to remove role %s from user %s: %v", roleID, userID, err)
		}
	}
}

// addPunishmentRecord adds a new punishment record to the database and returns the new record's ID.
func addPunishmentRecord(db *sqlx.DB, i *discordgo.InteractionCreate, targetUser *discordgo.User, reason, evidenceJSON, actionType string, tempRoles []string, rolesRemoveAt map[string]time.Time) (int64, error) {
	// Serialize temp roles to JSON
	tempRolesJSON, err := json.Marshal(tempRoles)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize temp roles: %w", err)
	}

	// Serialize roles remove times to JSON
	rolesRemoveAtJSON, err := json.Marshal(rolesRemoveAt)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize roles remove times: %w", err)
	}

	record := model.PunishmentRecord{
		MessageID:        i.ID,
		AdminID:          i.Member.User.ID,
		UserID:           targetUser.ID,
		UserUsername:     targetUser.Username,
		Reason:           reason,
		GuildID:          i.GuildID,
		Timestamp:        time.Now().Unix(),
		Evidence:         evidenceJSON,
		ActionType:       actionType,
		TempRolesJSON:    string(tempRolesJSON),
		RolesRemoveAt:    string(rolesRemoveAtJSON),
		PunishmentStatus: "active",
	}
	return punishments_db.AddPunishmentRecord(db, record)
}

// getPunishmentHistory retrieves and categorizes punishment records for a user.
func getPunishmentHistory(db *sqlx.DB, userID, currentGuildID string) ([]model.PunishmentRecord, map[string][]model.PunishmentRecord, error) {
	history, err := punishments_db.GetPunishmentRecordsByUserID(db, userID, nil)
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

// isUserWhitelistedForAction checks if a user has a role that is on the action-specific whitelist.
func isUserWhitelistedForAction(targetMember *discordgo.Member, actionConfig model.ActionConfig) bool {
	for _, whitelistRole := range actionConfig.WhitelistRoleID {
		for _, userRole := range targetMember.Roles {
			if userRole == whitelistRole {
				return true
			}
		}
	}
	return false
}

// getPunishmentLevel returns the punishment level configuration for the given count.
func getPunishmentLevel(actionConfig model.ActionConfig, count int) *model.PunishLevel {
	countStr := fmt.Sprintf("%d", count)
	if level, ok := actionConfig.Data[countStr]; ok {
		return &level
	}
	return nil
}

// getHighestPunishmentLevel returns the highest available punishment level.
func getHighestPunishmentLevel(actionConfig model.ActionConfig) *model.PunishLevel {
	var highest *model.PunishLevel
	var highestKey int = -1

	for key, level := range actionConfig.Data {
		if keyInt := parseIntSafe(key); keyInt > highestKey {
			highestKey = keyInt
			levelCopy := level
			highest = &levelCopy
		}
	}
	return highest
}

// parseIntSafe safely parses an integer, returning 0 if parsing fails.
func parseIntSafe(s string) int {
	if val, err := fmt.Sscanf(s, "%d", new(int)); err == nil && val == 1 {
		var result int
		fmt.Sscanf(s, "%d", &result)
		return result
	}
	return 0
}

// applyPunishmentLevel applies the punishment actions according to the punishment level.
// Returns: timeoutApplied, timeoutDurationStr, tempRoles, rolesRemoveAt
func applyPunishmentLevel(s *discordgo.Session, i *discordgo.InteractionCreate, targetUser *discordgo.User, level model.PunishLevel) (bool, string, []string, map[string]time.Time) {
	// Remove roles
	removePunishmentRoles(s, i.GuildID, targetUser.ID, level.RemoveRoleID)

	timeoutApplied := false
	timeoutDurationStr := ""

	// Apply timeout/ban
	if level.Timeout != "" && level.Timeout != "0" {
		if level.Timeout == "ban" {
			// Apply ban
			err := s.GuildBanCreateWithReason(i.GuildID, targetUser.ID, "Automatic punishment ban", 0)
			if err != nil {
				log.Printf("Failed to ban user %s: %v", targetUser.ID, err)
			} else {
				timeoutApplied = true
				timeoutDurationStr = "永久封禁"
			}
		} else {
			// Apply timeout (assume it's in days)
			days := parseIntSafe(level.Timeout)
			if days > 0 {
				timeoutDuration := time.Duration(days) * 24 * time.Hour
				timeoutUntil := time.Now().Add(timeoutDuration)
				err := s.GuildMemberTimeout(i.GuildID, targetUser.ID, &timeoutUntil)
				if err != nil {
					log.Printf("Failed to timeout user %s: %v", targetUser.ID, err)
				} else {
					timeoutApplied = true
					timeoutDurationStr = fmt.Sprintf("%d天", days)
				}
			}
		}
	}

	// Track temporary roles and their removal times
	var tempRoles []string
	rolesRemoveAt := make(map[string]time.Time)

	// Add roles with optional timeout
	for _, roleID := range level.AddRole {
		if roleID == "0" {
			continue // Skip "0" roles
		}

		err := s.GuildMemberRoleAdd(i.GuildID, targetUser.ID, roleID)
		if err != nil {
			log.Printf("Failed to add role %s to user %s: %v", roleID, targetUser.ID, err)
			continue
		}

		tempRoles = append(tempRoles, roleID)

		// Schedule role removal if timeout is specified
		if level.AddRoleTimeoutTime != "" && level.AddRoleTimeoutTime != "0" && level.AddRoleTimeoutTime != "-1" {
			timeoutMinutes := parseIntSafe(level.AddRoleTimeoutTime)
			if timeoutMinutes > 0 {
				removeAt := time.Now().Add(time.Duration(timeoutMinutes) * time.Minute)
				rolesRemoveAt[roleID] = removeAt
			}
		}
	}

	return timeoutApplied, timeoutDurationStr, tempRoles, rolesRemoveAt
}


// logPunishmentNew sends a detailed log message to the configured log channel using new config.
func logPunishmentNew(i *discordgo.InteractionCreate, actionConfig model.ActionConfig, targetUser *discordgo.User) {
	// Note: ActionConfig doesn't have LogChannelID, so we'll skip logging for now
	// This would need to be added to the configuration structure if required
	log.Printf("Punishment logged for action %s: user %s by %s", actionConfig.Type, targetUser.ID, i.Member.User.ID)
}
