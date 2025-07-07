package commands

import (
	"discord-bot/model"
	"discord-bot/utils"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type GuildConfig struct {
	Name    string `json:"name"`
	GuildID string `json:"guilds_id"`
	Data    map[string]struct {
		ChannelID string   `json:"channel_id"`
		ThreadIDs []string `json:"thread_id"`
	} `json:"data"`
}

func Scan(s *discordgo.Session, logChannelID string, scanMode string, targetGuildID string) {
	isFullScan := scanMode == "full"
	scanType := "活跃帖"
	if isFullScan {
		scanType = "全区"
	}

	// Read the configuration file
	file, err := os.ReadFile("data/task_config.json")
	if err != nil {
		log.Printf("Error reading config file: %v", err)
		return
	}

	var allConfigs map[string]GuildConfig
	if err := json.Unmarshal(file, &allConfigs); err != nil {
		log.Printf("Error unmarshalling config: %v", err)
		return
	}

	configsToScan := make(map[string]GuildConfig)
	if targetGuildID != "" {
		if config, ok := allConfigs[targetGuildID]; ok {
			configsToScan[targetGuildID] = config
		} else {
			log.Printf("Target guild ID %s not found in config", targetGuildID)
			return
		}
	} else {
		configsToScan = allConfigs
	}

	for guildID, guildConfig := range configsToScan {
		func(guildID string, guildConfig GuildConfig) {
			db, err := utils.InitDB(fmt.Sprintf("./data/%s.db", guildID))
			if err != nil {
				log.Printf("Error initializing database for guild %s: %v", guildID, err)
				return
			}
			defer db.Close()

			// Get and sort the keys to have a predictable order
			keys := make([]string, 0, len(guildConfig.Data))
			for k := range guildConfig.Data {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for i, key := range keys {
				startTime := time.Now()
				channelConfig := guildConfig.Data[key]
				channelID := channelConfig.ChannelID
				tableName := fmt.Sprintf("%s_%s", key, channelID[len(channelID)-4:])
				// Get the threads (forum posts) from the channel
				threads, err := s.ThreadsActive(channelConfig.ChannelID)
				if err != nil {
					log.Printf("Error getting threads for channel %s: %v", channelConfig.ChannelID, err)
					continue
				}

				if isFullScan {
					var before *time.Time
					for {
						// Limit is 100, the max allowed by the API
						archivedThreads, err := s.ThreadsArchived(channelConfig.ChannelID, before, 100)
						if err != nil {
							log.Printf("Error getting archived threads for channel %s: %v", channelConfig.ChannelID, err)
							break
						}

						if len(archivedThreads.Threads) == 0 {
							break
						}

						threads.Threads = append(threads.Threads, archivedThreads.Threads...)

						if !archivedThreads.HasMore {
							break
						}

						// Set 'before' to the timestamp of the last thread we received to paginate.
						lastThread := archivedThreads.Threads[len(archivedThreads.Threads)-1]
						if lastThread.ThreadMetadata == nil {
							log.Printf("Archived thread %s has no metadata, stopping pagination.", lastThread.ID)
							break
						}
						before = &lastThread.ThreadMetadata.ArchiveTimestamp
					}
				}

				// Create a map for quick lookup of existing thread IDs
				existingThreads := make(map[string]bool)
				if !isFullScan {
					// For active scans, we still need to know what's already in the DB
					allPosts, err := utils.GetAllPosts(db, tableName)
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

				for _, thread := range threads.Threads {
					if _, exists := existingThreads[thread.ID]; exists {
						continue // Skip already scanned threads
					}

					// Get the first message of the thread
					firstMessage, err := s.ChannelMessage(thread.ID, thread.ID)
					if err != nil {
						log.Printf("Error getting first message for thread %s: %v", thread.ID, err)
						continue
					}

					// Extract tags
					var tagNames []string
					if thread.AppliedTags != nil {
						for _, tagID := range thread.AppliedTags {
							// In a real scenario, you might need to fetch tag names from the channel's available tags
							tagNames = append(tagNames, string(tagID))
						}
					}

					content := firstMessage.Content
					if len(content) > 512 {
						content = content[:512]
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
					if err := utils.InsertPost(db, post, tableName); err != nil {
						log.Printf("Error inserting post %s into database: %v", post.ID, err)
					} else {
						fmt.Printf("Successfully saved post: %s to table %s\n", post.ID, tableName)
					}
				}
				nextGroup := "无"
				if i+1 < len(keys) {
					nextGroup = keys[i+1]
				}
				logMessage := fmt.Sprintf("配置文件：%s，在 %s 模式下的分区：%s 扫描完成，总计 %d 个条目\n耗时：%v。", guildConfig.Name, scanType, key, len(threads.Threads), time.Since(startTime))
				log.Printf("%s接下来扫描：%s", logMessage, nextGroup)
				if logChannelID != "" {
					err := utils.LogInfo(s, logChannelID, "扫描模块", "扫描完成", logMessage)
					if err != nil {
						log.Printf("Failed to send scan completion log: %v", err)
					}
				}
			}

			// After all guilds are scanned, write the lock file
			lockData := map[string]interface{}{
				"scan_mode": scanMode,
				"timestamp": time.Now().Unix(),
			}
			lockFile, err := json.MarshalIndent(lockData, "", "  ")
			if err != nil {
				log.Printf("Error marshalling lock file data: %v", err)
				return
			}

			err = os.WriteFile("data/scan_lock.json", lockFile, 0644)
			if err != nil {
				log.Printf("Error writing lock file: %v", err)
			}
		}(guildID, guildConfig)
	}
}
