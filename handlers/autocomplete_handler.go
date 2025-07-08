package handlers

import (
	"discord-bot/model"
	"discord-bot/scanner"
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

	switch data.Name {
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
	case "start_scan":
		var focusedOption discordgo.ApplicationCommandInteractionDataOption
		for _, opt := range data.Options {
			if opt.Focused {
				focusedOption = *opt
				break
			}
		}

		if focusedOption.Name == "guild" {
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

			for id, config := range configs {
				// Match against both name and ID
				if strings.Contains(strings.ToLower(config.Name), strings.ToLower(focusedOption.StringValue())) || strings.Contains(strings.ToLower(id), strings.ToLower(focusedOption.StringValue())) {
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

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}
