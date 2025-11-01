package new_card_emoji

import (
	"database/sql"
	"log"
	"newer_helper/model"
	"newer_helper/utils/database"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	timersMutex sync.RWMutex
	activeTimers = make(map[string]*PostTimerInfo) // key: postID
)

// CreateTimersForNewPost 为新发布的帖子创建72h和144h计时器
func CreateTimersForNewPost(s *discordgo.Session, guildID string, post *model.Post) {
	log.Printf("[NewCardEmoji] Creating timers for new post: %s (guild: %s)", post.ID, guildID)

	// 确保全局状态已初始化
	if globalState == nil {
		log.Printf("[NewCardEmoji] Warning: globalState is nil, skipping timer creation")
		return
	}

	// 检查是否已经存在记录
	if record, exists := globalState.GetRecord(post.ID); exists {
		log.Printf("[NewCardEmoji] Post %s already has timers, skipping", post.ID)
		// 如果已发送，不再创建计时器
		if record.Sent72h && record.Sent144h {
			return
		}
	}

	// 创建新记录
	record := &SentRecord{
		PostID:    post.ID,
		GuildID:   guildID,
		ChannelID: post.ChannelID,
		CreatedAt: post.Timestamp,
		Sent72h:   false,
		Sent144h:  false,
	}
	globalState.SetRecord(post.ID, record)

	// 保存状态
	if err := SaveState(globalState); err != nil {
		log.Printf("[NewCardEmoji] Error saving state: %v", err)
	}

	// 创建计时器信息
	timerInfo := &PostTimerInfo{
		PostID:    post.ID,
		GuildID:   guildID,
		ChannelID: post.ChannelID,
		CreatedAt: time.Unix(post.Timestamp, 0),
		Sent72h:   false,
		Sent144h:  false,
	}

	// 创建72小时计时器
	timerInfo.Timer72h = time.AfterFunc(72*time.Hour, func() {
		handle72hTimer(s, post.ID, guildID, post.ChannelID)
	})

	// 创建144小时计时器
	timerInfo.Timer144h = time.AfterFunc(144*time.Hour, func() {
		handle144hTimer(s, post.ID, guildID, post.ChannelID)
	})

	// 保存到活动计时器映射
	timersMutex.Lock()
	activeTimers[post.ID] = timerInfo
	timersMutex.Unlock()

	log.Printf("[NewCardEmoji] Timers created for post %s: 72h and 144h", post.ID)
}

// handle72hTimer 处理72小时计时器触发
func handle72hTimer(s *discordgo.Session, postID, guildID, channelID string) {
	log.Printf("[NewCardEmoji] 72h timer triggered for post %s", postID)

	// 检查是否已发送
	if record, exists := globalState.GetRecord(postID); exists && record.Sent72h {
		log.Printf("[NewCardEmoji] Post %s already sent at 72h, skipping", postID)
		return
	}

	// 添加到队列
	item := &QueueItem{
		PostID:    postID,
		GuildID:   guildID,
		ChannelID: channelID,
		Trigger:   "72h",
	}
	AddToQueue(item)
}

// handle144hTimer 处理144小时计时器触发
func handle144hTimer(s *discordgo.Session, postID, guildID, channelID string) {
	log.Printf("[NewCardEmoji] 144h timer triggered for post %s", postID)

	// 检查72h是否已发送
	if record, exists := globalState.GetRecord(postID); exists {
		if record.Sent72h {
			log.Printf("[NewCardEmoji] Post %s already sent at 72h, skipping 144h", postID)
			return
		}
		if record.Sent144h {
			log.Printf("[NewCardEmoji] Post %s already sent at 144h, skipping", postID)
			return
		}
	}

	// 添加到队列
	item := &QueueItem{
		PostID:    postID,
		GuildID:   guildID,
		ChannelID: channelID,
		Trigger:   "144h",
	}
	AddToQueue(item)
}

