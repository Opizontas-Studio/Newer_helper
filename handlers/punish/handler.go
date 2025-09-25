package punish

import (
	"crypto/rand"
	"discord-bot/bot"
	preset_pkg "discord-bot/handlers/preset"
	"discord-bot/utils"
	punishments_db "discord-bot/utils/database/punishments"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// HandlePunishCommand handles the initial slash command for punishing a user.
// It parses options and passes them to the core punishment logic.
func HandlePunishCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	if err := utils.DeferResponse(s, i, true); err != nil {
		log.Printf("Failed to defer interaction: %v", err)
		return
	}

	cmdOptions := parsePunishOptions(s, i)

	reason := cmdOptions.Reason
	if reason == "" {
		reason = "使用第三方类型提问，违反问答规范"
	}

	applyAndLogPunishment(s, i, b, cmdOptions.TargetUser, cmdOptions.Action, reason, cmdOptions.MessageLinks)
}

// HandleQuickPunishCommand creates and displays a modal for a quick punishment.
func HandleQuickPunishCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	// Permission Check
	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		return
	}
	permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
	if permissionLevel != utils.AdminPermission && permissionLevel != utils.SuperAdminPermission && permissionLevel != utils.DeveloperPermission {
		utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
		return
	}

	targetMessage := i.ApplicationCommandData().Resolved.Messages[i.ApplicationCommandData().TargetID]

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "punish_modal_" + targetMessage.ID,
			Title:    "快速处罚",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "reason",
							Label:       "处罚原因",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "请输入处罚原因",
							Value:       "使用第三方类型提问，违反问答规范",
							Required:    true,
						},
					},
				},
			},
		},
	})

	if err != nil {
		log.Printf("Error responding to quick punish command: %v", err)
	}
}

// HandlePunishModalSubmit handles the submission of the punishment modal.
// It extracts data and triggers the core punishment logic.
func HandlePunishModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	data := i.ModalSubmitData()
	reason := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	targetMessageID := strings.TrimPrefix(data.CustomID, "punish_modal_")

	targetMessage, err := s.ChannelMessage(i.ChannelID, targetMessageID)
	if err != nil {
		log.Printf("Error fetching target message: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Could not retrieve the message to be punished.")
		return
	}

	messageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, i.ChannelID, targetMessage.ID)

	// --- Create and store pending punishment ---
	pendingIDBytes := make([]byte, 8)
	_, err = rand.Read(pendingIDBytes)
	if err != nil {
		log.Printf("Error generating random ID for pending punishment: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to create a pending punishment.")
		return
	}
	pendingID := hex.EncodeToString(pendingIDBytes)

	pendingPunishment := &bot.PendingPunishment{
		TargetUser:    targetMessage.Author,
		Reason:        reason,
		EvidenceLinks: messageLink,
		Interaction:   i.Interaction,
		Timestamp:     time.Now(),
	}

	b.GetPendingPunishmentsMutex().Lock()
	b.GetPendingPunishments()[pendingID] = pendingPunishment
	b.GetPendingPunishmentsMutex().Unlock()

	// --- Load punish config to get actions ---
	punishConfig, err := utils.LoadPunishConfig("config/config_file/punish_config.json")
	if err != nil {
		log.Printf("Error loading punish config: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to load punishment configuration.")
		return
	}
	guildActions, ok := punishConfig.PunishConfig[i.GuildID]
	if !ok {
		utils.SendFollowUpError(s, i.Interaction, "此服务器未找到可用配置文件")
		return
	}

	// --- Create action buttons ---
	var components []discordgo.MessageComponent
	actionRow := discordgo.ActionsRow{}
	for actionKey, actionConfig := range guildActions {
		actionRow.Components = append(actionRow.Components, discordgo.Button{
			Label:    actionConfig.Name,
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("punish_action_%s_%s", pendingID, actionKey),
		})
	}
	components = append(components, actionRow)

	// --- Send preview message ---
	embed := buildPunishmentPreviewEmbed(i, targetMessage.Author, reason, messageLink)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsEphemeral,
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})
	if err != nil {
		log.Printf("Error sending punishment preview message: %v", err)
	}
}

