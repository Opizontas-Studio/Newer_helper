package scanner

import (
	"discord-bot/model"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
)

const maxPartitionConcurrency = 45          // 每个服务器内最大并发分区扫描数
const maxThreadConcurrencyPerPartition = 16 // 每个分区内最大并发线程处理数

// processThreadsChunk 处理线程分片的函数
func processThreadsChunk(s *discordgo.Session, chunk ThreadChunk, existingThreads map[string]bool, existingThreadsMutex *sync.RWMutex, task PartitionTask, tableName string, done <-chan struct{}) {
	for _, thread := range chunk.Threads {
		select {
		case <-done:
			return
		default:
		}

		// 线程安全检查是否已存在
		existingThreadsMutex.RLock()
		_, exists := existingThreads[thread.ID]
		existingThreadsMutex.RUnlock()

		if exists {
			continue
		}

		firstMessage, err := s.ChannelMessage(thread.ID, thread.ID)
		if err != nil {
			log.Printf("Error getting first message for thread %s: %v", thread.ID, err)
			continue
		}

		var tagNames []string
		if thread.AppliedTags != nil {
			for _, tagID := range thread.AppliedTags {
				tagNames = append(tagNames, string(tagID))
			}
		}

		content := firstMessage.Content
		runes := []rune(content)
		if len(runes) > 512 {
			content = string(runes[:512])
		}

		var coverImageURL string
		if len(firstMessage.Attachments) > 0 {
			coverImageURL = firstMessage.Attachments[0].URL
		}

		post := model.Post{
			ID:            thread.ID,
			ChannelID:     thread.ParentID,
			Title:         thread.Name,
			Author:        firstMessage.Author.Username,
			AuthorID:      firstMessage.Author.ID,
			Content:       content,
			Tags:          strings.Join(tagNames, ","),
			MessageCount:  thread.MessageCount,
			Timestamp:     firstMessage.Timestamp.Unix(),
			CoverImageURL: coverImageURL,
		}

		if err := database.InsertPost(task.DB, post, tableName); err != nil {
			log.Printf("Error inserting post %s into database: %v", post.ID, err)
		} else {
			atomic.AddInt64(task.TotalNewPostsFound, 1)

			// 线程安全更新已存在的线程映射
			existingThreadsMutex.Lock()
			existingThreads[post.ID] = true
			existingThreadsMutex.Unlock()

			completedCount := atomic.LoadInt64(task.PartitionsDone)
			remainingCount := task.TotalPartitions - int(completedCount)
			log.Printf("Successfully saved post: %s to table %s 丨 当前完成 %d · 全部还剩 %d (分片 %d)",
				post.ID, tableName, completedCount, remainingCount, chunk.Index)
		}
	}
}

// chunkThreads 将线程列表分片
func chunkThreads(threads []*discordgo.Channel, chunkSize int) []ThreadChunk {
	var chunks []ThreadChunk
	for i := 0; i < len(threads); i += chunkSize {
		end := min(i+chunkSize, len(threads))
		chunks = append(chunks, ThreadChunk{
			Threads: threads[i:end],
			Index:   len(chunks),
		})
	}
	return chunks
}

func calculateOptimalThreadsPerPartition(remainingPartitions int) int {
	if remainingPartitions <= 0 {
		return maxThreadConcurrencyPerPartition
	}

	const minThreadsPerPartition = 4
	var threadsPerPartition int

	if remainingPartitions <= 3 {
		threadsPerPartition = maxThreadConcurrencyPerPartition
	} else if remainingPartitions <= 5 {
		threadsPerPartition = 12
	} else if remainingPartitions <= 10 {
		threadsPerPartition = 8
	} else {
		threadsPerPartition = minThreadsPerPartition
	}

	// 确保不超过最大限制
	if threadsPerPartition > maxThreadConcurrencyPerPartition {
		threadsPerPartition = maxThreadConcurrencyPerPartition
	}

	return threadsPerPartition
}