// RebuildTimersOnStartup 在启动时从数据库重建计时器
func RebuildTimersOnStartup(s *discordgo.Session, cfg *model.Config) error {
	log.Println("[NewCardEmoji] Rebuilding timers from database...")

	// 遍历所有配置的服务器
	for guildID, threadConfig := range cfg.ThreadConfig {
		log.Printf("[NewCardEmoji] Processing guild %s (%s)", guildID, threadConfig.Name)

		// 打开数据库
		db, err := database.InitDB(threadConfig.Database)
		if err != nil {
			log.Printf("[NewCardEmoji] Error opening database for guild %s: %v", guildID, err)
			continue
		}

		// 获取所有表名
		tableNames, err := database.GetAllTableNames(db)
		if err != nil {
			log.Printf("[NewCardEmoji] Error getting table names for guild %s: %v", guildID, err)
			db.Close()
			continue
		}

		// 查询最近144小时的帖子
		posts, err := getRecentPosts(db, tableNames, 144)
		if err != nil {
			log.Printf("[NewCardEmoji] Error getting recent posts for guild %s: %v", guildID, err)
			db.Close()
			continue
		}

		db.Close()

		log.Printf("[NewCardEmoji] Found %d posts in last 144 hours for guild %s", len(posts), guildID)

		// 为每个帖子创建计时器
		for _, post := range posts {
			rebuildTimerForPost(s, guildID, &post)
		}
	}

	log.Println("[NewCardEmoji] Timer rebuild completed")
	return nil
}

// rebuildTimerForPost 为单个帖子重建计时器
func rebuildTimerForPost(s *discordgo.Session, guildID string, post *model.Post) {
	postAge := time.Since(time.Unix(post.Timestamp, 0))

	// 检查状态记录
	record, exists := globalState.GetRecord(post.ID)
	if !exists {
		// 创建新记录
		record = &SentRecord{
			PostID:    post.ID,
			GuildID:   guildID,
			ChannelID: post.ChannelID,
			CreatedAt: post.Timestamp,
			Sent72h:   false,
			Sent144h:  false,
		}
		globalState.SetRecord(post.ID, record)
	}

	// 检查是否已经都发送过了
	if record.Sent72h && record.Sent144h {
		return // 都已发送，无需创建计时器
	}

	timerInfo := &PostTimerInfo{
		PostID:    post.ID,
		GuildID:   guildID,
		ChannelID: post.ChannelID,
		CreatedAt: time.Unix(post.Timestamp, 0),
		Sent72h:   record.Sent72h,
		Sent144h:  record.Sent144h,
	}

	// 72小时计时器
	if !record.Sent72h {
		if postAge >= 72*time.Hour {
			// 已超过72小时，立即触发
			go handle72hTimer(s, post.ID, guildID, post.ChannelID)
		} else {
			// 创建剩余时间的计时器
			remaining := 72*time.Hour - postAge
			timerInfo.Timer72h = time.AfterFunc(remaining, func() {
				handle72hTimer(s, post.ID, guildID, post.ChannelID)
			})
			log.Printf("[NewCardEmoji] Created 72h timer for post %s, remaining: %v", post.ID, remaining)
		}
	}

	// 144小时计时器
	if !record.Sent144h && !record.Sent72h {
		if postAge >= 144*time.Hour {
			// 已超过144小时，立即触发
			go handle144hTimer(s, post.ID, guildID, post.ChannelID)
		} else {
			// 创建剩余时间的计时器
			remaining := 144*time.Hour - postAge
			timerInfo.Timer144h = time.AfterFunc(remaining, func() {
				handle144hTimer(s, post.ID, guildID, post.ChannelID)
			})
			log.Printf("[NewCardEmoji] Created 144h timer for post %s, remaining: %v", post.ID, remaining)
		}
	}

	// 保存到活动计时器映射
	timersMutex.Lock()
	activeTimers[post.ID] = timerInfo
	timersMutex.Unlock()
}

// getRecentPosts 获取最近N小时内的帖子
func getRecentPosts(db *sql.DB, tableNames []string, hours int) ([]model.Post, error) {
	if len(tableNames) == 0 {
		return []model.Post{}, nil
	}

	cutoffTime := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()

	var allPosts []model.Post

	for _, tableName := range tableNames {
		query := `SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url
                  FROM "` + tableName + `"
                  WHERE timestamp >= ?
                  ORDER BY timestamp DESC`

		rows, err := db.Query(query, cutoffTime)
		if err != nil {
			log.Printf("[NewCardEmoji] Error querying table %s: %v", tableName, err)
			continue
		}

		for rows.Next() {
			var post model.Post
			err := rows.Scan(
				&post.ID,
				&post.Title,
				&post.Author,
				&post.AuthorID,
				&post.Content,
				&post.Tags,
				&post.MessageCount,
				&post.Timestamp,
				&post.CoverImageURL,
			)
			if err != nil {
				log.Printf("[NewCardEmoji] Error scanning row: %v", err)
				continue
			}
			allPosts = append(allPosts, post)
		}
		rows.Close()
	}

	return allPosts, nil
}

// GetActiveTimerCount 获取活动计时器数量（用于监控）
func GetActiveTimerCount() int {
	timersMutex.RLock()
	defer timersMutex.RUnlock()
	return len(activeTimers)
}
