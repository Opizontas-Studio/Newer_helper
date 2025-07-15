package punish

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func HandlePunishCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	kickConfig, err := utils.LoadKickConfig("data/kick_config.json")
	if err != nil {
		utils.SendErrorResponse(s, i, "Failed to load kick configuration.")
		log.Printf("Error loading kick config: %v", err)
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	targetUser := optionMap["user"].UserValue(s)
	reason := optionMap["reason"].StringValue()

	configEntry, ok := kickConfig.InitConfig.Data[i.GuildID]
	if !ok {
		utils.SendErrorResponse(s, i, "❓ 此服务器未找到可用配置文件")
		return
	}

	targetMember, err := s.GuildMember(i.GuildID, targetUser.ID)
	if err != nil {
		utils.SendErrorResponse(s, i, "Could not retrieve member details.")
		log.Printf("Error getting member details: %v", err)
		return
	}

	for _, whitelistRole := range configEntry.WhitelistRoleID {
		for _, userRole := range targetMember.Roles {
			if userRole == whitelistRole {
				utils.SendErrorResponse(s, i, "This user is on the whitelist and cannot be punished.")
				return
			}
		}
	}

	for _, roleID := range configEntry.RemoveRoleID {
		err := s.GuildMemberRoleRemove(i.GuildID, targetUser.ID, roleID)
		if err != nil {
			log.Printf("Failed to remove role %s from user %s: %v", roleID, targetUser.ID, err)
		}
	}

	db, err := database.InitPunishmentDB(kickConfig.InitConfig.DBPath)
	if err != nil {
		utils.SendErrorResponse(s, i, "Failed to connect to the punishment database.")
		log.Printf("Error connecting to punishment DB: %v", err)
		return
	}
	defer db.Close()

	record := model.PunishmentRecord{
		MessageID:    i.ID,
		AdminID:      i.Member.User.ID,
		UserID:       targetUser.ID,
		UserUsername: targetUser.Username,
		Reason:       reason,
	}

	if err := database.AddPunishmentRecord(db, record); err != nil {
		utils.SendErrorResponse(s, i, "Failed to save the punishment record.")
		log.Printf("Error saving punishment record: %v", err)
		return
	}

	history, err := database.GetPunishmentRecordsByUserID(db, targetUser.ID)
	if err != nil {
		log.Printf("Error fetching punishment history: %v", err)
		// Decide if you want to send a message without history or an error
	}

	embed := &discordgo.MessageEmbed{
		Title: "用户惩罚",
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: targetUser.AvatarURL(""),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "用户",
				Value: targetUser.Mention(),
			},
			{
				Name:  "原因",
				Value: reason,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("由 %s 操作", i.Member.User.Username),
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Color:     0xff0000,
	}

	if len(history) > 0 {
		var historyValue string
		for _, rec := range history {
			historyValue += fmt.Sprintf("操作人: <@%s>, 原因: %s\n", rec.AdminID, rec.Reason)
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "历史处罚记录",
			Value: historyValue,
		})
	}

	// First, send an ephemeral confirmation to the user who issued the command
	utils.SendSimpleResponse(s, i, "✅ 惩罚指令已成功执行。")

	// Then, send the detailed punishment embed as a public message to the channel
	_, err = s.ChannelMessageSendEmbed(i.ChannelID, embed)
	if err != nil {
		log.Printf("Failed to send punishment embed to channel %s: %v", i.ChannelID, err)
	}
}
