package punish

import (
	"discord-bot/bot"
	"discord-bot/internal/config"
	"discord-bot/model"
	"discord-bot/utils"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// HandlePunishCommand is the primary handler for the /punish command.
func HandlePunishCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	// 1. Defer response
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		log.Printf("Failed to defer interaction: %v", err)
		return
	}

	// 2. Get services from bot
	repoManager := b.GetRepositoryManager()
	configService := b.GetConfigService()
	punishmentRepo := repoManager.PunishmentRepository()
	// timedTaskRepo := repoManager.TimedTaskRepository()

	if configService == nil || punishmentRepo == nil {
		utils.SendFollowUpError(s, i.Interaction, "核心服务未初始化，无法执行操作。")
		return
	}

	// 3. Load configuration
	punishConfig := configService.GetPunishConfig()
	punishGuildConfig, ok := punishConfig.Guilds[i.GuildID]
	if !ok {
		utils.SendFollowUpError(s, i.Interaction, "❓ 此服务器未找到可用配置文件")
		return
	}
	configEntry := convertPunishGuildConfig(punishGuildConfig, configService.GetLogChannelID())

	// 4. Parse command options
	cmdOptions := parsePunishOptions(s, i)
	targetUser := cmdOptions.TargetUser
	reason := cmdOptions.Reason

	// 5. Get member details and check whitelist
	targetMember, err := s.GuildMember(i.GuildID, targetUser.ID)
	if err != nil {
		log.Printf("Error getting member details: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "无法获取成员信息。")
		return
	}
	if isUserWhitelisted(targetMember, configEntry) {
		utils.SendFollowUpError(s, i.Interaction, "该用户在白名单中，无法被处罚。")
		return
	}

	// 6. Initial punishment action: remove roles
	removePunishmentRoles(s, i.GuildID, targetUser.ID, configEntry.RemoveRoleID)

	// 7. Process evidence
	evidenceJSON, allEvidence, err := processEvidence(s, cmdOptions.MessageLinks, targetUser)
	if err != nil {
		log.Printf("Error processing evidence: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "处理证据失败。")
		return
	}

	// 8. Create punishment record using the repository
	punishmentRecord := &model.PunishmentRecord{
		MessageID:    i.ID,
		AdminID:      i.Member.User.ID,
		UserID:       targetUser.ID,
		UserUsername: targetUser.Username,
		Reason:       reason,
		GuildID:      i.GuildID,
		Timestamp:    time.Now().Unix(),
		Evidence:     evidenceJSON,
	}
	punishmentID, err := punishmentRepo.Create(punishmentRecord)
	if err != nil {
		log.Printf("Error creating punishment record: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "创建惩罚记录失败。")
		return
	}
	punishmentRecord.PunishmentID = punishmentID // Assign the newly created ID

	// 9. Apply timeout if required, now using the correct repositories
	timeoutApplied, timeoutDurationStr, err := applyTimeoutIfRequired(s, i, punishmentRepo, nil, configEntry, targetUser)
	if err != nil {
		log.Printf("Error applying timeout: %v", err)
		// Non-fatal, just log and continue
	}

	// 10. Get punishment history using the repository
	history, err := punishmentRepo.GetByUserID(targetUser.ID)
	if err != nil {
		log.Printf("Error fetching punishment history: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "获取惩罚历史失败。")
		return
	}
	currentGuildHistory, otherGuildsHistory := categorizeHistory(history, i.GuildID)

	// 11. Build and send response messages
	kickConfigForEmbed := &model.KickConfig{Data: make(map[string]model.KickConfigEntry)}
	for gid, gconf := range punishConfig.Guilds {
		kickConfigForEmbed.Data[gid] = convertPunishGuildConfig(gconf, "")
	}

	embed := buildPunishmentEmbed(i, targetUser, reason, allEvidence, currentGuildHistory, otherGuildsHistory, kickConfigForEmbed, timeoutApplied, timeoutDurationStr, punishmentID)
	punishmentMessage := sendResponseMessages(s, i, targetUser, embed, timeoutApplied, timeoutDurationStr, reason)

	// 12. Log the punishment
	logPunishment(s, i, configEntry, targetUser, cmdOptions.MessageLinks, punishmentMessage, timeoutApplied, timeoutDurationStr)
}

// convertPunishGuildConfig converts the new config structure to the old one for backward compatibility.
func convertPunishGuildConfig(newConfig *config.PunishGuildConfig, globalLogChannelID string) model.KickConfigEntry {
	return model.KickConfigEntry{
		Name:            newConfig.Name,
		LogChannelID:    globalLogChannelID,
		BaseRoleID:      newConfig.BaseRoleID,
		RemoveRoleID:    newConfig.RemoveRoleIDs,
		WhitelistRoleID: newConfig.WhitelistRoleIDs,
		Timeout: model.TimeoutConfig{
			Frequency:          newConfig.Timeout.Frequency,
			Time:               newConfig.Timeout.Time,
			TimeoutTime:        newConfig.Timeout.TimeoutTime,
			AddRole:            newConfig.Timeout.AddRoles,
			AddRoleTimeoutTime: newConfig.Timeout.AddRoleTimeoutTime,
		},
	}
}

// categorizeHistory separates punishment records into current guild and other guilds.
func categorizeHistory(history []model.PunishmentRecord, currentGuildID string) ([]model.PunishmentRecord, map[string][]model.PunishmentRecord) {
	currentGuildHistory := []model.PunishmentRecord{}
	otherGuildsHistory := make(map[string][]model.PunishmentRecord)

	for _, rec := range history {
		if rec.GuildID == currentGuildID {
			currentGuildHistory = append(currentGuildHistory, rec)
		} else {
			otherGuildsHistory[rec.GuildID] = append(otherGuildsHistory[rec.GuildID], rec)
		}
	}
	return currentGuildHistory, otherGuildsHistory
}

// --- Helper functions moved from helpers.go ---

type ParsedOptions struct {
	TargetUser   *discordgo.User
	Reason       string
	MessageLinks string
}

func parsePunishOptions(s *discordgo.Session, i *discordgo.InteractionCreate) ParsedOptions {
	options := i.ApplicationCommandData().Options
	var result ParsedOptions

	for _, opt := range options {
		switch opt.Name {
		case "user":
			result.TargetUser = opt.UserValue(s)
		case "reason":
			result.Reason = opt.StringValue()
		case "evidence":
			result.MessageLinks = opt.StringValue()
		}
	}
	return result
}

func processEvidence(s *discordgo.Session, messageLinks string, targetUser *discordgo.User) (string, []string, error) {
	if messageLinks == "" {
		return "[]", []string{}, nil
	}

	links := strings.Split(messageLinks, " ")
	var allEvidence []string

	for _, link := range links {
		parts := strings.Split(link, "/")
		if len(parts) < 3 {
			continue
		}
		channelID := parts[len(parts)-2]
		messageID := parts[len(parts)-1]

		msg, err := s.ChannelMessage(channelID, messageID)
		if err != nil {
			log.Printf("Could not retrieve message for evidence link %s: %v", link, err)
			continue
		}

		evidenceText := fmt.Sprintf("`%s` (by: `%s`): %s", msg.Timestamp.Format("2006-01-02 15:04:05"), msg.Author.Username, msg.Content)
		allEvidence = append(allEvidence, evidenceText)
	}
	return "[]", allEvidence, nil
}

func buildPunishmentEmbed(
	i *discordgo.InteractionCreate,
	targetUser *discordgo.User,
	reason string,
	allEvidence []string,
	currentGuildHistory []model.PunishmentRecord,
	otherGuildsHistory map[string][]model.PunishmentRecord,
	kickConfig *model.KickConfig,
	timeoutApplied bool,
	timeoutDurationStr string,
	punishmentID int64,
) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: "⚔️ 用户处罚通知 ⚔️",
		Color: 0xff0000,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "被处罚用户", Value: fmt.Sprintf("%s (`%s`)", targetUser.Mention(), targetUser.ID), Inline: true},
			{Name: "执行管理员", Value: fmt.Sprintf("%s (`%s`)", i.Member.User.Mention(), i.Member.User.ID), Inline: true},
			{Name: "处罚原因", Value: reason, Inline: false},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Punishment ID: %d", punishmentID),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if len(allEvidence) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "💬 证据", Value: strings.Join(allEvidence, "\n"), Inline: false})
	}

	if timeoutApplied {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "⏳ 禁言", Value: fmt.Sprintf("用户已被禁言，时长：%s", timeoutDurationStr), Inline: false})
	}

	historyStr := formatHistory(currentGuildHistory, otherGuildsHistory, kickConfig)
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "📜 处罚历史", Value: historyStr, Inline: false})

	return embed
}

