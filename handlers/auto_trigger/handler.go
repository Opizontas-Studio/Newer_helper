package auto_trigger

import (
	"fmt"
	"log"
	"newer_helper/bot"
	"newer_helper/utils"
	"newer_helper/utils/database"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func extractPresetID(input string) string {
	re := regexp.MustCompile(`^\((.*?)\)`)
	matches := re.FindStringSubmatch(input)
	if len(matches) > 1 {
		return matches[1]
	}
	return strings.TrimSpace(input)
}

func HandleManageAutoTriggerCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		return
	}
	permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
	if permissionLevel < utils.AdminPermission {
		utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	action := optionMap["action"].StringValue()

	switch action {
	case "bind_keyword":
		handleBindKeyword(s, i, b, optionMap)
	case "unbind_keyword":
		handleUnbindKeyword(s, i, b, optionMap)
	// case "add_channel":
	// 	handleAddChannel(s, i, b, optionMap)
	// case "remove_channel":
	// 	handleRemoveChannel(s, i, b, optionMap)
	case "list_config":
		handleListConfig(s, i, b)
	case "overwrite_keyword":
		handleOverwriteKeyword(s, i, b, optionMap)
	}
}

func handleBindKeyword(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	keywordsRaw := optionMap["keyword"].StringValue()
	presetIDRaw := optionMap["id"].StringValue()
	presetID := extractPresetID(presetIDRaw)
	channel := optionMap["channel"].ChannelValue(s)

	if keywordsRaw == "" || presetID == "" || channel == nil {
		utils.SendEphemeralResponse(s, i, "Keywords, preset ID, and channel are required for this action.")
		return
	}

	keywords := strings.Split(keywordsRaw, ",")
	for _, keyword := range keywords {
		trimmedKeyword := strings.TrimSpace(keyword)
		if trimmedKeyword == "" {
			continue
		}
		err := database.AddAutoTrigger(b.GetDB(), i.GuildID, trimmedKeyword, presetID, channel.ID)
		if err != nil {
			log.Printf("Error binding keyword '%s': %v", trimmedKeyword, err)
			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Error binding keyword: `%s`.", trimmedKeyword))
			return
		}
	}

	b.ReloadConfig()
	utils.SendEphemeralResponse(s, i, fmt.Sprintf("Successfully bound keywords `%s` to preset `%s` in channel <#%s>.", keywordsRaw, presetID, channel.ID))
}
func handleUnbindKeyword(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	keywordsRaw := optionMap["keyword"].StringValue()
	channel := optionMap["channel"].ChannelValue(s)

	if keywordsRaw == "" || channel == nil {
		utils.SendEphemeralResponse(s, i, "Keywords and channel are required for this action.")
		return
	}

	keywords := strings.Split(keywordsRaw, ",")
	for _, keyword := range keywords {
		trimmedKeyword := strings.TrimSpace(keyword)
		if trimmedKeyword == "" {
			continue
		}
		err := database.DeleteAutoTrigger(b.GetDB(), i.GuildID, trimmedKeyword, channel.ID)
		if err != nil {
			log.Printf("Error unbinding keyword '%s': %v", trimmedKeyword, err)
			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Error unbinding keyword: `%s`.", trimmedKeyword))
			return
		}
	}

	b.ReloadConfig()
	utils.SendEphemeralResponse(s, i, fmt.Sprintf("Successfully unbound keywords `%s` from channel <#%s>.", keywordsRaw, channel.ID))
}

// func handleAddChannel(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption) {
// 	utils.SendEphemeralResponse(s, i, "This action is deprecated. Please use `bind_keyword` to add a keyword to a channel.")
// }

// func handleRemoveChannel(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption) {
// 	utils.SendEphemeralResponse(s, i, "This action is deprecated. Please use `unbind_keyword` to remove a keyword from a channel.")
// }

func handleListConfig(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	serverConfig, ok := b.GetConfig().ServerConfigs[i.GuildID]
	if !ok || len(serverConfig.AutoTriggers) == 0 {
		utils.SendEphemeralResponse(s, i, "No auto-triggers configured for this server.")
		return
	}

	var response string
	for _, trigger := range serverConfig.AutoTriggers {
		response += fmt.Sprintf("ID: %d, Channel: <#%s>, Keywords: `%s`, Preset ID: `%s`\n", trigger.ID, trigger.ChannelID, strings.Join(trigger.Keywords, ", "), trigger.PresetID)
	}

	if response == "" {
		response = "No auto-triggers configured for this server."
	}

	utils.SendEphemeralResponse(s, i, response)
}

func handleOverwriteKeyword(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	keywordsRaw := optionMap["keyword"].StringValue()
	presetIDRaw := optionMap["id"].StringValue()
	presetID := extractPresetID(presetIDRaw)
	channel := optionMap["channel"].ChannelValue(s)

	if keywordsRaw == "" || presetID == "" || channel == nil {
		utils.SendEphemeralResponse(s, i, "Keywords, preset ID, and channel are required for this action.")
		return
	}

	keywords := strings.Split(keywordsRaw, ",")
	for _, keyword := range keywords {
		trimmedKeyword := strings.TrimSpace(keyword)
		if trimmedKeyword == "" {
			continue
		}
		err := database.OverwriteAutoTrigger(b.GetDB(), i.GuildID, trimmedKeyword, presetID, channel.ID)
		if err != nil {
			log.Printf("Error overwriting keyword '%s': %v", trimmedKeyword, err)
			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Error overwriting keyword: `%s`.", trimmedKeyword))
			return
		}
	}
	b.ReloadConfig()
	utils.SendEphemeralResponse(s, i, fmt.Sprintf("Successfully overwrote keywords `%s` with preset `%s` in channel <#%s>.", keywordsRaw, presetID, channel.ID))
}