// HandlePunishActionSelection handles the selection of a punishment action from the preview message.
func HandlePunishActionSelection(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	customIDParts := strings.Split(i.MessageComponentData().CustomID, "_")
	if len(customIDParts) != 4 {
		log.Printf("Invalid punish action CustomID: %s", i.MessageComponentData().CustomID)
		return
	}
	pendingID := customIDParts[2]
	action := customIDParts[3]

	b.GetPendingPunishmentsMutex().Lock()
	pendingPunishment, ok := b.GetPendingPunishments()[pendingID]
	if !ok {
		b.GetPendingPunishmentsMutex().Unlock()
		utils.SendEphemeralResponse(s, i, "This punishment request has expired or is invalid.")
		return
	}
	// Immediately remove to prevent double execution
	delete(b.GetPendingPunishments(), pendingID)
	b.GetPendingPunishmentsMutex().Unlock()

	// Defer the response now that we have the interaction from the button click
	if err := utils.DeferResponse(s, i, true); err != nil {
		log.Printf("Failed to defer interaction: %v", err)
		return
	}

	// Execute the punishment
	applyAndLogPunishment(s, i, b, pendingPunishment.TargetUser, action, pendingPunishment.Reason, pendingPunishment.EvidenceLinks)

	// Disable components on the original message
	disabledComponents := []discordgo.MessageComponent{}
	for _, comp := range i.Message.Components {
		row, ok := comp.(*discordgo.ActionsRow)
		if !ok {
			continue
		}
		newRow := discordgo.ActionsRow{}
		for _, buttonComp := range row.Components {
			button, ok := buttonComp.(*discordgo.Button)
			if !ok {
				continue
			}
			newButton := *button
			newButton.Disabled = true
			newRow.Components = append(newRow.Components, newButton)
		}
		disabledComponents = append(disabledComponents, newRow)
	}

	// Update the original message to show it's been handled
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Components: &disabledComponents,
	})
}

