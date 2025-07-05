package model

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"discord-bot/utils"

	"github.com/bwmarrin/discordgo"
)

func HandlePresetMessageUpdateInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b interface{}) {
	type bot interface {
		GetConfig() *Config
		RefreshCommands(guildID string)
	}

	appBot := b.(bot)
	serverConfig, ok := appBot.GetConfig().ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", i.GuildID)
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var messageLinks string
	if option, ok := optionMap["messagelinks"]; ok {
		messageLinks = option.StringValue()
	}

	var customName string
	if option, ok := optionMap["name"]; ok {
		customName = option.StringValue()
	}

	// Regex to find discord message links
	re := regexp.MustCompile(`https://discord.com/channels/(\d+)/(\d+)/(\d+)`)
	matches := re.FindAllStringSubmatch(messageLinks, -1)

	var messages []string
	for _, match := range matches {
		if len(match) == 4 {
			channelID := match[2]
			messageID := match[3]
			msg, err := s.ChannelMessage(channelID, messageID)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Error fetching message %s: %s", match[0], err.Error()),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			messages = append(messages, msg.Content)
		}
	}

	if len(messages) > 0 {
		presetName := customName
		if presetName == "" {
			presetName = fmt.Sprintf("New Preset %d", len(serverConfig.PresetMessages)+1)
		}
		newPreset := PresetMessage{
			Name:  presetName,
			Value: strings.Join(messages, "\n"),
			Type:  "text",
		}
		serverConfig.PresetMessages = append(serverConfig.PresetMessages, newPreset)
		appBot.GetConfig().ServerConfigs[i.GuildID] = serverConfig

		file, err := json.MarshalIndent(appBot.GetConfig().ServerConfigs, "", "  ")
		if err != nil {
			log.Printf("Error marshalling config: %v", err)
			return
		}

		err = os.WriteFile("messages.json", file, 0644)
		if err != nil {
			log.Printf("Error writing to messages.json: %v", err)
			return
		}

		// Log the successful preset update
		channelLink := fmt.Sprintf("https://discord.com/channels/%s/%s", i.GuildID, i.ChannelID)
		logInfo := fmt.Sprintf("用户 `%s` 创建/更新了预设 `%s`\n[在频道中查看](%s)", i.Member.User.Username, presetName, channelLink)
		err = utils.LogInfo(appBot.GetConfig().LogWebhookURL, "预设", "创建/更新", logInfo)
		if err != nil {
			log.Printf("Failed to send log: %v", err)
		}
	}

	appBot.RefreshCommands(i.GuildID)

	response := "这是保存的预设和文件:\n" + strings.Join(messages, "\n---\n")

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
