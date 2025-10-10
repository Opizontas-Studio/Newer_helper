package punish_admin

import (
	"encoding/json"
	"fmt"
	"log"
	"newer_helper/model"
	"newer_helper/utils"
	punishments_db "newer_helper/utils/database/punishments"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

// HandlePunishRevokeCommand 处理 /punish_revoke 命令
func HandlePunishRevokeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	revokePunishment(s, i, punishDB, record)
}

func revokePunishment(s *discordgo.Session, i *discordgo.InteractionCreate, db *sqlx.DB, record *model.PunishmentRecord) {
	punishConfig, err := utils.LoadPunishConfig("config/config_file/punish_config.json")
	if err != nil {
		utils.SendFollowUpError(s, i.Interaction, "加载处罚配置失败。")
		return
	}
	// 检查用户是否被封禁，如果是则解封
	_, banErr := s.GuildBan(record.GuildID, record.UserID)
	if banErr == nil { // 如果 err 为 nil，则用户被封禁
		unbanErr := s.GuildBanDelete(record.GuildID, record.UserID)
		if unbanErr != nil {
			log.Printf("无法从服务器 %s 解封用户 %s: %v", record.GuildID, record.UserID, unbanErr)
			// 可选：通知管理员解封失败，但流程将继续
		}
	}

	// 获取特定于服务器的操作配置
	guildActions, ok := punishConfig.PunishConfig[record.GuildID]
	if !ok {
		utils.SendFollowUpError(s, i.Interaction, "找不到此服务器的处罚配置。")
		return
	}

	// 获取具体的操作配置
	actionConfig, ok := guildActions[record.ActionType]
	if !ok {
		utils.SendFollowUpError(s, i.Interaction, "找不到此处罚类型的配置。")
		return
	}

	// 获取用户相同类型的所有处罚，以确定处罚级别
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

	// 从撤销配置中恢复角色
	if revConfig, ok := punishConfig.Revocation[record.GuildID]; ok {
		if revConfig.RecoverRoleID != "" && revConfig.RecoverRoleID != "0" {
			// 检查原始惩罚是否实际移除了任何角色
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
					log.Printf("无法为用户 %s 恢复角色 %s: %v", record.UserID, revConfig.RecoverRoleID, err)
				}
			}
		}
	}

	// 移除此惩罚添加的临时角色
	if record.TempRolesJSON != "" && record.TempRolesJSON != "[]" {
		var tempRoles []string
		err := json.Unmarshal([]byte(record.TempRolesJSON), &tempRoles)
		if err == nil {
			for _, roleID := range tempRoles {
				s.GuildMemberRoleRemove(record.GuildID, record.UserID, roleID)
			}
		}
	}

	// 移除禁言
	s.GuildMemberTimeout(record.GuildID, record.UserID, nil)

	// 删除惩罚记录
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
