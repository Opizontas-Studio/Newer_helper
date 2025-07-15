package punish

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

func HandlePunishAdminCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Failed to defer interaction: %v", err)
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	targetUser := optionMap["user"].UserValue(s)
	action := optionMap["action"].StringValue()

	kickConfig, err := utils.LoadKickConfig("data/kick_config.json")
	if err != nil {
		utils.SendErrorResponse(s, i, "Failed to load kick configuration.")
		log.Printf("Error loading kick config: %v", err)
		return
	}

	db, err := database.InitPunishmentDB(kickConfig.InitConfig.DBPath)
	if err != nil {
		utils.SendErrorResponse(s, i, "Failed to connect to the punishment database.")
		log.Printf("Error connecting to punishment DB: %v", err)
		return
	}
	defer db.Close()

	switch action {
	case "search":
		handleSearch(s, i, db, targetUser)
	case "delete":
		handleDelete(s, i, db, optionMap)
	case "revoke":
		handleRevoke(s, i, db, optionMap, kickConfig, b)
	}
}

func handleSearch(s *discordgo.Session, i *discordgo.InteractionCreate, db *sqlx.DB, user *discordgo.User) {
	records, err := database.GetPunishmentRecordsByUserID(db, user.ID, nil)
	if err != nil {
		utils.SendErrorResponse(s, i, "Failed to retrieve punishment records.")
		log.Printf("Error getting punishment records: %v", err)
		return
	}

	if len(records) == 0 {
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &(&struct{ string }{"No punishment records found for this user."}).string,
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: "Punishment Records for " + user.Username,
		Color: 0x00ff00,
	}

	for _, record := range records {
		timestamp := time.Unix(record.Timestamp, 0).Format(time.RFC1123)
		field := &discordgo.MessageEmbedField{
			Name:  fmt.Sprintf("ID: %d on %s", record.PunishmentID, timestamp),
			Value: fmt.Sprintf("Reason: %s\nAdmin: <@%s>", record.Reason, record.AdminID),
		}
		embed.Fields = append(embed.Fields, field)
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

func handleDelete(s *discordgo.Session, i *discordgo.InteractionCreate, db *sqlx.DB, options map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	idOpt, ok := options["id"]
	if !ok {
		utils.SendErrorResponse(s, i, "Punishment ID is required for delete action.")
		return
	}
	punishmentID := idOpt.IntValue()

	err := database.DeletePunishmentRecordByID(db, punishmentID)
	if err != nil {
		utils.SendErrorResponse(s, i, fmt.Sprintf("Failed to delete punishment record: %v", err))
		log.Printf("Error deleting punishment record: %v", err)
		return
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &(&struct{ string }{fmt.Sprintf("Successfully deleted punishment record with ID: %d", punishmentID)}).string,
	})
}

func handleRevoke(s *discordgo.Session, i *discordgo.InteractionCreate, db *sqlx.DB, options map[string]*discordgo.ApplicationCommandInteractionDataOption, config *model.KickConfig, b *bot.Bot) {
	idOpt, ok := options["id"]
	if !ok {
		utils.SendErrorResponse(s, i, "Punishment ID is required for revoke action.")
		return
	}
	punishmentID := idOpt.IntValue()

	record, err := database.GetPunishmentRecordByID(db, punishmentID)
	if err != nil {
		utils.SendErrorResponse(s, i, fmt.Sprintf("Failed to find punishment record: %v", err))
		return
	}

	guildConfig, ok := config.InitConfig.Data[record.GuildID]
	if !ok {
		utils.SendErrorResponse(s, i, "Could not find server configuration for this punishment.")
		return
	}

	// Restore roles
	for _, roleID := range guildConfig.RemoveRoleID {
		err := s.GuildMemberRoleAdd(record.GuildID, record.UserID, roleID)
		if err != nil {
			log.Printf("Failed to restore role %s for user %s: %v", roleID, record.UserID, err)
		}
	}

	// Remove timeout
	err = s.GuildMemberTimeout(record.GuildID, record.UserID, nil)
	if err != nil {
		log.Printf("Failed to remove timeout for user %s: %v", record.UserID, err)
	}

	// Delete the punishment record
	err = database.DeletePunishmentRecordByID(db, punishmentID)
	if err != nil {
		utils.SendErrorResponse(s, i, fmt.Sprintf("Failed to delete punishment record after revoking: %v", err))
		return
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &(&struct{ string }{fmt.Sprintf("Successfully revoked and deleted punishment record with ID: %d", punishmentID)}).string,
	})
}
