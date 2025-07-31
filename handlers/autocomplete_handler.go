package handlers

import (
	"discord-bot/model"
	"discord-bot/scanner"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, config *model.Config) {
	data := i.ApplicationCommandData()
	var choices []*discordgo.ApplicationCommandOptionChoice

	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(data.Options))
	for _, opt := range data.Options {
		optionMap[opt.Name] = opt
	}

	switch data.Name {
	case "punish_admin":
		inputField, inputOk := optionMap["input"]
		searchByOpt, searchByOk := optionMap["search_by"]

		if inputOk && inputField.Focused && searchByOk {
			searchBy := searchByOpt.StringValue()
			inputValue := inputField.StringValue()

			if searchBy == "punishment_id" {
				kickConfig, err := utils.LoadKickConfig("data/kick_config.json")
				if err != nil {
					log.Printf("Autocomplete: failed to load kick config: %v", err)
					return
				}
				db, err := database.InitPunishmentDB(kickConfig.InitConfig.DBPath)
				if err != nil {
					log.Printf("Autocomplete: failed to connect to db: %v", err)
					return
				}
				defer db.Close()

				records, err := database.GetAllPunishmentRecords(db, i.GuildID)
				if err != nil {
					log.Printf("Autocomplete: failed to get records: %v", err)
					return
				}

				for _, r := range records {
					// Fuzzy match against reason, user ID or username
					matchReason := strings.Contains(strings.ToLower(r.Reason), strings.ToLower(inputValue))
					matchUserID := strings.Contains(r.UserID, inputValue)
					matchUsername := strings.Contains(strings.ToLower(r.UserUsername), strings.ToLower(inputValue))

					if matchReason || matchUserID || matchUsername {
						choiceName := fmt.Sprintf("ID: %d (%s) - %s", r.PunishmentID, r.UserUsername, r.Reason)
						if len(choiceName) > 100 {
							choiceName = choiceName[:97] + "..."
						}
						choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
							Name:  choiceName,
							Value: fmt.Sprintf("%d", r.PunishmentID),
						})
					}
				}
			}
		}
	case "ads_board_admin":
		var focusedOption *discordgo.ApplicationCommandInteractionDataOption
		for _, opt := range data.Options {
			if opt.Focused {
				focusedOption = opt
				break
			}
		}

		if focusedOption != nil && focusedOption.Name == "ad_id" {
			// Temporarily remove action check to debug if action value is missing in payload
			db, err := database.InitDB("data/guilds.db")
			if err != nil {
				log.Printf("Autocomplete: failed to connect to db: %v", err)
				return
			}
			defer db.Close()

			ads, err := database.ListLeaderboardAds(db, i.GuildID)
			if err != nil {
				log.Printf("Autocomplete: failed to list ads: %v", err)
				return
			}

			inputValue := focusedOption.StringValue()
			for _, ad := range ads {
				adIDStr := fmt.Sprintf("%d", ad.ID)
				// Fuzzy match against ad content or ID
				matchContent := strings.Contains(strings.ToLower(ad.Content), strings.ToLower(inputValue))
				matchID := strings.Contains(adIDStr, inputValue)

				if matchContent || matchID {
					choiceName := fmt.Sprintf("ID: %d - %s", ad.ID, ad.Content)
					if len(choiceName) > 100 {
						choiceName = choiceName[:97] + "..."
					}
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  choiceName,
						Value: adIDStr,
					})
				}
			}
		}
	case "preset-message", "preset-message_admin", "manage-auto-trigger":
		var focusedOption *discordgo.ApplicationCommandInteractionDataOption
		for _, opt := range data.Options {
			if opt.Focused {
				focusedOption = opt
				break
			}
		}

		if focusedOption != nil && focusedOption.Name == "id" {
			if serverConfig, ok := config.ServerConfigs[i.GuildID]; ok {
				for _, p := range serverConfig.PresetMessages {
					if strings.Contains(strings.ToLower(p.Name), strings.ToLower(focusedOption.StringValue())) {
						name := p.Name
						if len(name) > 80 {
							name = name[:80]
						}
						choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
							Name:  fmt.Sprintf("(%s) %s", p.ID, name),
							Value: p.ID,
						})
					}
				}
			}
		}
	case "start-scan":
		var focusedOption *discordgo.ApplicationCommandInteractionDataOption
		for _, option := range data.Options {
			if option.Focused {
				focusedOption = option
				break
			}
		}

		if focusedOption != nil && focusedOption.Name == "guild" {
			file, err := os.ReadFile("data/task_config.json")
			if err != nil {
				log.Printf("Error reading task_config.json for autocomplete: %v", err)
				return
			}
			var configs map[string]scanner.GuildConfig
			if err := json.Unmarshal(file, &configs); err != nil {
				log.Printf("Error unmarshalling task_config.json for autocomplete: %v", err)
				return
			}

			inputValue := focusedOption.Value.(string)
			for id, config := range configs {
				if strings.Contains(strings.ToLower(config.Name), strings.ToLower(inputValue)) || strings.Contains(strings.ToLower(id), strings.ToLower(inputValue)) {
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  fmt.Sprintf("%s (%s)", config.Name, id),
						Value: id,
					})
				}
			}
		}
	case "guilds_admin":
		var focusedOption *discordgo.ApplicationCommandInteractionDataOption
		for _, option := range data.Options {
			if option.Focused {
				focusedOption = option
				break
			}
		}

		if focusedOption != nil && focusedOption.Name == "guild" {
			db, err := database.InitDB("data/guilds.db")
			if err != nil {
				log.Printf("Autocomplete: failed to connect to db: %v", err)
				return
			}
			defer db.Close()

			guilds, err := database.GetAllGuilds(db)
			if err != nil {
				log.Printf("Autocomplete: failed to get guilds: %v", err)
				return
			}

			inputValue := focusedOption.Value.(string)
			for _, guild := range guilds {
				if strings.Contains(strings.ToLower(guild.Name), strings.ToLower(inputValue)) || strings.Contains(guild.GuildID, inputValue) {
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  fmt.Sprintf("%s (%s)", guild.Name, guild.GuildID),
						Value: guild.GuildID,
					})
				}
			}
		}
	}

	if len(choices) > 25 {
		choices = choices[:25]
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
	if err != nil {
		log.Printf("Error responding to autocomplete: %v", err)
	}
}