func formatHistory(currentGuildHistory []model.PunishmentRecord, otherGuildsHistory map[string][]model.PunishmentRecord, kickConfig *model.KickConfig) string {
	var historyBuilder strings.Builder
	historyBuilder.WriteString(fmt.Sprintf("**本服务器 (%d):**\n", len(currentGuildHistory)))
	if len(currentGuildHistory) == 0 {
		historyBuilder.WriteString("无\n")
	} else {
		for _, rec := range currentGuildHistory {
			historyBuilder.WriteString(fmt.Sprintf("- <t:%d:f> (ID: %d) - %s\n", rec.Timestamp, rec.PunishmentID, rec.Reason))
		}
	}

	if len(otherGuildsHistory) > 0 {
		historyBuilder.WriteString("\n**其他服务器:**\n")
		for guildID, records := range otherGuildsHistory {
			guildName := "未知服务器"
			if config, ok := kickConfig.Data[guildID]; ok {
				guildName = config.Name
			}
			historyBuilder.WriteString(fmt.Sprintf("**%s (%d):**\n", guildName, len(records)))
			for _, rec := range records {
				historyBuilder.WriteString(fmt.Sprintf("- <t:%d:f> (ID: %d) - %s\n", rec.Timestamp, rec.PunishmentID, rec.Reason))
			}
		}
	}
	return historyBuilder.String()
}

