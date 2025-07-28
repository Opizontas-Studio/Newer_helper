package scanner

import (
	"context"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	configMutex = &sync.Mutex{}
)

// AddThreadToExclusionList 将 threadID 添加到指定 guild 和 channel 的排除列表
func AddThreadToExclusionList(guildID, channelKey, threadID string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	filePath := "data/task_config.json"
	file, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("无法读取配置文件: %w", err)
	}

	var allConfigs map[string]GuildConfig
	if err := json.Unmarshal(file, &allConfigs); err != nil {
		return fmt.Errorf("无法解析配置文件: %w", err)
	}

	guildConfig, ok := allConfigs[guildID]
	if !ok {
		return fmt.Errorf("未找到 Guild ID: %s", guildID)
	}

	channelConfig, ok := guildConfig.Data[channelKey]
	if !ok {
		return fmt.Errorf("未找到 Channel Key: %s", channelKey)
	}

	// 检查 threadID 是否已存在
	for _, id := range channelConfig.ThreadIDs {
		if id == threadID {
			return nil
		}
	}

	channelConfig.ThreadIDs = append(channelConfig.ThreadIDs, threadID)
	guildConfig.Data[channelKey] = channelConfig
	allConfigs[guildID] = guildConfig

	updatedData, err := json.MarshalIndent(allConfigs, "", "  ")
	if err != nil {
		return fmt.Errorf("无法序列化配置文件: %w", err)
	}

	if err := os.WriteFile(filePath, updatedData, 0644); err != nil {
		return fmt.Errorf("无法写入配置文件: %w", err)
	}

	return nil
}

// worker 是工作池中的工作单元
func Scan(s *discordgo.Session, logChannelID string, scanMode string, targetGuildID string, ctx context.Context) {
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
	var totalNewPostsFound, partitionsDone int64

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

	// 预先计算总分区数
	totalPartitions := 0
	for _, guildConfig := range configsToScan {
		totalPartitions += len(guildConfig.Data)
	}

	var guildWg sync.WaitGroup
	for guildID, guildConfig := range configsToScan {
		guildWg.Add(1)
		go func(guildID string, guildConfig GuildConfig) {
			defer guildWg.Done()

			select {
			case <-ctx.Done():
				log.Println("Scan cancelled for guild:", guildConfig.Name)
				return
			default:
			}

			db, err := database.InitDB(fmt.Sprintf("./data/%s.db", guildID))
			if err != nil {
				log.Printf("Error initializing database for guild %s: %v", guildID, err)
				return
			}
			defer db.Close()

			keys := make([]string, 0, len(guildConfig.Data))
			for k := range guildConfig.Data {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			tasks := make(chan PartitionTask, len(keys))
			var workerWg sync.WaitGroup

			// 启动 worker 池
			for i := 1; i <= maxPartitionConcurrency; i++ {
				workerWg.Add(1)
				go worker(i, s, ctx, tasks, &workerWg)
			}

			// 分发任务
			for _, key := range keys {
				tasks <- PartitionTask{
					GuildConfig:        guildConfig,
					Key:                key,
					DB:                 db,
					IsFullScan:         isFullScan,
					ScanType:           scanType,
					TotalPartitions:    totalPartitions,
					PartitionsDone:     &partitionsDone,
					TotalNewPostsFound: &totalNewPostsFound,
				}
			}
			close(tasks) // 所有任务分发完毕，关闭 channel

			workerWg.Wait() // 等待所有 worker 完成

		}(guildID, guildConfig)
	}
	guildWg.Wait()

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
		totalPartitions, // 使用预先计算的总数
		atomic.LoadInt64(&totalNewPostsFound),
		time.Since(totalScanStartTime),
	)

	log.Println(summaryMessage)
	if logChannelID != "" {
		if err := utils.LogInfo(s, logChannelID, "扫描模块", "最终总结报告", summaryMessage); err != nil {
			log.Printf("Failed to send final summary report: %v", err)
		}
	}

	lockData := make(map[string]any)
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
