package handlers

import (
	"discord-bot/commands"
	"discord-bot/model"
	"discord-bot/utils"
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
	case "rollcard":
		var focusedOption discordgo.ApplicationCommandInteractionDataOption
		var poolName string
		for _, opt := range data.Options {
			if opt.Focused {
				focusedOption = *opt
			}
			if opt.Name == "pool" {
				poolName = opt.StringValue()
			}
		}

		guildID := i.GuildID
		rollCardGuildConfig, ok := config.RollCardConfigs[guildID]
		if !ok {
			return // or handle error
		}

		switch focusedOption.Name {
		case "pool":
			// Add the static option for all-server roll if it matches the user input or the input is empty
			if strings.Contains(strings.ToLower("全区抽卡"), strings.ToLower(focusedOption.StringValue())) {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  "全区抽卡",
					Value: "all-server-roll",
				})
			}
			for _, name := range rollCardGuildConfig.DataBaseTableNameMapping {
				if strings.Contains(strings.ToLower(name), strings.ToLower(focusedOption.StringValue())) {
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  name,
						Value: name,
					})
				}
			}
		case "tag":
			if poolName == "" {
				// If no pool is selected, we cannot suggest tags.
				return
			}

			// Load the tag mapping file associated with the guild.
			tagMapping, err := utils.LoadTagMapping(rollCardGuildConfig.TagMappingFile)
			if err != nil {
				log.Printf("Error loading tag mapping for guild %s: %v", guildID, err)
				return
			}

			// Filter tags based on the selected pool.
			if poolName == "all-server-roll" {
				// For all-server-roll, show all tags from all categories.
				for _, tags := range tagMapping {
					for id, name := range tags {
						if strings.Contains(strings.ToLower(name), strings.ToLower(focusedOption.StringValue())) {
							choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
								Name:  name,
								Value: id,
							})
						}
					}
				}
			} else if tags, ok := tagMapping[poolName]; ok {
				// For a specific pool, only show tags from that category.
				for id, name := range tags {
					if strings.Contains(strings.ToLower(name), strings.ToLower(focusedOption.StringValue())) {
						choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
							Name:  name,
							Value: id,
						})
					}
				}
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
			var configs map[string]commands.GuildConfig
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
