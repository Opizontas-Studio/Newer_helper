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
		if idField, ok := optionMap["id"]; ok && idField.Focused {
			userOpt, userOk := optionMap["user"]
			if !userOk {
				return // User must be selected first
			}
			userID := userOpt.UserValue(s).ID

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

			records, err := database.GetPunishmentRecordsByUserID(db, userID, nil)
			if err != nil {
				log.Printf("Autocomplete: failed to get records: %v", err)
				return
			}

			for _, r := range records {
				choiceName := fmt.Sprintf("ID: %d - %s", r.PunishmentID, r.Reason)
				if len(choiceName) > 100 {
					choiceName = choiceName[:97] + "..."
				}
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  choiceName,
					Value: r.PunishmentID,
				})
			}
		}
	case "preset-message", "preset-message_admin":
		if serverConfig, ok := config.ServerConfigs[i.GuildID]; ok {
			for _, p := range serverConfig.PresetMessages {
				if strings.Contains(strings.ToLower(p.Name), strings.ToLower(data.Options[0].StringValue())) {
					name := p.Name
					if len(name) > 80 {
						name = name[:80]
					}
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  fmt.Sprintf("(%s) %s", p.ID[:4], name),
						Value: p.ID,
					})
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
