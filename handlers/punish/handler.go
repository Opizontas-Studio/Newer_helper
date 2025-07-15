package punish

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Evidence holds the content and attachments of a message.
type Evidence struct {
	Content     string   `json:"content"`
	Attachments []string `json:"attachments"`
}

func HandlePunishCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	// Defer the response
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Failed to defer interaction: %v", err)
		return
	}

	var timeoutApplied bool
	var timeoutDurationStr string
	kickConfig, err := utils.LoadKickConfig("data/kick_config.json")
	if err != nil {
		errorMessage := "Failed to load kick configuration."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errorMessage,
		})
		log.Printf("Error loading kick config: %v", err)
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	targetUser := optionMap["user"].UserValue(s)
	reason := optionMap["reason"].StringValue()

	// Process message links for evidence
	var evidenceRecord model.PunishmentRecord
	var allEvidence []Evidence

	if messageLinksOpt, ok := optionMap["message_links"]; ok {
		messageLinks := messageLinksOpt.StringValue()
		links := strings.Fields(messageLinks)
		linkRegex := regexp.MustCompile(`https://discord.com/channels/(\d+)/(\d+)/(\d+)`)

		for _, link := range links {
			matches := linkRegex.FindStringSubmatch(link)
			if len(matches) != 4 {
				log.Printf("Invalid Discord message link format: %s", link)
				continue
			}
			_, channelID, messageID := matches[1], matches[2], matches[3]

			msg, err := s.ChannelMessage(channelID, messageID)
			if err != nil {
				log.Printf("Failed to fetch message %s: %v", messageID, err)
				continue
			}

			var downloadedAttachments []string
			for _, attachment := range msg.Attachments {
				fileName := fmt.Sprintf("%s-%s", attachment.ID, attachment.Filename)
				filePath, err := utils.DownloadFile(attachment.URL, filepath.Join("data", "evidence", targetUser.ID), fileName)
				if err != nil {
					log.Printf("Failed to download attachment %s: %v", attachment.URL, err)
					continue
				}
				downloadedAttachments = append(downloadedAttachments, filePath)
			}

			allEvidence = append(allEvidence, Evidence{
				Content:     msg.Content,
				Attachments: downloadedAttachments,
			})
		}
	}

	evidenceJSON, err := json.Marshal(allEvidence)
	if err != nil {
		errorMessage := "Failed to serialize evidence."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errorMessage,
		})
		log.Printf("Error marshalling evidence: %v", err)
		return
	}
	evidenceRecord.Evidence = string(evidenceJSON)

	configEntry, ok := kickConfig.InitConfig.Data[i.GuildID]
	if !ok {
		errorMessage := "❓ 此服务器未找到可用配置文件"
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errorMessage,
		})
		return
	}

	targetMember, err := s.GuildMember(i.GuildID, targetUser.ID)
	if err != nil {
		errorMessage := "Could not retrieve member details."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errorMessage,
		})
		log.Printf("Error getting member details: %v", err)
		return
	}

	for _, whitelistRole := range configEntry.WhitelistRoleID {
		for _, userRole := range targetMember.Roles {
			if userRole == whitelistRole {
				errorMessage := "This user is on the whitelist and cannot be punished."
				s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &errorMessage,
				})
				return
			}
		}
	}

	for _, roleID := range configEntry.RemoveRoleID {
		err := s.GuildMemberRoleRemove(i.GuildID, targetUser.ID, roleID)
		if err != nil {
			log.Printf("Failed to remove role %s from user %s: %v", roleID, targetUser.ID, err)
		}
	}

	// Connect to database first for checking history
	db, err := database.InitPunishmentDB(kickConfig.InitConfig.DBPath)
	if err != nil {
		errorMessage := "Failed to connect to the punishment database."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errorMessage,
		})
		log.Printf("Error connecting to punishment DB: %v", err)
		return
	}
	defer db.Close()

	// Timeout logic starts here - check BEFORE adding new record
	if configEntry.Timeout.Frequency > 0 {
		if configEntry.Timeout.Time != "" {
			duration, err := utils.ParseDuration(configEntry.Timeout.Time)
			if err != nil {
				log.Printf("Error parsing timeout duration: %v", err)
			} else {
				since := time.Now().Add(-duration)
				recentHistory, err := database.GetPunishmentRecordsByUserID(db, targetUser.ID, &since)
				if err != nil {
					log.Printf("Error fetching recent punishment history: %v", err)
				} else {
					if len(recentHistory) >= configEntry.Timeout.Frequency {
						// Apply timeout
						if configEntry.Timeout.TimeoutTime != "" {
							timeoutDuration, err := utils.ParseDuration(configEntry.Timeout.TimeoutTime)
							if err != nil {
								log.Printf("Error parsing timeout_time: %v", err)
							} else {
								timeoutUntil := time.Now().Add(timeoutDuration)
								err := s.GuildMemberTimeout(i.GuildID, targetUser.ID, &timeoutUntil)
								if err != nil {
									log.Printf("Failed to timeout user %s: %v", targetUser.ID, err)
								} else {
									timeoutApplied = true
									timeoutDurationStr = configEntry.Timeout.TimeoutTime
									log.Printf("Successfully timed out user %s for %s", targetUser.ID, timeoutDurationStr)
								}
							}
						}

						// Add roles and schedule their removal
						if configEntry.Timeout.AddRoleTimeoutTime != "" {
							removalDuration, err := utils.ParseDuration(configEntry.Timeout.AddRoleTimeoutTime)
							if err != nil {
								log.Printf("Error parsing add_role_timeout_time: %v", err)
							} else {
								removeAt := time.Now().Add(removalDuration)
								timedTaskDB, err := database.InitTimedTaskDB(kickConfig.InitConfig.DBPath)
								if err != nil {
									log.Printf("Failed to connect to timed task DB for scheduling: %v", err)
								} else {
									defer timedTaskDB.Close()
									for _, roleID := range configEntry.Timeout.AddRole {
										err := s.GuildMemberRoleAdd(i.GuildID, targetUser.ID, roleID)
										if err != nil {
											log.Printf("Failed to add role %s to user %s: %v", roleID, targetUser.ID, err)
										} else {
											task := model.TimedTask{
												GuildID:  i.GuildID,
												UserID:   targetUser.ID,
												RoleID:   roleID,
												RemoveAt: removeAt,
											}
											if err := database.AddTimedTask(timedTaskDB, task); err != nil {
												log.Printf("Failed to schedule role removal for user %s, role %s: %v", targetUser.ID, roleID, err)
											}
										}
									}
								}
							}
						} else {
							// Fallback to just adding roles without scheduling removal
							for _, roleID := range configEntry.Timeout.AddRole {
								err := s.GuildMemberRoleAdd(i.GuildID, targetUser.ID, roleID)
								if err != nil {
									log.Printf("Failed to add role %s to user %s: %v", roleID, targetUser.ID, err)
								}
							}
						}
					}
				}
			}
		}
	}

	// Add the new punishment record to database AFTER checking history
	record := model.PunishmentRecord{
		MessageID:    i.ID,
		AdminID:      i.Member.User.ID,
		UserID:       targetUser.ID,
		UserUsername: targetUser.Username,
		Reason:       reason,
		GuildID:      i.GuildID,
		Timestamp:    time.Now().Unix(),
		Evidence:     evidenceRecord.Evidence,
	}

	if err := database.AddPunishmentRecord(db, record); err != nil {
		errorMessage := "Failed to save the punishment record."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errorMessage,
		})
		log.Printf("Error saving punishment record: %v", err)
		return
	}

	// Get updated history including the new record for display purposes
	history, err := database.GetPunishmentRecordsByUserID(db, targetUser.ID, nil)
	if err != nil {
		log.Printf("Error fetching punishment history: %v", err)
		// Decide if you want to send a message without history or an error
	}

	embed := &discordgo.MessageEmbed{
		Title: "用户惩罚",
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: targetUser.AvatarURL(""),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "用户",
				Value: targetUser.Mention(),
			},
			{
				Name:  "原因",
				Value: reason,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("由 %s 操作", i.Member.User.Username),
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Color:     0xff0000,
	}

	if len(allEvidence) > 0 {
		var evidenceDetails string
		for _, ev := range allEvidence {
			evidenceDetails += ev.Content + "\n"
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "已存档证据",
			Value: fmt.Sprintf("已保存 %d 条消息作为证据。", len(allEvidence)),
		})
	}

	if len(history) > 0 {
		var historyValue string
		for _, rec := range history {
			historyValue += fmt.Sprintf("操作人: <@%s>, 原因: %s\n", rec.AdminID, rec.Reason)
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "历史处罚记录",
			Value: historyValue,
		})
	}

	// Edit the original deferred response
	responseMessage := "✅ 惩罚指令已成功执行。"
	if timeoutApplied {
		timeoutMessage := fmt.Sprintf("用户 %s 已被禁言，时长为 %s。", targetUser.Username, timeoutDurationStr)
		responseMessage = fmt.Sprintf("%s\n%s", responseMessage, timeoutMessage)
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &responseMessage,
	})

	if timeoutApplied {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "自动禁言",
			Value: fmt.Sprintf("该用户已被自动禁言，时长为: %s", timeoutDurationStr),
		})
	}

	if configEntry.Timeout.Frequency <= 0 {
		embed.Footer.Text += "\n此服务器已禁用自动禁言"
	}

	// Then, send the detailed punishment embed as a public message to the channel
	punishmentMessage, err := s.ChannelMessageSendEmbed(i.ChannelID, embed)
	if err != nil {
		log.Printf("Failed to send punishment embed to channel %s: %v", i.ChannelID, err)
	}

	// Logging the punishment
	if configEntry.LogChannelID != "" {
		var logDetails strings.Builder
		logDetails.WriteString(fmt.Sprintf("执行人: %s (`%s`)\n", i.Member.User.Username, i.Member.User.ID))
		logDetails.WriteString(fmt.Sprintf("被处罚用户: %s (`%s`)\n", targetUser.Username, targetUser.ID))

		timeoutStatus := "否"
		if timeoutApplied {
			timeoutStatus = fmt.Sprintf("是 (时长: %s)", timeoutDurationStr)
		}
		logDetails.WriteString(fmt.Sprintf("是否禁言: %s\n", timeoutStatus))

		if messageLinksOpt, ok := optionMap["message_links"]; ok {
			logDetails.WriteString(fmt.Sprintf("证据链接: %s\n", messageLinksOpt.StringValue()))
		}

		if punishmentMessage != nil {
			punishmentLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, i.ChannelID, punishmentMessage.ID)
			logDetails.WriteString(fmt.Sprintf("处罚消息链接: %s\n", punishmentLink))
		}

		err := utils.LogInfo(s, configEntry.LogChannelID, "处罚模块", "执行处罚", logDetails.String())
		if err != nil {
			log.Printf("Failed to send punish log: %v", err)
		}
	}
}
