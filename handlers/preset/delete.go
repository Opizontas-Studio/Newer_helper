package preset

import (
	"database/sql"
	"discord-bot/model"
	"discord-bot/utils"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func HandlePresetDeleteInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b interface{}) {
	type bot interface {
		GetConfig() *model.Config
		RefreshCommands(guildID string)
		GetDB() *sql.DB
	}
	appBot := b.(bot)

	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, "_")
	action := parts[0] + "_" + parts[1]
	id := parts[2]

	var responseContent string

	if action == "cancel_delete" {
		responseContent = "删除操作已取消。"
	} else if action == "confirm_delete" {
		serverConfig, ok := appBot.GetConfig().ServerConfigs[i.GuildID]
		if !ok {
			responseContent = "找不到服务器配置。"
		} else {
			found := false
			for _, p := range serverConfig.PresetMessages {
				if p.ID == id {
					found = true
					break
				}
			}

			if found {
				db := appBot.GetDB()
				if err := utils.DeletePreset(db, i.GuildID, id); err != nil {
					responseContent = "无法删除预设。"
					utils.LogError(s, appBot.GetConfig().LogChannelID, "预设管理", "删除预设失败", err.Error())
				} else {
					responseContent = "预设已被删除。"
					logMessage := fmt.Sprintf("ID: `%s`\n操作者: `%s`", id, i.Member.User.Username)
					utils.LogInfo(s, appBot.GetConfig().LogChannelID, "预设管理", "删除预设", logMessage)
					go appBot.RefreshCommands(i.GuildID)
				}
			} else {
				responseContent = "找不到要删除的预设。"
			}
		}
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    responseContent,
			Components: []discordgo.MessageComponent{}, // 移除按钮
		},
	})
	if err != nil {
		utils.LogError(s, appBot.GetConfig().LogChannelID, "预设管理", "响应交互失败", err.Error())
	}
}
