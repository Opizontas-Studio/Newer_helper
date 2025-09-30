package punish_admin

import (
	"discord-bot/utils"
	punishments_db "discord-bot/utils/database/punishments"
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

// HandlePunishDeleteCommand 处理 /punish_delete 命令
func HandlePunishDeleteCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	punishmentIDStr := optionMap["punishment_id"].StringValue()
	punishmentID, convErr := strconv.ParseInt(punishmentIDStr, 10, 64)
	if convErr != nil {
		utils.SendFollowUpError(s, i.Interaction, "无效的惩罚ID。")
		return
	}

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

	deletePunishment(s, i, punishDB, punishmentID)
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
