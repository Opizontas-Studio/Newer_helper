package punish

import (
	"bytes"
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	punishments_db "discord-bot/utils/database/punishments"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
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

	if action != "" {
		handleActionV2(s, i, searchBy, input, action)
	} else {
		displayPunishmentsV2(s, i.Interaction, searchBy, input, 1)
	}
}

func handleActionV2(s *discordgo.Session, i *discordgo.InteractionCreate, searchBy, input, action string) {
	var record *model.PunishmentRecord
	var err error

	punishConfig, err := utils.LoadPunishConfig("config/config_file/punish_config.json")
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "加载处罚配置失败。")
		return
	}
	punishDB, err := punishments_db.Init(punishConfig.DatabasePath)
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "连接惩罚数据库失败。")
		return
	}
	defer punishDB.Close()

	switch searchBy {
	case "punishment_id":
		id, convErr := strconv.ParseInt(input, 10, 64)
		if convErr != nil {
			utils.SendFollowUpError(s, i.Interaction, "无效的惩罚ID。")
			return
		}
		record, err = punishments_db.GetPunishmentRecordByID(punishDB, id)
	case "mute_db_id":
		// This search type is deprecated with the new punishment timer system
		utils.SendFollowUpError(s, i.Interaction, "mute_db_id 搜索类型已不再支持，请使用 punishment_id 搜索处罚记录。")
		return

	default:
		utils.SendFollowUpError(s, i.Interaction, "此搜索方式不支持执行操作。")
		return
	}

	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "找不到相关的惩罚记录。")
		log.Printf("Error finding punishment record for action: %v", err)
		return
	}

	switch action {
	case "revoke":
		revokePunishment(s, i, punishDB, record)
	case "delete":
		deletePunishment(s, i, punishDB, record.PunishmentID)
	case "print_evidence":
		printEvidence(s, i, record)
	default:
		utils.SendFollowUpError(s, i.Interaction, "无效的操作。")
	}
}

func displayPunishmentsV2(s *discordgo.Session, i *discordgo.Interaction, searchBy, input string, page int) {
	punishConfig, err := utils.LoadPunishConfig("config/config_file/punish_config.json")
	if err != nil {
		utils.SendFollowUpError(s, i, "加载处罚配置失败。")
		return
	}
	db, err := punishments_db.Init(punishConfig.DatabasePath)
	if err != nil {
		utils.SendFollowUpError(s, i, "连接惩罚数据库失败。")
		return
	}
	defer db.Close()

	var records []model.PunishmentRecord
	var title, description string

	switch searchBy {
	case "punishment_id":
		id, convErr := strconv.ParseInt(input, 10, 64)
		if convErr != nil {
			utils.SendFollowUpError(s, i, "无效的惩罚ID。")
			return
		}
		record, getErr := punishments_db.GetPunishmentRecordByID(db, id)
		if getErr != nil {
			utils.SendFollowUpError(s, i, "找不到该ID的惩罚记录。")
			return
		}
		records = append(records, *record)
		title = "惩罚记录 ID: " + input
	case "punished_user_id":
		records, err = punishments_db.GetPunishmentRecordsByUserID(db, input, nil)
		user, uErr := s.User(input)
		title = "用户的惩罚记录"
		if uErr == nil {
			title = fmt.Sprintf("用户 %s 的惩罚记录", user.Username)
		}
		description = fmt.Sprintf("用户: <@%s>", input)
	case "punisher_id":
		records, err = punishments_db.GetPunishmentRecordsByAdminID(db, input)
		user, uErr := s.User(input)
		title = "管理员执行的惩罚记录"
		if uErr == nil {
			title = fmt.Sprintf("管理员 %s 执行的惩罚记录", user.Username)
		}
		description = fmt.Sprintf("管理员: <@%s>", input)
	case "mute_db_id":
		// This case is for query only, action is handled in handleActionV2
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
			Text: fmt.Sprintf("第 %d 页，共 %d 页", page, totalPages),
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
		components = utils.CreatePaginationComponents(page, totalPages, "punish_page_v2", searchBy, input)
	}

	s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &components,
	})
}

