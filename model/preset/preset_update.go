package preset

import (
	"crypto/rand"
	"discord-bot/model"
	"discord-bot/utils"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func generateUniqueID() string {
	bytes := make([]byte, 8) // 16 characters
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%x", os.Getpid())
	}
	return hex.EncodeToString(bytes)
}

func ParseMessageLinks(s *discordgo.Session, messageLinks string) ([]string, error) {
	re := regexp.MustCompile(`https://discord.com/channels/(\d+)/(\d+)/(\d+)`)
	matches := re.FindAllStringSubmatch(messageLinks, -1)

	var messages []string
	for _, match := range matches {
		if len(match) == 4 {
			channelID := match[2]
			messageID := match[3]
			msg, err := s.ChannelMessage(channelID, messageID)
			if err != nil {
				return nil, fmt.Errorf("error fetching message %s: %w", match[0], err)
			}
			messages = append(messages, msg.Content)
		}
	}
	return messages, nil
}

func HandlePresetMessageUpdateInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b interface{}) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Error sending deferred response: %v", err)
		return
	}

	go func() {
		type bot interface {
			GetConfig() *model.Config
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

		messages, err := ParseMessageLinks(s, messageLinks)
		if err != nil {
			errorContent := err.Error()
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &errorContent,
			})
			return
		}

		var presetName string
		if len(messages) > 0 {
			presetName = customName
			if presetName == "" {
				presetName = fmt.Sprintf("New Preset %d", len(serverConfig.PresetMessages)+1)
			}
			newPreset := model.PresetMessage{
				ID:    generateUniqueID(),
				Name:  presetName,
				Value: strings.Join(messages, "\n"),
				Type:  "text",
			}
			serverConfig.PresetMessages = append(serverConfig.PresetMessages, newPreset)
			appBot.GetConfig().ServerConfigs[i.GuildID] = serverConfig

			if err := model.SaveConfig(appBot.GetConfig()); err != nil {
				log.Printf("Error saving config: %v", err)
				errorContent := "Error processing preset: could not save config."
				s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &errorContent,
				})
				return
			}

			// Log the successful preset update
			channelLink := fmt.Sprintf("https://discord.com/channels/%s/%s", i.GuildID, i.ChannelID)
			logInfo := fmt.Sprintf("用户 <@%s> 创建了新的预设 `%s`\n[在频道中查看](%s)", i.Member.User.Username, presetName, channelLink)
			if appBot.GetConfig().LogChannelID != "" {
				err = utils.LogInfo(s, appBot.GetConfig().LogChannelID, "预设", "创建/更新", logInfo)
				if err != nil {
					log.Printf("Failed to send log: %v", err)
				}
			}
		}

		appBot.RefreshCommands(i.GuildID)

		var webhookEdit discordgo.WebhookEdit
		if len(messages) == 0 {
			response := "未找到或解析任何消息链接。没有预设被创建或更新。"
			webhookEdit = discordgo.WebhookEdit{
				Content: &response,
			}
		} else {
			description := fmt.Sprintf(
				"已成功为您保存预设 `%s`。\n\n**预设内容预览:**\n```\n%s\n```",
				presetName,
				strings.Join(messages, "\n---\n"),
			)
			embed := &discordgo.MessageEmbed{
				Title:       "✅ 预设创建/更新成功",
				Description: description,
				Color:       0x57F287, // Green
				Footer: &discordgo.MessageEmbedFooter{
					Text: fmt.Sprintf("由 %s 操作", i.Member.User.Username),
				},
			}
			webhookEdit = discordgo.WebhookEdit{
				Embeds: &[]*discordgo.MessageEmbed{embed},
			}
		}

		s.InteractionResponseEdit(i.Interaction, &webhookEdit)
	}()
}
