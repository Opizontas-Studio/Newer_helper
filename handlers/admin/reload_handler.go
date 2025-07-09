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
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You do not have permission to use this command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	err := b.ReloadConfig()
	var content string
	if err != nil {
		content = fmt.Sprintf("配置重载失败: %v", err)
	} else {
		content = "✅ 配置已成功重载！"
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
