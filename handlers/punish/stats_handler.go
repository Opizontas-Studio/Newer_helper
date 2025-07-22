package punish

import (
	"database/sql"
	"discord-bot/tasks"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

type botProvider interface {
	GetDB() *sql.DB
	GetDBX() *sqlx.DB
}

func HandlePunishmentStatsCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b botProvider) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	action := optionMap["action"].StringValue()
	inputOption, ok := optionMap["input"]
	var input string
	if ok {
		input = inputOption.StringValue()
	}

	switch action {
	case "register":
		handleRegister(s, i, b, input)
	case "delete":
		handleDelete(s, i, b, input)
	case "set_server":
		handleSetServer(s, i, b, input)
	default:
		utils.SendEphemeralResponse(s, i, "未知的操作")
	}
}

func handleRegister(s *discordgo.Session, i *discordgo.InteractionCreate, b botProvider, channelID string) {
	if channelID == "" {
		utils.SendEphemeralResponse(s, i, "请输入要注册的频道ID")
		return
	}

	targetGuildID := i.GuildID

	err := database.AddPunishmentStatsChannel(b.GetDB(), i.GuildID, channelID, targetGuildID)
	if err != nil {
		log.Printf("Failed to register punishment stats channel: %v", err)
		utils.SendEphemeralResponse(s, i, "注册频道失败")
		return
	}

	utils.SendEphemeralResponse(s, i, fmt.Sprintf("频道 <#%s> 已成功注册用于显示服务器 %s 的处罚统计信息。正在生成初始排行榜...", channelID, targetGuildID))

	go func() {
		embed, err := tasks.GeneratePunishmentStatsEmbed(b.GetDBX(), targetGuildID, 24*time.Hour)
		if err != nil {
			log.Printf("Failed to generate initial punishment stats embed: %v", err)
			content := "频道注册成功，但生成初始排行榜失败。"
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &content,
			})
			return
		}

		msg, err := s.ChannelMessageSendEmbed(channelID, embed)
		if err != nil {
			log.Printf("Failed to send initial punishment stats message to channel %s: %v", channelID, err)
			content := "频道注册成功，但发送初始排行榜失败。"
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &content,
			})
			return
		}

		err = database.UpdatePunishmentStatsChannel(b.GetDB(), channelID, msg.ID)
		if err != nil {
			log.Printf("Failed to update punishment stats message ID for channel %s: %v", channelID, err)
		}

		content := fmt.Sprintf("频道 <#%s> 已成功注册，并已发送初始排行榜。", channelID)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
	}()
}

func handleDelete(s *discordgo.Session, i *discordgo.InteractionCreate, b botProvider, channelID string) {
	if channelID == "" {
		utils.SendEphemeralResponse(s, i, "请输入要删除的频道ID")
		return
	}

	err := database.DeletePunishmentStatsChannel(b.GetDB(), channelID)
	if err != nil {
		log.Printf("Failed to delete punishment stats channel: %v", err)
		utils.SendEphemeralResponse(s, i, "删除频道失败")
		return
	}

	utils.SendEphemeralResponse(s, i, fmt.Sprintf("频道 <#%s> 的处罚统计配置已成功删除。", channelID))
}

func handleSetServer(s *discordgo.Session, i *discordgo.InteractionCreate, b botProvider, targetGuildID string) {
	if targetGuildID == "" {
		utils.SendEphemeralResponse(s, i, "请输入目标服务器ID")
		return
	}

	channelID := i.ChannelID
	err := database.UpdatePunishmentStatsTargetGuild(b.GetDB(), channelID, targetGuildID)
	if err != nil {
		log.Printf("Failed to set target guild for punishment stats channel %s: %v", channelID, err)
		utils.SendEphemeralResponse(s, i, "设置目标服务器失败")
		return
	}

	utils.SendEphemeralResponse(s, i, fmt.Sprintf("频道 <#%s> 的处罚统计目标服务器已成功设置为 %s。", channelID, targetGuildID))
}
