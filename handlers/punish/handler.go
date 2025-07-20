package punish

import (
	"discord-bot/bot"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"log"

	"github.com/bwmarrin/discordgo"
)

func HandlePunishCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	// 1. Defer initial response
	if err := utils.DeferResponse(s, i, true); err != nil {
		log.Printf("Failed to defer interaction: %v", err)
		return
	}

	// 2. Load configuration
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

	// 3. Parse command options
	cmdOptions := parsePunishOptions(s, i)
	targetUser := cmdOptions.TargetUser
	reason := cmdOptions.Reason

	// 4. Get member details and check whitelist
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

	// 5. Initial punishment action: remove roles
	removePunishmentRoles(s, i.GuildID, targetUser.ID, configEntry.RemoveRoleID)

	// 6. Process evidence
	evidenceJSON, allEvidence, err := processEvidence(s, cmdOptions.MessageLinks, targetUser)
	if err != nil {
		log.Printf("Error processing evidence: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to process evidence.")
		return
	}

	// 7. Connect to the database
	db, err := database.InitPunishmentDB(kickConfig.InitConfig.DBPath)
	if err != nil {
		log.Printf("Error connecting to punishment DB: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to connect to the punishment database.")
		return
	}
	defer db.Close()

	// 8. Apply timeout logic if applicable
	timeoutApplied, timeoutDurationStr, err := applyTimeoutIfRequired(s, i, db, kickConfig, configEntry, targetUser)
	if err != nil {
		log.Printf("Error applying timeout: %v", err)
		// Non-fatal, just log the error and continue
	}

	// 9. Add punishment record to the database
	punishmentID, err := addPunishmentRecord(db, i, targetUser, reason, evidenceJSON)
	if err != nil {
		log.Printf("Error saving punishment record: %v", err)
		utils.SendFollowUpError(s, i.Interaction, "Failed to save the punishment record.")
		return
	}

	// 10. Get punishment history for display
	currentGuildHistory, otherGuildsHistory, err := getPunishmentHistory(db, targetUser.ID, i.GuildID)
	if err != nil {
		log.Printf("Error fetching punishment history: %v", err)
		// Non-fatal, just log the error and continue
	}

	// 11. Build and send responses
	embed := buildPunishmentEmbed(i, targetUser, reason, allEvidence, currentGuildHistory, otherGuildsHistory, kickConfig, timeoutApplied, timeoutDurationStr, punishmentID)
	punishmentMessage := sendResponseMessages(s, i, targetUser, embed, timeoutApplied, timeoutDurationStr, reason)

	// 12. Log the punishment
	logPunishment(s, i, configEntry, targetUser, cmdOptions.MessageLinks, punishmentMessage, timeoutApplied, timeoutDurationStr)
}
