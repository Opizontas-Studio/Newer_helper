package personalnav

import (
	"fmt"
	"log"
	"newer_helper/bot"
	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"
	"strings"
	"time"
	"unicode/utf8"

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

// buildSafeDescription 构建一个带长度限制的 description，确保不超过 Discord 的限制
func buildSafeDescription(prefix string, lines []string, fallback string, maxLength int) string {
	if len(lines) == 0 {
		return prefix + fallback
	}

	// 从完整内容开始，逐步减少行数直到满足长度限制
	for numLines := len(lines); numLines > 0; numLines-- {
		currentLines := lines[:numLines]
		description := prefix + strings.Join(currentLines, "\n")

		if len(description) <= maxLength {
			// 如果被截断了，添加提示
			if numLines < len(lines) {
				truncated := fmt.Sprintf("\n\n_（显示前 %d 个，共 %d 个）_", numLines, len(lines))
				if len(description)+len(truncated) <= maxLength {
					description += truncated
				}
			}
			log.Printf("personal-nav: buildSafeDescription used %d/%d lines, length=%d/%d", numLines, len(lines), len(description), maxLength)
			return description
		}
	}

	// 如果连一行都放不下，返回后备文本
	log.Printf("personal-nav: WARNING - even single line exceeds limit, using fallback")
	return prefix + fallback
}

func buildEmbeds(guildID string, channelInfos []channelChoice, latestPosts []model.Post, topPosts []model.Post) (myWorksEmbeds []*discordgo.MessageEmbed, topWorks, latest *discordgo.MessageEmbed) {
	// 按分区分组作品
	postsByPartition := groupPostsByPartition(latestPosts, channelInfos)

	// 为每个分区构建 embed（可能多个）
	for _, ci := range channelInfos {
		posts := postsByPartition[ci.TableName]

		// 构建该分区的 embed（自动分页处理所有作品）
		partitionEmbeds := buildPartitionEmbeds(ci.ChannelName, ci.ChannelID, guildID, posts, len(posts))
		myWorksEmbeds = append(myWorksEmbeds, partitionEmbeds...)
	}

	displayTop := topPosts
	if len(displayTop) > maxLatestPostsToDisplay {
		displayTop = displayTop[:maxLatestPostsToDisplay]
	}
	topLines := make([]string, 0, len(displayTop))
	for _, post := range displayTop {
		topLines = append(topLines, formatPostLineWithStats(guildID, post))
	}

	const maxEmbedDescriptionLength = 3800 // Discord 限制 4096，使用 3800 作为安全阈值
	topDescription := buildSafeDescription(
		"根据消息数量 (MessageCount) 排序。\n\n",
		topLines,
		"暂无数据。",
		maxEmbedDescriptionLength,
	)

	topWorks = &discordgo.MessageEmbed{
		Title:       "🔥 最高消息作品",
		Description: topDescription,
		Color:       embedColorHighlight,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	displayRecent := latestPosts
	if len(displayRecent) > maxLatestPostsToDisplay {
		displayRecent = displayRecent[:maxLatestPostsToDisplay]
	}
	latestLines := make([]string, 0, len(displayRecent))
	for _, post := range displayRecent {
		latestLines = append(latestLines, formatPostLineWithDate(guildID, post))
	}

	latestDescription := buildSafeDescription(
		"按时间倒序展示最新作品。\n\n",
		latestLines,
		"暂无数据。",
		maxEmbedDescriptionLength,
	)

	latest = &discordgo.MessageEmbed{
		Title:       "🆕 最新作品",
		Description: latestDescription,
		Color:       embedColorSecondary,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return myWorksEmbeds, topWorks, latest
}

func formatPostLine(guildID string, post model.Post) string {
	title := post.Title
	if strings.TrimSpace(title) == "" {
		title = "未命名作品"
	}
	return fmt.Sprintf("[%s](%s) · 💬 %d · <t:%d:R>", utils.TruncateString(title, 70), post.URL(guildID), post.MessageCount, post.Timestamp)
}

func formatPostLineWithStats(guildID string, post model.Post) string {
	title := post.Title
	if strings.TrimSpace(title) == "" {
		title = "未命名作品"
	}
	return fmt.Sprintf("[%s](%s)\n> 💬 %d · <t:%d:R>", utils.TruncateString(title, 70), post.URL(guildID), post.MessageCount, post.Timestamp)
}

func formatPostLineWithDate(guildID string, post model.Post) string {
	title := post.Title
	if strings.TrimSpace(title) == "" {
		title = "未命名作品"
	}
	return fmt.Sprintf("[%s](%s)\n> <t:%d:F>", utils.TruncateString(title, 70), post.URL(guildID), post.Timestamp)
}

// groupPostsByPartition 按分区分组作品
func groupPostsByPartition(posts []model.Post, channelInfos []channelChoice) map[string][]model.Post {
	result := make(map[string][]model.Post)

	for _, post := range posts {
		for _, ci := range channelInfos {
			if post.ChannelID == ci.ChannelID {
				result[ci.TableName] = append(result[ci.TableName], post)
				break
			}
		}
	}

	return result
}

// buildPartitionEmbeds 为单个分区构建一个或多个 embed（超过限制时拆分）
func buildPartitionEmbeds(partitionName, channelID, guildID string, posts []model.Post, totalCount int) []*discordgo.MessageEmbed {
	const maxDescriptionLength = 4000 // 优化阈值，Discord description 限制为 4096，保留96字符安全边距

	if len(posts) == 0 {
		// 没有作品，返回一个空的 embed
		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("📁 我的作品 - %s (%d个投稿)", partitionName, totalCount),
			Description: fmt.Sprintf("频道：<#%s>\n\n暂无作品记录。", channelID),
			Color:       embedColorPrimary,
			Timestamp:   time.Now().Format(time.RFC3339),
		}
		return []*discordgo.MessageEmbed{embed}
	}

	// 构建作品行
	lines := make([]string, 0, len(posts))
	for _, post := range posts {
		lines = append(lines, formatPostLine(guildID, post))
	}

	// 计算是否需要拆分
	var embeds []*discordgo.MessageEmbed
	var currentLines []string
	channelPrefix := fmt.Sprintf("频道：<#%s>\n\n", channelID)

	for _, line := range lines {
		// 模拟拼接后的内容来检查长度
		var testValue string
		if len(currentLines) == 0 {
			testValue = channelPrefix + line
		} else {
			testValue = channelPrefix + strings.Join(currentLines, "\n") + "\n" + line
		}

		// 如果加入这一行会超过限制，先保存当前的 embed
		if len(testValue) > maxDescriptionLength && len(currentLines) > 0 {
			embeds = append(embeds, createPartitionEmbed(partitionName, channelID, totalCount, currentLines, len(embeds)+1, 0))
			currentLines = []string{line}
		} else {
			currentLines = append(currentLines, line)
		}
	}

	// 添加最后一个 embed
	if len(currentLines) > 0 {
		totalPages := len(embeds) + 1
		embeds = append(embeds, createPartitionEmbed(partitionName, channelID, totalCount, currentLines, len(embeds)+1, totalPages))
	}

	// 如果有多页，需要更新之前的 embed 标题以显示页码
	if len(embeds) > 1 {
		for i := 0; i < len(embeds)-1; i++ {
			embeds[i].Title = fmt.Sprintf("📁 我的作品 - %s (第%d/%d页)", partitionName, i+1, len(embeds))
		}
		embeds[len(embeds)-1].Title = fmt.Sprintf("📁 我的作品 - %s (第%d/%d页)", partitionName, len(embeds), len(embeds))
	}

	return embeds
}

// createPartitionEmbed 创建一个分区 embed
func createPartitionEmbed(partitionName, channelID string, totalCount int, lines []string, pageNum, totalPages int) *discordgo.MessageEmbed {
	title := fmt.Sprintf("📁 我的作品 - %s (%d个投稿)", partitionName, totalCount)
	if totalPages > 1 {
		title = fmt.Sprintf("📁 我的作品 - %s (第%d/%d页)", partitionName, pageNum, totalPages)
	}

	// 构建 description：频道信息 + 作品列表
	description := fmt.Sprintf("频道：<#%s>\n\n%s", channelID, strings.Join(lines, "\n"))

	// 安全检查：确保不超过 Discord 限制（description 最大 4096 字符）
	if len(description) > 4096 {
		log.Printf("personal-nav: WARNING - description exceeds 4096 chars (%d), truncating", len(description))
		// 截断到 4090 字节（留出省略号的空间），并确保不破坏 UTF-8 字符
		maxLen := 4090
		for maxLen > 0 && maxLen < len(description) {
			// 检查是否在 UTF-8 字符边界上
			if utf8.ValidString(description[:maxLen]) {
				break
			}
			maxLen--
		}
		description = description[:maxLen] + "\n..."
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       embedColorPrimary,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return embed
}
