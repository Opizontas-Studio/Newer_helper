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
