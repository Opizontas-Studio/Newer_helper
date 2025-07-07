package handlers

import (
	"discord-bot/bot"
	"discord-bot/commands"
	"discord-bot/model"
	"discord-bot/model/preset"
	"discord-bot/utils"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func Register(b *bot.Bot) {
	b.CommandHandlers = commandHandlers(b)
	addHandlers(b)
}

func commandHandlers(b *bot.Bot) map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"rollcard": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			HandleRollCardInteraction(s, i, b)
		},
		"preset-message": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.Config.ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs)
			if permissionLevel == utils.GuestPermission {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "You do not have permission to use this command.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			preset.HandlePresetMessageInteraction(s, i, b)
		},
		"preset-message_upd": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.Config.ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs)
			if permissionLevel != utils.AdminPermission {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "You do not have permission to use this command.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			preset.HandlePresetMessageUpdateInteraction(s, i, b)
		},
		"preset-message_admin": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			serverConfig, ok := b.Config.ServerConfigs[i.GuildID]
			if !ok {
				log.Printf("Could not find server config for guild: %s", i.GuildID)
				return
			}
			permissionLevel := utils.CheckPermission(i.Member.Roles, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs)
			if permissionLevel != utils.AdminPermission {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "You do not have permission to use this command.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			preset.HandlePresetMessageAdminInteraction(s, i, b)
		},
	}
}

func addHandlers(b *bot.Bot) {
	b.Session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
		if b.Config.LogChannelID != "" {
			err := utils.LogInfo(s, b.Config.LogChannelID, "System", "启动", "Bot has started successfully.")
			if err != nil {
				log.Printf("Failed to send startup log: %v", err)
			}
		}
	})
	b.Session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			if h, ok := b.CommandHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i)
			}
		case discordgo.InteractionMessageComponent:
			if strings.HasPrefix(i.MessageComponentData().CustomID, "confirm_delete_") || strings.HasPrefix(i.MessageComponentData().CustomID, "cancel_delete_") {
				commands.HandlePresetDeleteInteraction(s, i, b)
			}
		case discordgo.InteractionApplicationCommandAutocomplete:
			handleAutocomplete(s, i, b.Config)
		}
	})
	b.Session.AddHandler(func(s *discordgo.Session, t *discordgo.ThreadCreate) {
		// Pass the bot's config to the handler
		commands.HandleThreadCreate(s, t, b.Config)
	})
}

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
			tagMapping, err := loadTagMapping(rollCardGuildConfig.TagMappingFile)
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
