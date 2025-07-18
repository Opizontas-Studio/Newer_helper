package punish

import (
	"bytes"
	"discord-bot/bot"
	"discord-bot/internal/repository"
	"discord-bot/model"
	"discord-bot/utils"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const recordsPerPageV2 = 5

// HandlePunishAdminCommandV2 is the new handler for the /punish_admin command.
func HandlePunishAdminCommandV2(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
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

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	searchBy := optionMap["search_by"].StringValue()
	input := optionMap["input"].StringValue()
	var action string
	if actionOpt, ok := optionMap["action"]; ok {
		action = actionOpt.StringValue()
	}

	punishmentRepo := b.GetRepositoryManager().PunishmentRepository()
	if punishmentRepo == nil {
		utils.SendFollowUpError(s, i.Interaction, "数据库服务暂时不可用")
		return
	}

	if action != "" {
		handleActionV2(s, i, searchBy, input, action, punishmentRepo)
	} else {
		displayPunishmentsV2(s, i.Interaction, searchBy, input, 1, punishmentRepo)
	}
}

func handleActionV2(s *discordgo.Session, i *discordgo.InteractionCreate, searchBy, input, action string, repo repository.PunishmentRepository) {
	var record *model.PunishmentRecord
	var err error

	switch searchBy {
	case "punishment_id":
		id, convErr := strconv.ParseInt(input, 10, 64)
		if convErr != nil {
			utils.SendFollowUpError(s, i.Interaction, "无效的惩罚ID。")
			return
		}
		record, err = repo.GetByID(id)
	// Note: mute_db_id action logic needs to be refactored to use TimedTaskRepository
	// This part is left as a placeholder for future refactoring.
	case "mute_db_id":
		utils.SendFollowUpError(s, i.Interaction, "通过禁言ID执行操作的功能正在重构中，请暂时使用惩罚ID。")
		return
	default:
		utils.SendFollowUpError(s, i.Interaction, "此搜索方式不支持执行操作。")
		return
	}

	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "查找惩罚记录时出错。")
		log.Printf("Error finding punishment record for action: %v", err)
		return
	}
	if record == nil {
		utils.SendFollowUpError(s, i.Interaction, "找不到相关的惩罚记录。")
		return
	}

	switch action {
	case "revoke":
		// Revoke logic needs access to config and other services,
		// which is a larger refactoring task. For now, we just delete the record.
		log.Printf("Revoking punishment ID %d (currently only deletes the record)", record.PunishmentID)
		deletePunishment(s, i, repo, record.PunishmentID)
	case "delete":
		deletePunishment(s, i, repo, record.PunishmentID)
	case "print_evidence":
		printEvidence(s, i, record)
	default:
		utils.SendFollowUpError(s, i.Interaction, "无效的操作。")
	}
}

