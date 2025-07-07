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
		if rollCardGuildConfig, ok := config.RollCardConfigs[i.GuildID]; ok {
			for _, poolName := range rollCardGuildConfig.DataBaseTableNameMapping {
				if strings.Contains(strings.ToLower(poolName), strings.ToLower(data.Options[0].StringValue())) {
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  poolName,
						Value: poolName, // Use the user-friendly name as the value
					})
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
