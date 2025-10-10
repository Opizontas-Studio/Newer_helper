package leaderboard

import (
	"log"
	"newer_helper/handlers/leaderboard/lbadmin"
	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"

	"github.com/bwmarrin/discordgo"
)

func HandleAdsBoardAdminCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b model.BotConfigProvider) {
	// Defer the response
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Error sending deferred response: %v", err)
		return
	}

	// Run the logic in a goroutine
	go func() {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		action := optionMap["action"].StringValue()
		var input, adIDStr string
		if opt, ok := optionMap["input"]; ok {
			input = opt.StringValue()
		}
		if opt, ok := optionMap["ad_id"]; ok {
			adIDStr = opt.StringValue()
		}

		db, err := database.InitDB("data/guilds.db")
		if err != nil {
			log.Printf("Failed to connect to database: %v", err)
			utils.SendFollowUpError(s, i.Interaction, "数据库连接失败")
			return
		}
		defer db.Close()

		switch action {
		case "add":
			lbadmin.HandleAddAd(s, i, db, b, input)
		case "delete":
			lbadmin.HandleDeleteAd(s, i, db, b, adIDStr)
		case "list":
			lbadmin.HandleListAds(s, i, db)
		case "toggle":
			lbadmin.HandleToggleAd(s, i, db, b, adIDStr)
		case "modify":
			lbadmin.HandleModifyAd(s, i, db, b, adIDStr, input)
		case "print":
			lbadmin.HandlePrintAd(s, i, db, b, adIDStr)
		}
	}()
}