func displayPunishmentsV2(s *discordgo.Session, i *discordgo.Interaction, searchBy, input string, page int, repo repository.PunishmentRepository) {
	var records []model.PunishmentRecord
	var err error
	var title, description string

	switch searchBy {
	case "punishment_id":
		id, convErr := strconv.ParseInt(input, 10, 64)
		if convErr != nil {
			utils.SendFollowUpError(s, i, "无效的惩罚ID。")
			return
		}
		record, getErr := repo.GetByID(id)
		if getErr != nil {
			utils.SendFollowUpError(s, i, "查找该ID的惩罚记录时出错。")
			log.Printf("Error getting punishment by ID: %v", getErr)
			return
		}
		if record != nil {
			records = append(records, *record)
		}
		title = "惩罚记录 ID: " + input
	case "punished_user_id":
		records, err = repo.GetByUserID(input)
		user, uErr := s.User(input)
		title = "用户的惩罚记录"
		if uErr == nil {
			title = fmt.Sprintf("用户 %s 的惩罚记录", user.Username)
		}
		description = fmt.Sprintf("用户: <@%s>", input)
	case "punisher_id":
		records, err = repo.GetByAdminID(input)
		user, uErr := s.User(input)
		title = "管理员执行的惩罚记录"
		if uErr == nil {
			title = fmt.Sprintf("管理员 %s 执行的惩罚记录", user.Username)
		}
		description = fmt.Sprintf("管理员: <@%s>", input)
	case "mute_db_id":
		utils.SendFollowUpError(s, i, "此搜索方式仅支持操作，不支持查询。")
		return
	}

	if err != nil {
		utils.SendFollowUpError(s, i, "检索惩罚记录失败。")
		log.Printf("Error getting punishment records: %v", err)
		return
	}

	if len(records) == 0 {
		utils.SendFollowUp(s, i, "未找到惩罚记录。")
		return
	}

	totalPages := (len(records) + recordsPerPageV2 - 1) / recordsPerPageV2
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * recordsPerPageV2
	end := start + recordsPerPageV2
	if end > len(records) {
		end = len(records)
	}
	pageRecords := records[start:end]

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       0x00ff00,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("第 %d 页，共 %d 页 (共 %d 条记录)", page, totalPages, len(records)),
		},
	}

	for _, record := range pageRecords {
		timestamp := time.Unix(record.Timestamp, 0).Format(time.RFC1123)
		value := fmt.Sprintf("用户: <@%s> (%s)\n原因: %s\n管理员: <@%s>", record.UserID, record.UserUsername, record.Reason, record.AdminID)

		member, err := s.State.Member(record.GuildID, record.UserID)
		if err != nil {
			member, err = s.GuildMember(record.GuildID, record.UserID)
		}
		if err == nil && member.CommunicationDisabledUntil != nil && member.CommunicationDisabledUntil.After(time.Now()) {
			value += fmt.Sprintf("\n**禁言至:** %s", member.CommunicationDisabledUntil.Format(time.RFC1123))
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  fmt.Sprintf("ID: %d, 时间: %s", record.PunishmentID, timestamp),
			Value: value,
		})
	}

	var components []discordgo.MessageComponent
	if totalPages > 1 {
		components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "上一页",
						Style:    discordgo.PrimaryButton,
						Disabled: page == 1,
						CustomID: fmt.Sprintf("punish_page_v2:%d:%s:%s", page-1, searchBy, input),
					},
					discordgo.Button{
						Label:    "下一页",
						Style:    discordgo.PrimaryButton,
						Disabled: page == totalPages,
						CustomID: fmt.Sprintf("punish_page_v2:%d:%s:%s", page+1, searchBy, input),
					},
				},
			},
		}
	}
	s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &components,
	})
}

func deletePunishment(s *discordgo.Session, i *discordgo.InteractionCreate, repo repository.PunishmentRepository, punishmentID int64) {
	err := repo.Delete(punishmentID)
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, fmt.Sprintf("删除惩罚记录失败: %v", err))
		return
	}
	content := fmt.Sprintf("✅ 成功删除ID为 %d 的惩罚记录。", punishmentID)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
}

func HandlePunishPaginationV2(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Failed to defer pagination interaction: %v", err)
		return
	}

	punishmentRepo := b.GetRepositoryManager().PunishmentRepository()
	if punishmentRepo == nil {
		utils.SendFollowUpError(s, i.Interaction, "数据库服务暂时不可用")
		return
	}

	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, ":")
	if len(parts) != 4 {
		log.Printf("Invalid custom ID for v2 pagination: %s", customID)
		return
	}

	page, _ := strconv.Atoi(parts[1])
	searchBy := parts[2]
	input := parts[3]

	displayPunishmentsV2(s, i.Interaction, searchBy, input, page, punishmentRepo)
}

func printEvidence(s *discordgo.Session, i *discordgo.InteractionCreate, record *model.PunishmentRecord) {
	if record.Evidence == "" {
		utils.SendFollowUpError(s, i.Interaction, "此记录没有证据。")
		return
	}

	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, []byte(record.Evidence), "", "  ")
	if err != nil {
		// Fallback to raw string if indent fails
		prettyJSON.WriteString(record.Evidence)
	}

	content := fmt.Sprintf("```json\n%s\n```", prettyJSON.String())
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
}
