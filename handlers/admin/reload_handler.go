package admin

import (
	"discord-bot/bot"
	"discord-bot/utils"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func HandleReloadConfig(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, nil, nil, b.GetConfig().DeveloperUserIDs, nil)
	if permissionLevel != utils.DeveloperPermission {
		utils.SendErrorResponse(s, i, "You do not have permission to use this command.")
		return
	}

	err := b.ReloadConfig()
	var content string
	if err != nil {
		content = fmt.Sprintf("配置重载失败: %v", err)
		utils.SendErrorResponse(s, i, content)
	} else {
		content = "✅ 配置已成功重载！"
		utils.SendSimpleResponse(s, i, content)
	}
}