// worker 是工作池中的工作单元
func worker(id int, s *discordgo.Session, done <-chan struct{}, tasks <-chan PartitionTask, wg *sync.WaitGroup) {
	defer wg.Done()
	for task := range tasks {
		// 将完成计数的逻辑移到任务的最后
		defer atomic.AddInt64(task.PartitionsDone, 1)

		select {
		case <-done:
			log.Printf("Worker %d cancelling task for partition %s.", id, task.Key)
			return
		default:
			// 继续执行任务
		}

		startTime := time.Now()
		channelConfig := task.GuildConfig.Data[task.Key]
		channelID := channelConfig.ChannelID
		tableName := fmt.Sprintf("%s_%s", task.Key, channelID[len(channelID)-4:])

		existingThreads := make(map[string]bool)
		existingThreadsMutex := &sync.RWMutex{}

		if !task.IsFullScan {
			allPosts, err := database.GetAllPosts(task.DB, tableName)
			if err != nil {
				log.Printf("Error getting all posts for active scan from table %s: %v", tableName, err)
			} else {
				for _, post := range allPosts {
					existingThreads[post.ID] = true
				}
			}
		} else {
			for _, id := range channelConfig.ThreadIDs {
				existingThreads[id] = true
			}
		}

		// 并发处理线程列表的函数
		processThreadsConcurrently := func(threads []*discordgo.Channel) {
			if len(threads) == 0 {
				return
			}

			// 动态计算当前分区的最佳线程数（基于剩余分区数）
			completedPartitions := atomic.LoadInt64(task.PartitionsDone)
			remainingPartitions := task.TotalPartitions - int(completedPartitions)
			optimalThreadsPerPartition := calculateOptimalThreadsPerPartition(remainingPartitions)

			// 记录动态分配的线程数和分片情况
			log.Printf("分区 %s 线程分配: %d个线程处理器 | 线程数:%d | 剩余分区数:%d | 总分区数:%d",
				task.Key, optimalThreadsPerPartition, len(threads), remainingPartitions, task.TotalPartitions)

			// 计算每个分片的大小
			chunkSize := (len(threads) + optimalThreadsPerPartition - 1) / optimalThreadsPerPartition
			if chunkSize == 0 {
				chunkSize = 1
			}

			chunks := chunkThreads(threads, chunkSize)
			var chunkWg sync.WaitGroup

			// 限制并发数量
			concurrencyLimit := min(len(chunks), optimalThreadsPerPartition)
			semaphore := make(chan struct{}, concurrencyLimit)

			// 记录分片详细信息
			log.Printf("分区 %s 分片信息: 总共%d个分片, 每片大小%d, 并发限制%d, 剩余分区数:%d",
				task.Key, len(chunks), chunkSize, concurrencyLimit, remainingPartitions)

			for _, chunk := range chunks {
				chunkWg.Add(1)
				go func(chunk ThreadChunk) {
					defer chunkWg.Done()

					// 获取信号量
					semaphore <- struct{}{}
					defer func() { <-semaphore }()

					processThreadsChunk(s, chunk, existingThreads, existingThreadsMutex, task, tableName, done)
				}(chunk)
			}

			chunkWg.Wait()
		}

		activeThreads, err := s.ThreadsActive(channelConfig.ChannelID)
		if err != nil {
			log.Printf("Error getting threads for channel %s: %v", channelConfig.ChannelID, err)
			return
		}
		processThreadsConcurrently(activeThreads.Threads)

		if task.IsFullScan {
			var before *time.Time
			for {
				select {
				case <-done:
					log.Println("Scan cancelled during pagination.")
					return
				default:
				}

				archivedThreads, err := s.ThreadsArchived(channelConfig.ChannelID, before, 100)
				if err != nil {
					log.Printf("Error getting archived threads for channel %s: %v", channelConfig.ChannelID, err)
					break
				}

				if len(archivedThreads.Threads) == 0 {
					break
				}

				processThreadsConcurrently(archivedThreads.Threads)

				if !archivedThreads.HasMore {
					break
				}

				lastThread := archivedThreads.Threads[len(archivedThreads.Threads)-1]
				if lastThread.ThreadMetadata == nil {
					log.Printf("Archived thread %s has no metadata, stopping pagination.", lastThread.ID)
					break
				}
				before = &lastThread.ThreadMetadata.ArchiveTimestamp
			}
		}

		// 这个日志现在只用于记录单个分区的耗时
		log.Printf("分区 %s (%s) 扫描完成, 耗时: %v", task.Key, task.GuildConfig.Name, time.Since(startTime))
	}
}
