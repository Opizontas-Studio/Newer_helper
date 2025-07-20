package admin

import (
	"discord-bot/bot"
	"discord-bot/utils"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// HandleNewPostPushAdminCommand handles the unified admin command for new post push settings.
func HandleNewPostPushAdminCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, nil, nil, b.GetConfig().DeveloperUserIDs, b.GetConfig().SuperAdminRoleIDs)
	if permissionLevel < utils.DeveloperPermission {
		utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
		return
	}

	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
	for _, opt := range i.ApplicationCommandData().Options {
		optionMap[opt.Name] = opt
	}

	action, ok := optionMap["action"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: action option is missing.")
		return
	}

	actionStr := action.StringValue()
	_, inputExists := optionMap["input"]
	_, channelExists := optionMap["channel"]

	switch actionStr {
	case "add_channel", "remove_channel":
		if !inputExists {
			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Error: 'input' option (with Channel ID) is required for action '%s'.", actionStr))
			return
		}
		if actionStr == "add_channel" {
			handleAddChannel(s, i, optionMap)
		} else {
			handleRemoveChannel(s, i, optionMap)
		}
	case "add_whitelist", "remove_whitelist":
		if !inputExists {
			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Error: 'input' option (with Message ID) is required for action '%s'.", actionStr))
			return
		}
		if !channelExists {
			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Error: 'channel' option is required for action '%s'.", actionStr))
			return
		}
		if actionStr == "add_whitelist" {
			handleWhitelistAdd(s, i, optionMap)
		} else {
			handleWhitelistRemove(s, i, optionMap)
		}
	case "list_config":
		handleListConfig(s, i)
	default:
		utils.SendEphemeralResponse(s, i, "Unknown action.")
	}
}

func handleAddChannel(s *discordgo.Session, i *discordgo.InteractionCreate, options map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	input, ok := options["input"]
	if !ok {
		// This check is redundant due to the main handler, but good for safety.
		utils.SendEphemeralResponse(s, i, "Error: input option is missing for add_channel.")
		return
	}
	// The 'input' is a string. We assume it's a channel ID.
	channelID := input.StringValue()
	guildID := i.GuildID

	config, err := utils.LoadNewCardPushConfig(guildID)
	if err != nil {
		log.Printf("Error loading new card push config for guild %s: %v", guildID, err)
		utils.SendEphemeralResponse(s, i, "An error occurred while loading the configuration.")
		return
	}

	for _, id := range config.PushChannelIDs {
		if id == channelID {
			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Channel <#%s> is already in the push list.", channelID))
			return
		}
	}

	config.PushChannelIDs = append(config.PushChannelIDs, channelID)
	if err := utils.SaveNewCardPushConfig(guildID, config); err != nil {
		log.Printf("Error saving config: %v", err)
		utils.SendEphemeralResponse(s, i, "Failed to save configuration.")
		return
	}
	utils.SendEphemeralResponse(s, i, fmt.Sprintf("Successfully added <#%s> to the push list.", channelID))
}

func handleRemoveChannel(s *discordgo.Session, i *discordgo.InteractionCreate, options map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	input, ok := options["input"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: input option is missing for remove_channel.")
		return
	}
	channelID := input.StringValue()
	guildID := i.GuildID

	config, err := utils.LoadNewCardPushConfig(guildID)
	if err != nil {
		log.Printf("Error loading config: %v", err)
		utils.SendEphemeralResponse(s, i, "An error occurred while loading the configuration.")
		return
	}

	var newChannels []string
	found := false
	for _, id := range config.PushChannelIDs {
		if id == channelID {
			found = true
		} else {
			newChannels = append(newChannels, id)
		}
	}

	if !found {
		utils.SendEphemeralResponse(s, i, fmt.Sprintf("Channel <#%s> not found in the push list.", channelID))
		return
	}

	config.PushChannelIDs = newChannels
	if err := utils.SaveNewCardPushConfig(guildID, config); err != nil {
		log.Printf("Error saving config: %v", err)
		utils.SendEphemeralResponse(s, i, "Failed to save configuration.")
		return
	}
	utils.SendEphemeralResponse(s, i, fmt.Sprintf("Successfully removed <#%s> from the push list.", channelID))
}

