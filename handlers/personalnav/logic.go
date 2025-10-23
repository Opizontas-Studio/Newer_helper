package personalnav

import (
	"fmt"
	"log"
	"newer_helper/bot"
	"newer_helper/model"
	"newer_helper/utils/database"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// This file will contain the core business logic for creating, refreshing,
// and managing personal navigation embeds.

// fetchNavigationData retrieves all necessary post data from the database for a given user and set of tables.
func fetchNavigationData(b *bot.Bot, guildID, userID string, tableNames []string) ([]model.Post, []model.Post, []channelChoice, error) {
	var allLatestPosts, allTopPosts []model.Post
	var channelInfos []channelChoice

	for _, tableName := range tableNames {
		tableName = strings.TrimSpace(tableName)
		if tableName == "" {
			continue
		}

		channelInfo, err := resolveChannelChoice(b, guildID, tableName)
		if err != nil {
			log.Printf("personal-nav: failed to resolve channel for table %s: %v", tableName, err)
			continue // Skip if a table can't be resolved
		}
		channelInfos = append(channelInfos, channelInfo)

		dbPath := fmt.Sprintf("data/%s.db", guildID)
		db, err := database.InitDB(dbPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("打开数据库失败: %w", err)
		}
		defer db.Close()

		if err := database.EnsurePostTableSchema(db, tableName); err != nil {
			log.Printf("personal-nav: failed to ensure schema for table %s: %v", tableName, err)
			continue
		}

		// Fetch latest posts
		latestPosts, err := database.GetPostsByAuthor(db, tableName, userID, 0)
		if err != nil {
			log.Printf("personal-nav: failed to get posts for table %s: %v", tableName, err)
			continue
		}
		for idx := range latestPosts {
			latestPosts[idx].ChannelID = channelInfo.ChannelID
		}
		allLatestPosts = append(allLatestPosts, latestPosts...)

		// Fetch top posts
		topPosts, err := database.GetTopPostsByAuthor(db, tableName, userID, 0)
		if err != nil {
			log.Printf("personal-nav: failed to get top posts for table %s: %v", tableName, err)
			continue
		}
		for idx := range topPosts {
			topPosts[idx].ChannelID = channelInfo.ChannelID
		}
		allTopPosts = append(allTopPosts, topPosts...)
	}

	if len(allLatestPosts) == 0 {
		return nil, nil, nil, fmt.Errorf("在所选分区中没有找到属于您的作品")
	}

	return allLatestPosts, allTopPosts, channelInfos, nil
}

// updateNavigation is the core logic for creating or refreshing a navigation.
// It fetches data, builds embeds, and applies them.
func updateNavigation(s *discordgo.Session, b *bot.Bot, i *discordgo.InteractionCreate, navID int, tableNames []string, userID string) error {
	guildID := i.GuildID
	log.Printf("personal-nav: updating navigation guild=%s user=%s nav=%d tables=%v", guildID, userID, navID, tableNames)

	// 1. Fetch all necessary data
	allLatestPosts, allTopPosts, channelInfos, err := fetchNavigationData(b, guildID, userID, tableNames)
	if err != nil {
		return err
	}

	// 2. Build embeds
	myWorksEmbeds, embedTop, embedLatest := buildEmbeds(guildID, channelInfos, allLatestPosts, allTopPosts)

	// 3. Find existing navigation to update
	existing, err := database.GetPersonalNavigation(userID, guildID, navID)
	if err != nil {
		return fmt.Errorf("读取旧导航记录失败: %w", err)
	}

	// 4. Apply embeds (send or edit messages)
	messageChannel, myWorksIDs, topWorkID, latestWorkID, err := applyNavigationEmbeds(s, i.ChannelID, existing, myWorksEmbeds, embedTop, embedLatest)
	if err != nil {
		return err
	}
	if len(myWorksIDs) == 0 || topWorkID == "" || latestWorkID == "" {
		return fmt.Errorf("未能成功创建或更新所有导航消息")
	}

	// 5. Prepare and save the navigation record
	var tableNamesStr, channelIDsStr, channelNamesStr string
	if len(tableNames) > 0 {
		tableNamesStr = strings.Join(tableNames, ",")
	}
	if len(channelInfos) > 0 {
		var cids, cnames []string
		for _, ci := range channelInfos {
			cids = append(cids, ci.ChannelID)
			cnames = append(cnames, ci.ChannelName)
		}
		channelIDsStr = strings.Join(cids, ",")
		channelNamesStr = strings.Join(cnames, ",")
	}

	record := model.PersonalNavigation{
		UserID:               userID,
		GuildID:              guildID,
		NavID:                navID,
		ChannelID:            channelIDsStr,
		TableName:            tableNamesStr,
		ChannelName:          channelNamesStr,
		MessageChannelID:     messageChannel,
		MessageIDMyWorks:     strings.Join(myWorksIDs, ","),
		MessageIDTopWorks:    topWorkID,
		MessageIDLatestWorks: latestWorkID,
	}

	if err := database.UpsertPersonalNavigation(record); err != nil {
		return fmt.Errorf("保存导航记录失败: %w", err)
	}

	log.Printf("personal-nav: navigation slot %d saved successfully", navID)
	return nil
}

func applyNavigationEmbeds(s *discordgo.Session, fallbackChannelID string, existing *model.PersonalNavigation, myWorksEmbeds []*discordgo.MessageEmbed, embedTop, embedLatest *discordgo.MessageEmbed) (messageChannelID string, myWorksIDs []string, topWorkID, latestWorkID string, err error) {
	targetChannelID := fallbackChannelID
	if existing != nil && existing.MessageChannelID != "" {
		targetChannelID = existing.MessageChannelID
	}
	if targetChannelID == "" && existing != nil {
		targetChannelID = existing.ChannelID
	}
	if targetChannelID == "" {
		return "", nil, "", "", fmt.Errorf("无法确定导航消息要发送的频道。")
	}

	messageChannelID = targetChannelID

	log.Printf("personal-nav: apply embeds targetChannel=%s fallback=%s hasExisting=%t myWorksCount=%d", targetChannelID, fallbackChannelID, existing != nil, len(myWorksEmbeds))

	sendOrEdit := func(existingMessageID string, embed *discordgo.MessageEmbed) (string, error) {
		// 记录 embed 大小信息用于调试
		descLen := 0
		if embed.Description != "" {
			descLen = len(embed.Description)
		}
		fieldsCount := len(embed.Fields)
		log.Printf("personal-nav: sendOrEdit embed title=%q descLen=%d fieldsCount=%d", embed.Title, descLen, fieldsCount)

		if existing != nil && existingMessageID != "" {
			msg, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				Channel: targetChannelID,
				ID:      existingMessageID,
				Embeds:  &[]*discordgo.MessageEmbed{embed},
			})
			if err == nil {
				messageChannelID = msg.ChannelID
				log.Printf("personal-nav: edited existing message %s in channel %s", msg.ID, msg.ChannelID)
				return msg.ID, nil
			}
			if restErr, ok := err.(*discordgo.RESTError); !ok || restErr.Response == nil || restErr.Response.StatusCode != 404 {
				return "", fmt.Errorf("编辑导航消息失败: %w", err)
			}
			log.Printf("personal-nav: existing message %s missing, sending new one", existingMessageID)
		}

		msg, err := s.ChannelMessageSendComplex(targetChannelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{embed},
		})
		if err != nil {
			if targetChannelID != fallbackChannelID && fallbackChannelID != "" {
				log.Printf("personal-nav: send to targetChannel %s failed, attempting fallback to %s", targetChannelID, fallbackChannelID)
				msg, err = s.ChannelMessageSendComplex(fallbackChannelID, &discordgo.MessageSend{
					Embeds: []*discordgo.MessageEmbed{embed},
				})
				if err == nil {
					targetChannelID = fallbackChannelID
					messageChannelID = msg.ChannelID
					log.Printf("personal-nav: fallback send succeeded message=%s channel=%s", msg.ID, msg.ChannelID)
					return msg.ID, nil
				}
			}
			return "", fmt.Errorf("发送导航消息失败: %w", err)
		}
		messageChannelID = msg.ChannelID
		log.Printf("personal-nav: sent new message %s in channel %s", msg.ID, msg.ChannelID)
		return msg.ID, nil
	}

	// 解析旧的"我的作品"消息 ID（逗号分隔）
	var existingMyWorksIDs []string
	if existing != nil && existing.MessageIDMyWorks != "" {
		existingMyWorksIDs = strings.Split(existing.MessageIDMyWorks, ",")
	}

	// 处理"我的作品" embeds
	for i, embed := range myWorksEmbeds {
		var existingID string
		if i < len(existingMyWorksIDs) {
			existingID = strings.TrimSpace(existingMyWorksIDs[i])
		}

		newID, err := sendOrEdit(existingID, embed)
		if err != nil {
			return "", nil, "", "", err
		}
		myWorksIDs = append(myWorksIDs, newID)
	}

	// 删除多余的旧"我的作品"消息
	for i := len(myWorksEmbeds); i < len(existingMyWorksIDs); i++ {
		oldID := strings.TrimSpace(existingMyWorksIDs[i])
		if oldID != "" {
			if err := s.ChannelMessageDelete(targetChannelID, oldID); err != nil {
				log.Printf("personal-nav: failed to delete old myWorks message %s: %v", oldID, err)
			} else {
				log.Printf("personal-nav: deleted old myWorks message %s", oldID)
			}
		}
	}

	// 处理"最高消息作品"
	var existingTopID string
	if existing != nil {
		existingTopID = existing.MessageIDTopWorks
	}
	topWorkID, err = sendOrEdit(existingTopID, embedTop)
	if err != nil {
		return "", nil, "", "", err
	}

	// 处理"最新作品"
	var existingLatestID string
	if existing != nil {
		existingLatestID = existing.MessageIDLatestWorks
	}
	latestWorkID, err = sendOrEdit(existingLatestID, embedLatest)
	if err != nil {
		return "", nil, "", "", err
	}

	return messageChannelID, myWorksIDs, topWorkID, latestWorkID, nil
}

func resolveChannelChoice(b *bot.Bot, guildID, tableName string) (channelChoice, error) {
	guildTask, ok := b.GetConfig().TaskConfig[guildID]
	if !ok {
		return channelChoice{}, fmt.Errorf("未配置该服务器的分区信息。")
	}
	for name, channelTask := range guildTask.Data {
		if len(channelTask.ChannelID) < 4 {
			continue
		}
		expected := fmt.Sprintf("%s_%s", name, channelTask.ChannelID[len(channelTask.ChannelID)-4:])
		if expected == tableName {
			return channelChoice{
				TableName:   tableName,
				ChannelID:   channelTask.ChannelID,
				ChannelName: name,
			}, nil
		}
	}
	return channelChoice{}, fmt.Errorf("无法解析导航所属的分区。")
}