func sendResponseMessages(s *discordgo.Session, i *discordgo.InteractionCreate, targetUser *discordgo.User, embed *discordgo.MessageEmbed, timeoutApplied bool, timeoutDurationStr, reason string) *discordgo.Message {
	content := fmt.Sprintf("✅ 操作成功：已处罚用户 %s。", targetUser.Mention())
	response, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
		Embeds:  &[]*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		log.Printf("Failed to send initial response: %v", err)
		return nil
	}

	dmMessage := fmt.Sprintf("您好，您因为「%s」已被管理员处罚。", reason)
	if timeoutApplied {
		dmMessage += fmt.Sprintf(" 同时，您已被禁言，时长为 %s。", timeoutDurationStr)
	}
	sendDM(s, targetUser.ID, dmMessage)

	return response
}

func sendDM(s *discordgo.Session, userID, message string) {
	channel, err := s.UserChannelCreate(userID)
	if err != nil {
		log.Printf("Failed to create DM channel with user %s: %v", userID, err)
		return
	}
	_, err = s.ChannelMessageSend(channel.ID, message)
	if err != nil {
		log.Printf("Failed to send DM to user %s: %v", userID, err)
	}
}

func logPunishment(s *discordgo.Session, i *discordgo.InteractionCreate, config model.KickConfigEntry, targetUser *discordgo.User, messageLinks string, punishmentMessage *discordgo.Message, timeoutApplied bool, timeoutDurationStr string) {
	if config.LogChannelID == "" {
		log.Println("Log channel ID is not configured, skipping log message.")
		return
	}

	logEmbed := &discordgo.MessageEmbed{
		Title: "📝 处罚日志",
		Color: 0xf0e68c,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "被处罚用户", Value: fmt.Sprintf("%s (`%s`)", targetUser.Username, targetUser.ID), Inline: true},
			{Name: "执行管理员", Value: fmt.Sprintf("%s (`%s`)", i.Member.User.Username, i.Member.User.ID), Inline: true},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if messageLinks != "" {
		logEmbed.Fields = append(logEmbed.Fields, &discordgo.MessageEmbedField{Name: "证据链接", Value: messageLinks, Inline: false})
	}

	if punishmentMessage != nil {
		logEmbed.Fields = append(logEmbed.Fields, &discordgo.MessageEmbedField{
			Name:  "处罚消息",
			Value: fmt.Sprintf("[点击跳转](https://discord.com/channels/%s/%s/%s)", punishmentMessage.GuildID, punishmentMessage.ChannelID, punishmentMessage.ID),
		})
	}

	if timeoutApplied {
		logEmbed.Fields = append(logEmbed.Fields, &discordgo.MessageEmbedField{Name: "禁言时长", Value: timeoutDurationStr, Inline: true})
	}

	_, err := s.ChannelMessageSendEmbed(config.LogChannelID, logEmbed)
	if err != nil {
		log.Printf("Failed to send log message to channel %s: %v", config.LogChannelID, err)
	}
}