func handleWhitelistAdd(s *discordgo.Session, i *discordgo.InteractionCreate, options map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	channelOpt, ok := options["channel"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: channel option is missing for add_whitelist.")
		return
	}
	channelID := channelOpt.ChannelValue(s).ID

	input, ok := options["input"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: input option (message ID) is missing for add_whitelist.")
		return
	}
	messageID := input.StringValue()
	guildID := i.GuildID

	config, err := utils.LoadNewCardPushConfig(guildID)
	if err != nil {
		log.Printf("Error loading config: %v", err)
		utils.SendEphemeralResponse(s, i, "An error occurred while loading the configuration.")
		return
	}

	if config.WhitelistedMessages == nil {
		config.WhitelistedMessages = make(map[string][]string)
	}

	for _, id := range config.WhitelistedMessages[channelID] {
		if id == messageID {
			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Message `%s` is already in the whitelist for <#%s>.", messageID, channelID))
			return
		}
	}

	config.WhitelistedMessages[channelID] = append(config.WhitelistedMessages[channelID], messageID)
	if err := utils.SaveNewCardPushConfig(guildID, config); err != nil {
		log.Printf("Error saving config: %v", err)
		utils.SendEphemeralResponse(s, i, "Failed to save configuration.")
		return
	}
	utils.SendEphemeralResponse(s, i, fmt.Sprintf("Successfully added message `%s` to the whitelist for <#%s>.", messageID, channelID))
}

func handleWhitelistRemove(s *discordgo.Session, i *discordgo.InteractionCreate, options map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	channelOpt, ok := options["channel"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: channel option is missing for remove_whitelist.")
		return
	}
	channelID := channelOpt.ChannelValue(s).ID

	input, ok := options["input"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: input option (message ID) is missing for remove_whitelist.")
		return
	}
	messageID := input.StringValue()
	guildID := i.GuildID

	config, err := utils.LoadNewCardPushConfig(guildID)
	if err != nil {
		log.Printf("Error loading config: %v", err)
		utils.SendEphemeralResponse(s, i, "An error occurred while loading the configuration.")
		return
	}

	if _, ok := config.WhitelistedMessages[channelID]; !ok {
		utils.SendEphemeralResponse(s, i, fmt.Sprintf("No whitelist found for channel <#%s>.", channelID))
		return
	}

	var newWhitelist []string
	found := false
	for _, id := range config.WhitelistedMessages[channelID] {
		if id == messageID {
			found = true
		} else {
			newWhitelist = append(newWhitelist, id)
		}
	}

	if !found {
		utils.SendEphemeralResponse(s, i, fmt.Sprintf("Message `%s` not found in the whitelist for <#%s>.", messageID, channelID))
		return
	}

	config.WhitelistedMessages[channelID] = newWhitelist
	if err := utils.SaveNewCardPushConfig(guildID, config); err != nil {
		log.Printf("Error saving config: %v", err)
		utils.SendEphemeralResponse(s, i, "Failed to save configuration.")
		return
	}
	utils.SendEphemeralResponse(s, i, fmt.Sprintf("Successfully removed message `%s` from the whitelist for <#%s>.", messageID, channelID))
}

func handleListConfig(s *discordgo.Session, i *discordgo.InteractionCreate) {
	guildID := i.GuildID
	config, err := utils.LoadNewCardPushConfig(guildID)
	if err != nil {
		log.Printf("Error loading config: %v", err)
		utils.SendEphemeralResponse(s, i, "An error occurred while loading the configuration.")
		return
	}

	var builder strings.Builder
	builder.WriteString("**New Post Push Configuration**\n")

	if len(config.PushChannelIDs) == 0 {
		builder.WriteString("No push channels configured.\n")
	} else {
		builder.WriteString("Push Channels:\n")
		for _, id := range config.PushChannelIDs {
			builder.WriteString(fmt.Sprintf("- <#%s>\n", id))
		}
	}

	if len(config.WhitelistedMessages) == 0 {
		builder.WriteString("No whitelisted messages.\n")
	} else {
		builder.WriteString("Whitelisted Messages:\n")
		for chID, msgIDs := range config.WhitelistedMessages {
			if len(msgIDs) > 0 {
				builder.WriteString(fmt.Sprintf("  - In <#%s>:\n", chID))
				for _, msgID := range msgIDs {
					builder.WriteString(fmt.Sprintf("    - `%s`\n", msgID))
				}
			}
		}
	}

	utils.SendEphemeralResponse(s, i, builder.String())
}
