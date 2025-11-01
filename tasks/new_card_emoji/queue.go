package new_card_emoji

import (
	"context"
	"fmt"
	"log"
	"newer_helper/model"
	"newer_helper/utils"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	globalQueue *MessageQueue
	globalState *TimerState
)

// InitQueue 初始化全局队列和状态
func InitQueue() error {
	globalQueue = NewMessageQueue()

	// 加载状态
	state, err := LoadState()
	if err != nil {
		return err
	}
	globalState = state

	return nil
}

// AddToQueue 添加帖子到全局队列
func AddToQueue(item *QueueItem) {
	if globalQueue == nil {
		log.Printf("[NewCardEmoji] Error: Queue not initialized")
		return
	}

	globalQueue.Enqueue(item)
	log.Printf("[NewCardEmoji] Added to queue: post=%s, trigger=%s, queue_size=%d",
		item.PostID, item.Trigger, globalQueue.Size())
}

// StartQueueProcessor 启动队列处理器
// 每5分钟处理一个队列项，每小时清理一次旧记录
func StartQueueProcessor(s *discordgo.Session, cfg *model.Config, ctx context.Context) {
	log.Println("[NewCardEmoji] Starting queue processor...")

	queueTicker := time.NewTicker(5 * time.Minute)
	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer queueTicker.Stop()
	defer cleanupTicker.Stop()

	// 立即处理一次（如果队列中有项目）
	go processNextItem(s, cfg.LogChannelID)

	// 立即清理一次旧记录
	go performCleanup()

	for {
		select {
		case <-queueTicker.C:
			go processNextItem(s, cfg.LogChannelID)
		case <-cleanupTicker.C:
			go performCleanup()
		case <-ctx.Done():
			log.Println("[NewCardEmoji] Queue processor stopped")
			return
		}
	}
}

// performCleanup 执行清理任务
func performCleanup() {
	if globalState == nil {
		return
	}

	log.Println("[NewCardEmoji] Running cleanup task...")
	deletedCount := CleanupOldRecords(globalState)

	// 如果有删除记录，保存状态
	if deletedCount > 0 {
		if err := SaveState(globalState); err != nil {
			log.Printf("[NewCardEmoji] Error saving state after cleanup: %v", err)
		} else {
			log.Printf("[NewCardEmoji] State saved after cleanup")
		}
	}
}

// processNextItem 处理队列中的下一个项目
func processNextItem(s *discordgo.Session, logChannelID string) {
	if globalQueue == nil {
		return
	}

	item := globalQueue.Dequeue()
	if item == nil {
		return // 队列为空
	}

	log.Printf("[NewCardEmoji] Processing queue item: post=%s, trigger=%s",
		item.PostID, item.Trigger)

	sent, err := processItem(s, logChannelID, item)
	if err != nil {
		log.Printf("[NewCardEmoji] Error processing item %s: %v", item.PostID, err)
		utils.LogError(s, logChannelID, "NewCardEmoji", "ProcessItem",
			fmt.Sprintf("Failed to send emoji to post <#%s> at %s: %v", item.PostID, item.Trigger, err))
		return
	}

	// 只有在实际发送了emoji时，才更新状态和记录成功日志
	if sent {
		// 更新状态并保存
		if item.Trigger == "72h" {
			globalState.UpdateSent72h(item.PostID)
		} else if item.Trigger == "144h" {
			globalState.UpdateSent144h(item.PostID)
		}

		if err := SaveState(globalState); err != nil {
			log.Printf("[NewCardEmoji] Error saving state: %v", err)
		}

		// 发送成功日志
		utils.LogInfo(s, logChannelID, "NewCardEmoji", "EmojiSent",
			fmt.Sprintf("Successfully sent emoji to archived post <#%s> at %s trigger", item.PostID, item.Trigger))

		log.Printf("[NewCardEmoji] Successfully processed item: post=%s, trigger=%s",
			item.PostID, item.Trigger)
	} else {
		log.Printf("[NewCardEmoji] Skipped item (not sent): post=%s, trigger=%s",
			item.PostID, item.Trigger)
	}
}

// processItem 处理单个队列项
// 返回值: (sent bool, err error)
// - sent: 是否实际发送了emoji（跳过的情况返回false）
// - err: 处理过程中的错误
func processItem(s *discordgo.Session, logChannelID string, item *QueueItem) (bool, error) {
	// 1. 检查线程状态（锁定和归档）
	locked, archived, err := CheckThreadStatus(s, item.PostID)
	if err != nil {
		return false, err
	}

	// 2. 如果线程已锁定，跳过
	if locked {
		log.Printf("[NewCardEmoji] Thread %s is locked, skipping", item.PostID)
		utils.LogInfo(s, logChannelID, "NewCardEmoji", "CheckLocked",
			fmt.Sprintf("Post <#%s> is locked, skipping emoji send", item.PostID))
		return false, nil
	}

	// 3. 如果线程未归档，跳过
	if !archived {
		log.Printf("[NewCardEmoji] Thread %s is not archived, skipping", item.PostID)
		utils.LogInfo(s, logChannelID, "NewCardEmoji", "CheckArchive",
			fmt.Sprintf("Post <#%s> is not archived, skipping emoji send", item.PostID))
		return false, nil
	}

	// 4. 解除归档
	if err := UnarchiveThread(s, item.PostID); err != nil {
		return false, err
	}

	// 等待一小段时间确保解除归档生效
	time.Sleep(2 * time.Second)

	// 5. 发送随机emoji
	if err := SendEmojiToThread(s, item.PostID, item.GuildID); err != nil {
		return false, err
	}

	return true, nil
}

// GetQueueSize 获取当前队列大小
func GetQueueSize() int {
	if globalQueue == nil {
		return 0
	}
	return globalQueue.Size()
}

// GetState 获取全局状态（只读）
func GetState() *TimerState {
	return globalState
}
