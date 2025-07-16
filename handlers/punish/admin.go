package punish

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

const recordsPerPage = 5

func HandlePunishAdminCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
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

	db, err := database.InitPunishmentDB("data/kick_user.db")
	if err != nil {
		utils.SendErrorResponse(s, i, "连接惩罚数据库失败。")
		log.Printf("Error connecting to punishment DB: %v", err)
		return
	}
	defer db.Close()

	if action != "" {
		if searchBy != "punishment_id" {
			utils.SendErrorResponse(s, i, "只有在按惩罚ID搜索时才能执行操作。")
			return
		}
		kickConfig, err := utils.LoadKickConfig("data/kick_config.json")
		if err != nil {
			utils.SendErrorResponse(s, i, "加载踢人配置失败。")
			return
		}
		handleAction(s, i, db, input, action, kickConfig)
	} else {
		displayPunishmentPage(s, i.Interaction, db, searchBy, input, 1)
	}
}

func HandlePunishPagination(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
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

	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, ":")
	if len(parts) != 4 {
		log.Printf("Invalid custom ID for pagination: %s", customID)
		return
	}

	page, _ := strconv.Atoi(parts[1])
	searchBy := parts[2]
	input := parts[3]

	db, err := database.InitPunishmentDB("data/kick_user.db")
	if err != nil {
		utils.SendErrorResponse(s, i, "连接惩罚数据库失败。")
		log.Printf("Error connecting to punishment DB: %v", err)
		return
	}
	defer db.Close()

	displayPunishmentPage(s, i.Interaction, db, searchBy, input, page)
}

func displayPunishmentPage(s *discordgo.Session, i *discordgo.Interaction, db *sqlx.DB, searchBy, input string, page int) {
	var records []model.PunishmentRecord
	var err error
	var title string
	var description string

	switch searchBy {
	case "punishment_id":
		id, convErr := strconv.ParseInt(input, 10, 64)
		if convErr != nil {
			s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
				Content: &(&struct{ string }{"无效的惩罚ID格式。"}).string,
			})
			return
		}
		record, getErr := database.GetPunishmentRecordByID(db, id)
		if getErr != nil {
			s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
				Content: &(&struct{ string }{"找不到该ID的惩罚记录。"}).string,
			})
			log.Printf("Error getting punishment record by ID: %v", getErr)
			return
		}
		records = append(records, *record)
		title = "惩罚记录 ID: " + input
	case "punished_user_id":
		records, err = database.GetPunishmentRecordsByUserID(db, input, nil)
		user, uErr := s.User(input)
		if uErr != nil {
			log.Printf("Failed to get user %s: %v", input, uErr)
			title = "用户的惩罚记录"
		} else {
			title = fmt.Sprintf("用户 %s 的惩罚记录", user.Username)
		}
		description = fmt.Sprintf("用户: <@%s>", input)
	case "punisher_id":
		records, err = database.GetPunishmentRecordsByAdminID(db, input)
		user, uErr := s.User(input)
		if uErr != nil {
			log.Printf("Failed to get user %s: %v", input, uErr)
			title = "管理员执行的惩罚记录"
		} else {
			title = fmt.Sprintf("管理员 %s 执行的惩罚记录", user.Username)
		}
		description = fmt.Sprintf("管理员: <@%s>", input)
	}

	if err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
			Content: &(&struct{ string }{"检索惩罚记录失败。"}).string,
		})
		log.Printf("Error getting punishment records: %v", err)
		return
	}

	if len(records) == 0 {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
			Content: &(&struct{ string }{"未找到惩罚记录。"}).string,
		})
		return
	}

	totalPages := (len(records) + recordsPerPage - 1) / recordsPerPage
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * recordsPerPage
	end := start + recordsPerPage
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

		field := &discordgo.MessageEmbedField{
			Name:  fmt.Sprintf("ID: %d, 时间: %s", record.PunishmentID, timestamp),
			Value: value,
		}
		embed.Fields = append(embed.Fields, field)
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
						CustomID: fmt.Sprintf("punish_page:%d:%s:%s", page-1, searchBy, input),
					},
					discordgo.Button{
						Label:    "下一页",
						Style:    discordgo.PrimaryButton,
						Disabled: page == totalPages,
						CustomID: fmt.Sprintf("punish_page:%d:%s:%s", page+1, searchBy, input),
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

func handleAction(s *discordgo.Session, i *discordgo.InteractionCreate, db *sqlx.DB, inputID, action string, config *model.KickConfig) {
	punishmentID, err := strconv.ParseInt(inputID, 10, 64)
	if err != nil {
		utils.SendErrorResponse(s, i, "无效的惩罚ID格式。")
		return
	}

	switch action {
	case "delete":
		err := database.DeletePunishmentRecordByID(db, punishmentID)
		if err != nil {
			utils.SendErrorResponse(s, i, fmt.Sprintf("删除惩罚记录失败: %v", err))
			log.Printf("Error deleting punishment record: %v", err)
			return
		}
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &(&struct{ string }{fmt.Sprintf("成功删除ID为 %d 的惩罚记录", punishmentID)}).string,
		})
	case "revoke":
		record, err := database.GetPunishmentRecordByID(db, punishmentID)
		if err != nil {
			utils.SendErrorResponse(s, i, fmt.Sprintf("找不到惩罚记录: %v", err))
			return
		}

		guildConfig, ok := config.InitConfig.Data[record.GuildID]
		if !ok {
			utils.SendErrorResponse(s, i, "找不到此惩罚的服务器配置。")
			return
		}

		// Restore roles
		for _, roleID := range guildConfig.RemoveRoleID {
			err := s.GuildMemberRoleAdd(record.GuildID, record.UserID, roleID)
			if err != nil {
				log.Printf("Failed to restore role %s for user %s: %v", roleID, record.UserID, err)
			}
		}

		// Remove timeout
		err = s.GuildMemberTimeout(record.GuildID, record.UserID, nil)
		if err != nil {
			log.Printf("Failed to remove timeout for user %s: %v", record.UserID, err)
		}

		// Delete the punishment record
		err = database.DeletePunishmentRecordByID(db, punishmentID)
		if err != nil {
			utils.SendErrorResponse(s, i, fmt.Sprintf("撤销后删除惩罚记录失败: %v", err))
			return
		}

		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &(&struct{ string }{fmt.Sprintf("成功撤销并删除ID为 %d 的惩罚记录", punishmentID)}).string,
		})
	}
}
