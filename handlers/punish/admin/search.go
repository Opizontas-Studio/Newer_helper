package punish_admin

import (
	"fmt"
	"log"
	"newer_helper/model"
	"newer_helper/utils"
	punishments_db "newer_helper/utils/database/punishments"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const recordsPerPageV2 = 5

// HandlePunishSearchCommand 处理 /punish_search 命令
func HandlePunishSearchCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("无法延迟交互: %v", err)
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	searchBy := optionMap["search_by"].StringValue()
	input := optionMap["input"].StringValue()

	displayPunishmentsV2(s, i.Interaction, searchBy, input, 1)
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
	}

	if err != nil {
		utils.SendFollowUpError(s, i, "检索惩罚记录失败。")
		log.Printf("获取惩罚记录时出错: %v", err)
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

func HandlePunishPaginationV2(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