func revokePunishment(s *discordgo.Session, i *discordgo.InteractionCreate, db *sqlx.DB, record *model.PunishmentRecord) {
	punishConfig, err := utils.LoadPunishConfig("config/config_file/punish_config.json")
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "加载处罚配置失败。")
		return
	}
	// Check if the user is banned and unban them if so
	_, banErr := s.GuildBan(record.GuildID, record.UserID)
	if banErr == nil { // If err is nil, the user is banned
		unbanErr := s.GuildBanDelete(record.GuildID, record.UserID)
		if unbanErr != nil {
			log.Printf("Failed to unban user %s from guild %s: %v", record.UserID, record.GuildID, unbanErr)
			// Optionally, notify the admin that unbanning failed but the process will continue
		}
	}

	// Get guild-specific action configurations
	guildActions, ok := punishConfig.PunishConfig[record.GuildID]
	if !ok {
		utils.SendFollowUpError(s, i.Interaction, "找不到此服务器的处罚配置。")
		return
	}

	// Get specific action configuration
	actionConfig, ok := guildActions[record.ActionType]
	if !ok {
		utils.SendFollowUpError(s, i.Interaction, "找不到此处罚类型的配置。")
		return
	}

	// Get all punishments of the same type for the user to determine the punishment level
	userPunishments, err := punishments_db.GetPunishmentRecordsByUserIDAndActionType(db, record.UserID, record.ActionType)
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "获取用户处罚历史失败。")
		return
	}

	punishmentIndex := -1
	for i, p := range userPunishments {
		if p.PunishmentID == record.PunishmentID {
			punishmentIndex = i
			break
		}
	}

	if punishmentIndex != -1 {
		punishmentLevel := strconv.Itoa(punishmentIndex)
		if levelData, ok := actionConfig.Data[punishmentLevel]; ok {
			for _, roleID := range levelData.AddRole {
				if roleID != "0" {
					s.GuildMemberRoleRemove(record.GuildID, record.UserID, roleID)
				}
			}
		}
	}

	// Restore role from revocation config
	if revConfig, ok := punishConfig.Revocation[record.GuildID]; ok {
		if revConfig.RecoverRoleID != "" && revConfig.RecoverRoleID != "0" {
			// Check if any roles were actually removed by the original punishment
			rolesWereRemoved := false
			if punishmentIndex != -1 {
				punishmentLevel := strconv.Itoa(punishmentIndex)
				if levelData, ok := actionConfig.Data[punishmentLevel]; ok {
					for _, roleID := range levelData.RemoveRoleID {
						if roleID != "0" {
							rolesWereRemoved = true
							break
						}
					}
				}
			}

			if rolesWereRemoved {
				err := s.GuildMemberRoleAdd(record.GuildID, record.UserID, revConfig.RecoverRoleID)
				if err != nil {
					log.Printf("Failed to restore role %s for user %s: %v", revConfig.RecoverRoleID, record.UserID, err)
				}
			}
		}
	}

	// Remove temporary roles that were added by this punishment
	if record.TempRolesJSON != "" && record.TempRolesJSON != "[]" {
		var tempRoles []string
		err := json.Unmarshal([]byte(record.TempRolesJSON), &tempRoles)
		if err == nil {
			for _, roleID := range tempRoles {
				s.GuildMemberRoleRemove(record.GuildID, record.UserID, roleID)
			}
		}
	}

	// Remove timeout
	s.GuildMemberTimeout(record.GuildID, record.UserID, nil)

	// Delete the punishment record
	err = punishments_db.DeletePunishmentRecordByID(db, record.PunishmentID)
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, fmt.Sprintf("撤销后删除惩罚记录失败: %v", err))
		return
	}

	content := fmt.Sprintf("✅ 成功撤销并删除ID为 %d 的惩罚记录。", record.PunishmentID)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
}

func deletePunishment(s *discordgo.Session, i *discordgo.InteractionCreate, db *sqlx.DB, punishmentID int64) {
	err := punishments_db.DeletePunishmentRecordByID(db, punishmentID)
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
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		log.Printf("Failed to defer pagination interaction: %v", err)
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

	displayPunishmentsV2(s, i.Interaction, searchBy, input, page)
}

func printEvidence(s *discordgo.Session, i *discordgo.InteractionCreate, record *model.PunishmentRecord) {
	if record.Evidence == "" {
		utils.SendEphemeralResponse(s, i, "此记录没有证据。")
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