// buildPunishmentPreviewEmbed creates the embed for the punishment preview message.
func buildPunishmentPreviewEmbed(i *discordgo.InteractionCreate, targetUser *discordgo.User, reason, evidenceLinks string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "处罚预览",
		Description: fmt.Sprintf("请选择要对 %s 执行的处罚类型。", targetUser.Mention()),
		Color:       0xffa500, // Orange
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "处罚对象",
				Value: fmt.Sprintf("%s (`%s`)", targetUser.Username, targetUser.ID),
			},
			{
				Name:  "处罚原因",
				Value: reason,
			},
			{
				Name:  "证据链接",
				Value: evidenceLinks,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    fmt.Sprintf("操作人: %s", i.Member.User.Username),
			IconURL: i.Member.User.AvatarURL(""),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// applyAndLogPunishment is the core function that handles the punishment process.
// It centralizes the logic for configuration loading, validation, database operations, and notifications.
func applyAndLogPunishment(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, targetUser *discordgo.User, action, reason, evidenceLinks string) {
	isSelfPunish := i.Member.User.ID == targetUser.ID

	if !isSelfPunish {
		if !utils.CheckAndSetPunishLock(targetUser.ID) {
			utils.SendFollowUpError(s, i.Interaction, "对该用户的处罚操作过于频繁，请 5 分钟后再试。")
			return
		}
	}

	// Load new punishment configuration
	punishConfig, err := utils.LoadPunishConfig("config/config_file/punish_config.json")
	if err != nil {
		log.Printf("Error loading punish config: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to load punishment configuration.")
		return
	}

	// Get guild-specific action configurations
	guildActions, ok := punishConfig.PunishConfig[i.GuildID]
	if !ok {
		utils.SendFollowUpError(s, i.Interaction, "❓ 此服务器未找到可用配置文件")
		return
	}

	// Get specific action configuration
	actionConfig, ok := guildActions[action]
	if !ok {
		utils.SendFollowUpError(s, i.Interaction, fmt.Sprintf("❓ 处罚类型 '%s' 未在配置中找到", action))
		return
	}

	// Check admin rate limit
	if !utils.CheckAndIncrementAdminAction(i.Member.User.ID, action, actionConfig.PeeUserLimit, 24*time.Hour) {
		utils.SendFollowUpError(s, i.Interaction, fmt.Sprintf("您今天执行 '%s' 操作的次数已达上限。", actionConfig.Name))
		return
	}

	// Get target member for whitelist check
	targetMember, err := s.GuildMember(i.GuildID, targetUser.ID)
	if err != nil {
		log.Printf("Error getting member details: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Could not retrieve member details.")
		return
	}

	// Check whitelist (using action-specific whitelist)
	if !isSelfPunish && isUserWhitelistedForAction(targetMember, actionConfig) {
		utils.SendFollowUpError(s, i.Interaction, "This user is on the whitelist and cannot be punished.")
		return
	}

	// Process evidence
	evidenceJSON, allEvidence, err := processEvidence(s, evidenceLinks, targetUser)
	if err != nil {
		log.Printf("Error processing evidence: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to process evidence.")
		return
	}

	if isSelfPunish {
		// For self-punishment, just remove roles and show message
		removePunishmentRoles(s, i.GuildID, targetUser.ID, actionConfig.RemoveRoleID)
		embed := buildPunishmentEmbedNew(i, targetUser, action, reason, allEvidence, nil, nil, false, "", -1)
		sendResponseMessages(s, i, targetUser, embed, false, "", reason)
		return
	}

	// Connect to database using the database path from punish config
	db, err := punishments_db.Init(punishConfig.DatabasePath)
	if err != nil {
		log.Printf("Error connecting to punishment DB: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to connect to the punishment database.")
		return
	}
	defer db.Close()

	// Get total punishment count for this user (all action types)
	punishmentCount, err := punishments_db.GetTotalPunishmentCountByUser(db, i.GuildID, targetUser.ID)
	if err != nil {
		log.Printf("Error getting total punishment count: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to retrieve punishment history.")
		return
	}

	// Determine punishment level based on count
	punishLevel := getPunishmentLevel(actionConfig, punishmentCount)
	if punishLevel == nil {
		// Use the highest available level if count exceeds configured levels
		punishLevel = getHighestPunishmentLevel(actionConfig)
	}

	// Apply punishments according to the level
	timeoutApplied, timeoutDurationStr, tempRoles, rolesRemoveAt := applyPunishmentLevel(s, i, targetUser, *punishLevel)

	// Record the punishment
	punishmentID, err := addPunishmentRecord(db, i, targetUser, reason, evidenceJSON, action, tempRoles, rolesRemoveAt)
	if err != nil {
		log.Printf("Error saving punishment record: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to save the punishment record.")
		return
	}

	// Get history for display
	currentGuildHistory, otherGuildsHistory, err := getPunishmentHistory(db, targetUser.ID, i.GuildID)
	if err != nil {
		log.Printf("Error fetching punishment history: %v", err)
	}
	// Build and send response
	embed := buildPunishmentEmbedNew(i, targetUser, action, reason, allEvidence, currentGuildHistory, otherGuildsHistory, timeoutApplied, timeoutDurationStr, punishmentID)

	// Prepare preset message if configured
	var presetContent string
	var presetEmbeds []*discordgo.MessageEmbed
	if punishLevel.SendPresetID != "" {
		preset := b.FindPresetByID(punishLevel.SendPresetID)
		if preset != nil {
			messageSend := preset_pkg.FormatPresetMessageSend(preset, "")
			presetContent = messageSend.Content
			presetEmbeds = messageSend.Embeds
		} else {
			log.Printf("Preset with ID '%s' not found for punishment.", punishLevel.SendPresetID)
		}
	}

	// Combine punishment embed with preset message for the private message
	privateEmbeds := []*discordgo.MessageEmbed{embed}
	privateEmbeds = append(privateEmbeds, presetEmbeds...)

	// Send combined private message
	if presetContent != "" {
		utils.SendPrivateMessage(s, targetUser.ID, presetContent)
	}
	for _, e := range privateEmbeds {
		utils.SendPrivateEmbedMessage(s, targetUser.ID, e)
	}

	// The public message only contains the original punishment embed.
	_, err = s.ChannelMessageSendEmbed(i.ChannelID, embed)
	if err != nil {
		log.Printf("Error sending public punishment message: %v", err)
	}

	// Log to configured channel
	logPunishmentNew(i, actionConfig, targetUser)
}
