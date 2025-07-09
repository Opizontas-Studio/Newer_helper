package scanner

import (
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
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

func Scan(s *discordgo.Session, logChannelID string, scanMode string, targetGuildID string, done <-chan struct{}) {
	targetServer := "所有服务器"
	if targetGuildID != "" {
		targetServer = targetGuildID
	}
	startMessage := fmt.Sprintf("扫描已开始。模式: %s, 目标: %s", scanMode, targetServer)
	if err := utils.LogInfo(s, logChannelID, "扫描模块", "扫描开始", startMessage); err != nil {
		log.Printf("Failed to send scan start log: %v", err)
	}

	isFullScan := scanMode == "full"
	scanType := "活跃帖"
	if isFullScan {
		scanType = "全区"
	}

	totalScanStartTime := time.Now()
	var totalPartitionsScanned, totalNewPostsFound int

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
		// Check for shutdown signal before processing a new guild
		select {
		case <-done:
			log.Println("Scan cancelled.")
			return
		default:
		}

		db, err := database.InitDB(fmt.Sprintf("./data/%s.db", guildID))
		if err != nil {
			log.Printf("Error initializing database for guild %s: %v", guildID, err)
			continue
		}
		defer db.Close()

		keys := make([]string, 0, len(guildConfig.Data))
		for k := range guildConfig.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, key := range keys {
			// Check for shutdown signal before processing a new partition
			select {
			case <-done:
				log.Println("Scan cancelled.")
				return
			default:
			}

			totalPartitionsScanned++
			startTime := time.Now()
			channelConfig := guildConfig.Data[key]
			channelID := channelConfig.ChannelID
			tableName := fmt.Sprintf("%s_%s", key, channelID[len(channelID)-4:])

			existingThreads := make(map[string]bool)
			if !isFullScan {
				allPosts, err := database.GetAllPosts(db, tableName)
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

			processThreads := func(threads []*discordgo.Channel) {
				for _, thread := range threads {
					if _, exists := existingThreads[thread.ID]; exists {
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
					if err := database.InsertPost(db, post, tableName); err != nil {
						log.Printf("Error inserting post %s into database: %v", post.ID, err)
					} else {
						totalNewPostsFound++
						fmt.Printf("Successfully saved post: %s to table %s\n", post.ID, tableName)
						existingThreads[post.ID] = true
					}
				}
			}

			activeThreads, err := s.ThreadsActive(channelConfig.ChannelID)
			if err != nil {
				log.Printf("Error getting threads for channel %s: %v", channelConfig.ChannelID, err)
				continue
			}
			processThreads(activeThreads.Threads)

			if isFullScan {
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

					processThreads(archivedThreads.Threads)

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

			nextGroup := "无"
			if i+1 < len(keys) {
				nextGroup = keys[i+1]
			}
			logMessage := fmt.Sprintf("配置文件：%s，在 %s 模式下的分区：%s 扫描完成\n耗时：%v ", guildConfig.Name, scanType, key, time.Since(startTime))
			log.Printf("%s接下来扫描：%s", logMessage, nextGroup)
		}
	}

	var targetSummary string
	if targetGuildID == "" {
		targetSummary = "所有服务器"
	} else {
		serverNames := make([]string, 0, len(configsToScan))
		for _, config := range configsToScan {
			serverNames = append(serverNames, config.Name)
		}
		if len(serverNames) > 0 {
			targetSummary = strings.Join(serverNames, ", ")
		} else {
			targetSummary = targetGuildID
		}
	}

	summaryMessage := fmt.Sprintf(
		"**%s** 模式扫描完成总结报告\n- **目标**: %s\n- **服务器数**: %d\n- **分区数**: %d\n- **新帖数**: %d\n- **总耗时**: %v",
		scanType,
		targetSummary,
		len(configsToScan),
		totalPartitionsScanned,
		totalNewPostsFound,
		time.Since(totalScanStartTime),
	)

	log.Println(summaryMessage)
	if logChannelID != "" {
		if err := utils.LogInfo(s, logChannelID, "扫描模块", "最终总结报告", summaryMessage); err != nil {
			log.Printf("Failed to send final summary report: %v", err)
		}
	}

	lockData := make(map[string]interface{})
	lockFile, err := os.ReadFile("data/scan_lock.json")
	if err == nil {
		json.Unmarshal(lockFile, &lockData)
	}
	lockData["scan_mode"] = scanMode
	lockData["timestamp"] = time.Now().Unix()

	lockFile, err = json.MarshalIndent(lockData, "", "  ")
	if err != nil {
		log.Printf("Error marshalling lock file data: %v", err)
		return
	}

	err = os.WriteFile("data/scan_lock.json", lockFile, 0644)
	if err != nil {
		log.Printf("Error writing lock file: %v", err)
	}
}

func CleanOldPosts(s *discordgo.Session, logChannelID string, done <-chan struct{}) {
	const postDir = "data/new_post/"
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour).Unix()

	files, err := os.ReadDir(postDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		utils.LogError(s, logChannelID, "CleanPosts", "ReadDir", fmt.Sprintf("Error reading post directory %s: %v", postDir, err))
		return
	}

	for _, file := range files {
		select {
		case <-done:
			log.Println("CleanOldPosts cancelled.")
			return
		default:
		}

		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := fmt.Sprintf("%s%s", postDir, file.Name())
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			utils.LogError(s, logChannelID, "CleanPosts", "ReadFile", fmt.Sprintf("Error reading post file %s: %v", filePath, err))
			continue
		}

		var posts []model.Post
		if err := json.Unmarshal(fileData, &posts); err != nil {
			utils.LogError(s, logChannelID, "CleanPosts", "Unmarshal", fmt.Sprintf("Error unmarshalling posts from %s: %v", filePath, err))
			continue
		}

		var newPosts []model.Post
		var removedPosts []string
		for _, post := range posts {
			if post.Timestamp >= sevenDaysAgo {
				newPosts = append(newPosts, post)
			} else {
				removedPosts = append(removedPosts, post.ID)
			}
		}

		if len(removedPosts) > 0 {
			utils.LogInfo(s, logChannelID, "CleanPosts", "Remove", fmt.Sprintf("Removing %d old posts from %s", len(removedPosts), filePath))
			if len(newPosts) == 0 {
				if err := os.Remove(filePath); err != nil {
					utils.LogError(s, logChannelID, "CleanPosts", "RemoveFile", fmt.Sprintf("Error removing empty post file %s: %v", filePath, err))
				} else {
					utils.LogInfo(s, logChannelID, "CleanPosts", "RemoveFile", fmt.Sprintf("Removed empty post file %s", filePath))
				}
			} else {
				jsonData, err := json.MarshalIndent(newPosts, "", "  ")
				if err != nil {
					utils.LogError(s, logChannelID, "CleanPosts", "Marshal", fmt.Sprintf("Error marshalling cleaned posts to JSON for %s: %v", filePath, err))
					continue
				}
				if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
					utils.LogError(s, logChannelID, "CleanPosts", "WriteFile", fmt.Sprintf("Error writing cleaned posts to file %s: %v", filePath, err))
				}
			}
		}
	}
}
