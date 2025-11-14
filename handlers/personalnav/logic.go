package personalnav

import (
	"errors"
	"fmt"
	"log"
	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

const (
	endMarkerContent = "--- 个人导航结束 ---"
)

var (
	// ErrStaleNavigation indicates that the navigation message is missing and the record should be deleted.
	ErrStaleNavigation = errors.New("stale navigation record due to missing message")
	// ErrArchivedThread indicates that the navigation message is in an archived thread.
	ErrArchivedThread = errors.New("navigation message is in an archived thread")
)

// fetchNavigationData 从数据库中为指定用户和分区（表）检索所有必要的帖子数据。
// 它返回用户的最新帖子、热门帖子以及所选分区的详细信息。
func fetchNavigationData(cfg *model.Config, guildID, userID string, tableNames []string) ([]model.Post, []model.Post, []ChannelChoice, error) {
	var allLatestPosts, allTopPosts []model.Post
	var channelInfos []ChannelChoice

	// 遍历用户选择的所有分区
	for _, tableName := range tableNames {
		tableName = strings.TrimSpace(tableName)
		if tableName == "" {
			continue
		}

		// 从配置中解析分区信息（频道ID、名称等）
		channelInfo, err := resolveChannelChoice(cfg, guildID, tableName)
		if err != nil {
			log.Printf("personal-nav: failed to resolve channel for table %s: %v", tableName, err)
			continue // 如果无法解析某个分区，则跳过
		}
		channelInfos = append(channelInfos, *channelInfo)

		// 初始化数据库连接
		dbPath := fmt.Sprintf("data/%s.db", guildID)
		db, err := database.InitDB(dbPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("打开数据库失败: %w", err)
		}
		defer db.Close()

		// 确保帖子表结构存在
		if err := database.EnsurePostTableSchema(db, tableName); err != nil {
			log.Printf("personal-nav: failed to ensure schema for table %s: %v", tableName, err)
			continue
		}

		// 获取最新帖子
		latestPosts, err := database.GetPostsByAuthor(db, tableName, userID, 0)
		if err != nil {
			log.Printf("personal-nav: failed to get posts for table %s: %v", tableName, err)
			continue
		}
		// 为每个帖子附加频道ID，以便后续生成跳转链接
		for idx := range latestPosts {
			latestPosts[idx].ChannelID = channelInfo.ChannelID
		}
		allLatestPosts = append(allLatestPosts, latestPosts...)

		// 获取热门帖子
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

	// 如果在所有选定分区中都找不到用户的任何作品，则返回错误
	if len(allLatestPosts) == 0 {
		return nil, nil, nil, fmt.Errorf("在所选分区中没有找到属于您的作品")
	}

	return allLatestPosts, allTopPosts, channelInfos, nil
}

// updateNavigation 是创建或刷新导航的核心逻辑。
// 它执行以下步骤：
// 1. 获取数据 (fetchNavigationData)
// 2. 获取旧的导航记录（如果存在）
// 3. 构建 Embeds (buildEmbeds)
// 4. 应用 Embeds（发送或编辑消息） (applyNavigationEmbeds)
// 5. 将新的导航状态保存到数据库
//
// 参数:
// - guildID: 导航所在的服务器ID。
// - fallbackChannelID: 如果是首次创建或找不到旧消息，导航消息将发送到此频道。
// - navID: 导航槽位ID (1, 2, or 3)。
// - tableNames: 用户选择的分区列表。
// - userID: 导航所属的用户ID。
// - updateMode: 更新模式 ("edit" 或 "delete")。
func updateNavigation(s *discordgo.Session, cfg *model.Config, guildID, fallbackChannelID string, navID int, tableNames []string, userID, updateMode string) error {
	log.Printf("personal-nav: updating navigation guild=%s user=%s nav=%d tables=%v updateMode=%s", guildID, userID, navID, tableNames, updateMode)

	// 1. 获取所有需要的数据
	allLatestPosts, allTopPosts, channelInfos, err := fetchNavigationData(cfg, guildID, userID, tableNames)
	if err != nil {
		return err
	}

	// 2. 查找现有的导航记录以获取其数据库唯一ID
	existing, err := database.GetPersonalNavigation(userID, guildID, navID)
	if err != nil {
		return fmt.Errorf("读取旧导航记录失败: %w", err)
	}

	// 获取数据库唯一ID，用于在Embed的页脚中显示，方便调试和管理
	var navigationID int64
	if existing != nil {
		navigationID = existing.ID
	}

	// 3. 构建所有导航消息的 Embeds
	myWorksEmbeds, embedTop, embedLatest := buildEmbeds(guildID, channelInfos, allLatestPosts, allTopPosts, navigationID, navID)

	// 4. 应用 Embeds（发送新消息或编辑旧消息）
	messageChannel, myWorksIDs, topWorkID, latestWorkID, err := applyNavigationEmbeds(s, fallbackChannelID, existing, myWorksEmbeds, embedTop, embedLatest, updateMode)
	if err != nil {
		if errors.Is(err, ErrStaleNavigation) {
			log.Printf("personal-nav: navigation for user %s (slot %d) is stale, deleting record.", userID, navID)
			if delErr := database.DeletePersonalNavigation(userID, guildID, navID); delErr != nil {
				log.Printf("personal-nav: failed to delete stale navigation record for user %s (slot %d): %v", userID, navID, delErr)
				return delErr // Return the deletion error
			}
			// Return nil to indicate the issue is handled and not a failure for the batch job.
			return nil
		}
		return err
	}
	if len(myWorksIDs) == 0 || topWorkID == "" || latestWorkID == "" {
		return fmt.Errorf("未能成功创建或更新所有导航消息")
	}

	// 5. 准备导航记录并保存到数据库
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
		UpdateMode:           updateMode,
	}

	if err := database.UpsertPersonalNavigation(record); err != nil {
		return fmt.Errorf("保存导航记录失败: %w", err)
	}

	log.Printf("personal-nav: navigation slot %d saved successfully with updateMode=%s", navID, updateMode)
	return nil
}

// applyNavigationEmbeds 负责将构建好的 Embeds 发送到 Discord。
// 它会根据 updateMode 和是否存在旧消息来决定是发送新消息、编辑旧消息还是先删后发。
func applyNavigationEmbeds(s *discordgo.Session, fallbackChannelID string, existing *model.PersonalNavigation, myWorksEmbeds []*discordgo.MessageEmbed, embedTop, embedLatest *discordgo.MessageEmbed, updateMode string) (messageChannelID string, myWorksIDs []string, topWorkID, latestWorkID string, err error) {
	// 确定目标频道
	targetChannelID := fallbackChannelID
	if existing != nil && existing.MessageChannelID != "" {
		targetChannelID = existing.MessageChannelID
	}
	if targetChannelID == "" && existing != nil {
		// 作为备用，尝试使用旧记录中的第一个分区频道
		channelIDs := strings.Split(existing.ChannelID, ",")
		if len(channelIDs) > 0 {
			targetChannelID = strings.TrimSpace(channelIDs[0])
		}
	}
	if targetChannelID == "" {
		return "", nil, "", "", fmt.Errorf("无法确定导航消息要发送的频道")
	}

	messageChannelID = targetChannelID

	// 在“删除更新”模式下，清理旧的结束标记
	if updateMode == "delete" && existing != nil {
		if marker := findEndMarkerMessage(s, existing.MessageChannelID, existing.MessageIDLatestWorks); marker != nil {
			// Unpin before deleting
			if err := s.ChannelMessageUnpin(marker.ChannelID, marker.ID); err != nil {
				log.Printf("personal-nav: failed to unpin old end marker %s (ignoring): %v", marker.ID, err)
			}
			if err := s.ChannelMessageDelete(marker.ChannelID, marker.ID); err != nil {
				log.Printf("personal-nav: failed to delete old end marker %s: %v", marker.ID, err)
			} else {
				log.Printf("personal-nav: deleted old end marker %s", marker.ID)
			}
		}
	}

	log.Printf("personal-nav: apply embeds targetChannel=%s fallback=%s hasExisting=%t myWorksCount=%d updateMode=%s", targetChannelID, fallbackChannelID, existing != nil, len(myWorksEmbeds), updateMode)

	// sendOrEdit 是一个辅助函数，用于处理单个消息的发送或编辑
	sendOrEdit := func(existingMessageID string, embed *discordgo.MessageEmbed) (string, error) {
		// 记录 embed 大小信息用于调试
		descLen := 0
		if embed.Description != "" {
			descLen = len(embed.Description)
		}
		fieldsCount := len(embed.Fields)
		log.Printf("personal-nav: sendOrEdit embed title=%q descLen=%d fieldsCount=%d updateMode=%s", embed.Title, descLen, fieldsCount, updateMode)

		// 如果是“删除更新”模式，并且存在旧消息，则先删除旧消息
		if updateMode == "delete" && existing != nil && existingMessageID != "" {
			log.Printf("personal-nav: delete mode - deleting existing message %s", existingMessageID)
			err := s.ChannelMessageDelete(targetChannelID, existingMessageID)
			if err != nil {
				// 如果消息已不存在 (404)，则忽略错误继续执行
				if restErr, ok := err.(*discordgo.RESTError); ok && restErr.Response != nil && restErr.Response.StatusCode == 404 {
					log.Printf("personal-nav: existing message %s already deleted", existingMessageID)
				} else {
					log.Printf("personal-nav: failed to delete message %s: %v", existingMessageID, err)
				}
			} else {
				log.Printf("personal-nav: deleted existing message %s", existingMessageID)
			}
			// 清空旧消息ID，强制后续逻辑发送新消息
			existingMessageID = ""
		}

		// 如果是“编辑”模式，并且存在旧消息，则尝试编辑
		if updateMode == "edit" && existing != nil && existingMessageID != "" {
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
			// 如果编辑失败是因为消息不存在 (404)
			if restErr, ok := err.(*discordgo.RESTError); ok && restErr.Response != nil && restErr.Response.StatusCode == 404 {
				log.Printf("personal-nav: existing message %s missing, flagging for deletion.", existingMessageID)
				if logErr := utils.LogStaleMessage(existing.GuildID, existing.UserID, targetChannelID, existingMessageID, "personal-nav"); logErr != nil {
					log.Printf("personal-nav: failed to log stale message: %v", logErr)
				}
				return "", ErrStaleNavigation
			}
			// 对于其他编辑错误
			return "", fmt.Errorf("编辑导航消息失败: %w", err)
		}

		// 发送新消息
		msg, err := s.ChannelMessageSendComplex(targetChannelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{embed},
		})
		if err != nil {
			// 如果在目标频道发送失败，并且提供了备用频道，则尝试在备用频道发送
			if targetChannelID != fallbackChannelID && fallbackChannelID != "" {
				log.Printf("personal-nav: send to targetChannel %s failed, attempting fallback to %s", targetChannelID, fallbackChannelID)
				msg, err = s.ChannelMessageSendComplex(fallbackChannelID, &discordgo.MessageSend{
					Embeds: []*discordgo.MessageEmbed{embed},
				})
				if err == nil {
					targetChannelID = fallbackChannelID // 更新目标频道为备用频道
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

	// 解析旧的“我的作品”消息ID列表（逗号分隔）
	var existingMyWorksIDs []string
	if existing != nil && existing.MessageIDMyWorks != "" {
		existingMyWorksIDs = strings.Split(existing.MessageIDMyWorks, ",")
	}

	// 循环处理“我的作品” embeds，发送或编辑消息
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

	// 如果新的“我的作品”消息数量少于旧的，则删除多余的旧消息
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

	// 处理“最高热度作品”
	var existingTopID string
	if existing != nil {
		existingTopID = existing.MessageIDTopWorks
	}
	topWorkID, err = sendOrEdit(existingTopID, embedTop)
	if err != nil {
		return "", nil, "", "", err
	}

	// 处理“最新作品”
	var existingLatestID string
	if existing != nil {
		existingLatestID = existing.MessageIDLatestWorks
	}
	latestWorkID, err = sendOrEdit(existingLatestID, embedLatest)
	if err != nil {
		return "", nil, "", "", err
	}

	// 只有在首次创建或“删除更新”模式下才发送新的结束标记
	if existing == nil || updateMode == "delete" {
		if msg, err := s.ChannelMessageSend(messageChannelID, endMarkerContent); err != nil {
			log.Printf("personal-nav: failed to send end marker: %v", err)
			// 不将此视为致命错误
		} else {
			log.Printf("personal-nav: sent end marker %s in channel %s", msg.ID, messageChannelID)
			if err := s.ChannelMessagePin(messageChannelID, msg.ID); err != nil {
				log.Printf("personal-nav: failed to pin end marker message %s: %v", msg.ID, err)
			} else {
				log.Printf("personal-nav: pinned end marker message %s", msg.ID)
			}
		}
	}

	return messageChannelID, myWorksIDs, topWorkID, latestWorkID, nil
}

// resolveChannelChoice 根据分区表名（如 "作品_1234"）从服务器配置中解析出对应的频道信息。
func resolveChannelChoice(cfg *model.Config, guildID, tableName string) (*ChannelChoice, error) {
	guildTask, ok := cfg.TaskConfig[guildID]
	if !ok {
		return nil, fmt.Errorf("未配置该服务器的分区信息。")
	}
	for name, channelTask := range guildTask.Data {
		if len(channelTask.ChannelID) < 4 {
			continue
		}
		// 表名是根据分区名和频道ID后四位生成的
		expected := fmt.Sprintf("%s_%s", name, channelTask.ChannelID[len(channelTask.ChannelID)-4:])
		if expected == tableName {
			return &ChannelChoice{
				TableName:   tableName,
				ChannelID:   channelTask.ChannelID,
				ChannelName: name,
			}, nil
		}
	}
	return nil, fmt.Errorf("无法解析导航所属的分区。")
}

// UpdateNavigationScheduled 在计划任务上下文中更新单个导航（没有用户交互）。
// 这是由 bot 包中的定时器调用的公共函数。
func UpdateNavigationScheduled(s *discordgo.Session, cfg *model.Config, nav model.PersonalNavigation) error {
	tableNames := strings.Split(nav.TableName, ",")

	// 使用存储的更新模式，如果未设置则默认为 "delete"
	updateMode := nav.UpdateMode
	if updateMode == "" {
		updateMode = updateModeDelete
		log.Printf("personal-nav: nav %d has no UpdateMode, defaulting to delete", nav.NavID)
	}

	// 使用存储的消息频道ID作为目标频道。
	// 在计划更新中，我们没有交互上下文，因此完全依赖于数据库中存储的频道ID。
	fallbackChannelID := nav.MessageChannelID
	if fallbackChannelID == "" {
		// 如果消息频道ID因故丢失，则尝试使用分区频道列表中的第一个作为备用。
		channelIDs := strings.Split(nav.ChannelID, ",")
		if len(channelIDs) > 0 {
			fallbackChannelID = strings.TrimSpace(channelIDs[0])
		}
	}

	if fallbackChannelID == "" {
		return fmt.Errorf("no valid channel ID found for nav %d (guild=%s user=%s)",
			nav.NavID, nav.GuildID, nav.UserID)
	}

	// 检查消息所在的频道/帖子状态
	channel, err := s.Channel(fallbackChannelID)
	if err != nil {
		log.Printf("personal-nav: failed to get channel %s for nav %d: %v", fallbackChannelID, nav.NavID, err)
		if restErr, ok := err.(*discordgo.RESTError); ok && restErr.Response != nil && restErr.Response.StatusCode == 404 {
			// 频道不存在，标记为过时记录
			return ErrStaleNavigation
		}
		return err // 其他错误，可能稍后重试
	}

	// 检查是否是子区
	isThread := channel.Type == discordgo.ChannelTypeGuildPublicThread ||
		channel.Type == discordgo.ChannelTypeGuildPrivateThread ||
		channel.Type == discordgo.ChannelTypeGuildNewsThread

	if isThread {
		// 检查子区是否已归档
		if channel.ThreadMetadata != nil && channel.ThreadMetadata.Archived {
			log.Printf("personal-nav: skipping nav %d (guild=%s user=%s) - message channel %s is archived",
				nav.NavID, nav.GuildID, nav.UserID, fallbackChannelID)
			return ErrArchivedThread
		}

		// 检查子区近期活动
		messages, err := s.ChannelMessages(fallbackChannelID, 5, "", "", "")
		if err != nil {
			// 如果获取消息失败，为保险起见，继续更新
			log.Printf("personal-nav: failed to fetch messages for thread %s, proceeding with update anyway: %v", fallbackChannelID, err)
		} else {
			hasUserActivity := false
			for _, msg := range messages {
				if msg.Author != nil && !msg.Author.Bot {
					hasUserActivity = true
					break
				}
			}
			if !hasUserActivity {
				log.Printf("personal-nav: skipping nav %d in thread %s due to no recent user activity", nav.NavID, fallbackChannelID)
				return nil // 跳过更新，这不是一个错误
			}
		}
	}

	// 调用核心更新逻辑
	return updateNavigation(s, cfg, nav.GuildID, fallbackChannelID, nav.NavID, tableNames, nav.UserID, updateMode)
}

// deleteNavigation handles the deletion of a personal navigation.
// It removes all associated Discord messages and the database record.
func deleteNavigation(s *discordgo.Session, nav model.PersonalNavigation) error {
	log.Printf("personal-nav: delete navigation start guild=%s user=%s nav=%d", nav.GuildID, nav.UserID, nav.NavID)

	// Determine the channel where messages are located.
	channelID := nav.MessageChannelID
	if channelID == "" {
		// Fallback to the first channel in the list if MessageChannelID is missing.
		channelIDs := strings.Split(nav.ChannelID, ",")
		if len(channelIDs) > 0 && strings.TrimSpace(channelIDs[0]) != "" {
			channelID = strings.TrimSpace(channelIDs[0])
		}
	}

	if channelID == "" {
		log.Printf("personal-nav: cannot determine channel for nav %d, proceeding to delete DB record", nav.ID)
	} else {
		// Collect all message IDs to be deleted.
		var allIDs []string
		if nav.MessageIDMyWorks != "" {
			for _, id := range strings.Split(nav.MessageIDMyWorks, ",") {
				if trimmedID := strings.TrimSpace(id); trimmedID != "" {
					allIDs = append(allIDs, trimmedID)
				}
			}
		}
		if nav.MessageIDTopWorks != "" {
			allIDs = append(allIDs, nav.MessageIDTopWorks)
		}
		if nav.MessageIDLatestWorks != "" {
			allIDs = append(allIDs, nav.MessageIDLatestWorks)
		}

		// Find and add the end marker message to the deletion list.
		if marker := findEndMarkerMessage(s, channelID, nav.MessageIDLatestWorks); marker != nil {
			if err := s.ChannelMessageUnpin(marker.ChannelID, marker.ID); err != nil {
				log.Printf("personal-nav: failed to unpin end marker %s (ignoring): %v", marker.ID, err)
			}
			allIDs = append(allIDs, marker.ID)
			log.Printf("personal-nav: queued end marker message %s for deletion", marker.ID)
		}

		// Concurrently delete all messages.
		var wg sync.WaitGroup
		for _, msgID := range allIDs {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				if err := s.ChannelMessageDelete(channelID, id); err != nil {
					// Tolerate "not found" errors as the message might have been deleted manually.
					if restErr, ok := err.(*discordgo.RESTError); ok && restErr.Response != nil && restErr.Response.StatusCode == 404 {
						log.Printf("personal-nav: message %s in channel %s already deleted.", id, channelID)
					} else {
						log.Printf("personal-nav: failed to delete message %s in channel %s: %v", id, channelID, err)
					}
				} else {
					log.Printf("personal-nav: deleted message %s in channel %s", id, channelID)
				}
			}(msgID)
		}
		wg.Wait()
	}

	// Finally, delete the navigation record from the database.
	if err := database.DeletePersonalNavigation(nav.UserID, nav.GuildID, nav.NavID); err != nil {
		return fmt.Errorf("删除数据库中的导航记录失败: %w", err)
	}

	log.Printf("personal-nav: delete navigation finished for nav=%d", nav.NavID)
	return nil
}

// findEndMarkerMessage finds the message that acts as an end marker for a navigation.
// It looks for a specific message sent by the bot right after the 'latest works' message.
func findEndMarkerMessage(s *discordgo.Session, channelID, afterMessageID string) *discordgo.Message {
	if channelID == "" || afterMessageID == "" {
		return nil
	}

	// Fetch the message that comes immediately after the latest works message.
	messages, err := s.ChannelMessages(channelID, 1, "", afterMessageID, "")
	if err != nil {
		log.Printf("personal-nav: failed to fetch messages after %s in channel %s: %v", afterMessageID, channelID, err)
		return nil
	}

	// We expect exactly one message.
	if len(messages) != 1 {
		return nil
	}

	markerMsg := messages[0]

	// Validate that the message is the one we're looking for.
	// It must be sent by the bot and have the specific marker content.
	if markerMsg.Author.ID == s.State.User.ID && markerMsg.Content == endMarkerContent {
		return markerMsg
	}

	return nil
}
