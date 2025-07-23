package punish

import (
	"discord-bot/bot"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"strings"

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

	applyAndLogPunishment(s, i, cmdOptions.TargetUser, reason, cmdOptions.MessageLinks)
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
	if err := utils.DeferResponse(s, i, true); err != nil {
		log.Printf("Failed to defer interaction: %v", err)
		return
	}

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
	applyAndLogPunishment(s, i, targetMessage.Author, reason, messageLink)
}

// applyAndLogPunishment is the core function that handles the punishment process.
// It centralizes the logic for configuration loading, validation, database operations, and notifications.
func applyAndLogPunishment(s *discordgo.Session, i *discordgo.InteractionCreate, targetUser *discordgo.User, reason, evidenceLinks string) {
	if !utils.CheckAndSetPunishLock(targetUser.ID) {
		utils.SendFollowUpError(s, i.Interaction, "对该用户的处罚操作过于频繁，请 5 分钟后再试。")
		return
	}

	kickConfig, err := utils.LoadKickConfig("data/kick_config.json")
	if err != nil {
		log.Printf("Error loading kick config: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to load kick configuration.")
		return
	}
	configEntry, ok := kickConfig.Data[i.GuildID]
	if !ok {
		utils.SendFollowUpError(s, i.Interaction, "❓ 此服务器未找到可用配置文件")
		return
	}

	targetMember, err := s.GuildMember(i.GuildID, targetUser.ID)
	if err != nil {
		log.Printf("Error getting member details: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Could not retrieve member details.")
		return
	}
	if isUserWhitelisted(targetMember, configEntry) {
		utils.SendFollowUpError(s, i.Interaction, "This user is on the whitelist and cannot be punished.")
		return
	}

	removePunishmentRoles(s, i.GuildID, targetUser.ID, configEntry.RemoveRoleID)

	evidenceJSON, allEvidence, err := processEvidence(s, evidenceLinks, targetUser)
	if err != nil {
		log.Printf("Error processing evidence: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to process evidence.")
		return
	}

	db, err := database.InitPunishmentDB(kickConfig.InitConfig.DBPath)
	if err != nil {
		log.Printf("Error connecting to punishment DB: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to connect to the punishment database.")
		return
	}
	defer db.Close()

	timeoutApplied, timeoutDurationStr, err := applyTimeoutIfRequired(s, i, db, kickConfig, configEntry, targetUser)
	if err != nil {
		log.Printf("Error applying timeout: %v", err)
	}

	punishmentID, err := addPunishmentRecord(db, i, targetUser, reason, evidenceJSON)
	if err != nil {
		log.Printf("Error saving punishment record: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to save the punishment record.")
		return
	}

	currentGuildHistory, otherGuildsHistory, err := getPunishmentHistory(db, targetUser.ID, i.GuildID)
	if err != nil {
		log.Printf("Error fetching punishment history: %v", err)
	}

	embed := buildPunishmentEmbed(i, targetUser, reason, allEvidence, currentGuildHistory, otherGuildsHistory, kickConfig, timeoutApplied, timeoutDurationStr, punishmentID)
	punishmentMessage := sendResponseMessages(s, i, targetUser, embed, timeoutApplied, timeoutDurationStr, reason)

	logPunishment(s, i, configEntry, targetUser, evidenceLinks, punishmentMessage, timeoutApplied, timeoutDurationStr)
}
