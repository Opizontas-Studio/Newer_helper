package punish_admin

import (
	"bytes"
	"discord-bot/model"
	"discord-bot/utils"
	punishments_db "discord-bot/utils/database/punishments"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// HandlePunishPrintEvidenceCommand 处理 /punish_print_evidence 命令
func HandlePunishPrintEvidenceCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	record, err := punishments_db.GetPunishmentRecordByID(punishDB, punishmentID)
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "找不到相关的惩罚记录。")
		log.Printf("查找要执行操作的惩罚记录时出错: %v", err)
		return
	}

	printEvidence(s, i, record)
}

func printEvidence(s *discordgo.Session, i *discordgo.InteractionCreate, record *model.PunishmentRecord) {
	if record.Evidence == "" {
		utils.SendEphemeralResponse(s, i, "此记录没有证据。")
		return
	}

	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, []byte(record.Evidence), "", "  ")
	if err != nil {
		// 如果缩进失败，则回退到原始字符串
		prettyJSON.WriteString(record.Evidence)
	}

	content := fmt.Sprintf("```json\n%s\n```", prettyJSON.String())
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
}
