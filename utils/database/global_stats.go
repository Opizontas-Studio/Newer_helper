package database

import (
	"database/sql"
	"discord-bot/model"
	"fmt"
	"log"
	"sync"
	"time"
)

// GlobalStatsResult 包含全局统计结果
type GlobalStatsResult struct {
	TotalPosts     int                      // 总帖子数
	TodayPosts     int                      // 今日新增
	YesterdayPosts int                      // 昨日新增
	Last3DaysPosts int                      // 近3日新增
	Last7DaysPosts int                      // 近7日新增
	GuildStats     map[string]GuildStatInfo // 每个服务器的统计信息
	LatestPosts    []model.Post             // 最新帖子列表
	SourceGuilds   []string                 // 数据来源服务器列表
	Errors         []string                 // 错误信息
}

// GuildStatInfo 包含单个服务器的统计信息
type GuildStatInfo struct {
	GuildID        string
	TotalPosts     int
	TodayPosts     int
	YesterdayPosts int
	Last3DaysPosts int
	Last7DaysPosts int
	DatabasePath   string
	TableNames     []string
}

// GetGlobalStats 获取全局统计数据
func GetGlobalStats(guildMappings map[string]model.GuildMapping, threadConfigs map[string]model.ThreadGuildConfig) (*GlobalStatsResult, error) {
	result := &GlobalStatsResult{
		GuildStats:   make(map[string]GuildStatInfo),
		SourceGuilds: []string{},
		Errors:       []string{},
	}

	// 使用 WaitGroup 和 channel 进行并发查询
	var wg sync.WaitGroup
	statsChan := make(chan GuildStatInfo, len(guildMappings))
	errorsChan := make(chan string, len(guildMappings))

	// 计算时间范围
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	if now.Hour() < 4 {
		todayStart = todayStart.AddDate(0, 0, -1)
	}
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	threeDaysAgo := todayStart.AddDate(0, 0, -3)
	sevenDaysAgo := todayStart.AddDate(0, 0, -7)

	// 并发查询每个服务器的数据
	for guildID, mapping := range guildMappings {
		wg.Add(1)
		go func(gID string, gMapping model.GuildMapping) {
			defer wg.Done()

			// 获取数据库路径
			dbPath := gMapping.Database
			if dbPath == "" {
				// 尝试从 threadConfigs 获取
				if threadConfig, ok := threadConfigs[gID]; ok {
					dbPath = threadConfig.Database
				}
			}

			if dbPath == "" {
				errorsChan <- fmt.Sprintf("服务器 %s 没有配置数据库路径", gID)
				return
			}

			// 获取表名列表
			// 获取表名列表
			var tableNames []string
			// 优先从 databaseMapping.json 获取
			if len(gMapping.DataBaseTableNameMapping) > 0 {
				for tableName := range gMapping.DataBaseTableNameMapping {
					tableNames = append(tableNames, tableName)
				}
			} else if threadConfig, ok := threadConfigs[gID]; ok && threadConfig.TableName != "" && threadConfig.TableName != "all_posts" {
				// 其次从 thread_config.json 获取，但忽略 "all_posts"
				tableNames = append(tableNames, threadConfig.TableName)
			}

			// 连接数据库
			db, err := InitDB(dbPath)
			if err != nil {
				errorsChan <- fmt.Sprintf("服务器 %s 数据库连接失败: %v", gID, err)
				return
			}
			defer db.Close()

			// 如果没有从配置中获取到具体的表名，则从数据库中读取所有表
			if len(tableNames) == 0 {
				tableNames, err = GetAllTableNames(db)
				if err != nil {
					errorsChan <- fmt.Sprintf("服务器 %s 获取表名失败: %v", gID, err)
					return
				}
			}

			if len(tableNames) == 0 {
				errorsChan <- fmt.Sprintf("服务器 %s 的数据库中没有找到任何数据表", gID)
				return
			}

			// 查询统计数据
			guildStat := GuildStatInfo{
				GuildID:      gID,
				DatabasePath: dbPath,
				TableNames:   tableNames,
			}

			// 今日数据
			todayCount, err := CountPostsInTimeRange(db, tableNames, todayStart.Unix(), now.Unix())
			if err != nil {
				log.Printf("Error counting today posts for guild %s: %v", gID, err)
			} else {
				guildStat.TodayPosts = todayCount
			}

			// 昨日数据
			yesterdayCount, err := CountPostsInTimeRange(db, tableNames, yesterdayStart.Unix(), todayStart.Unix())
			if err != nil {
				log.Printf("Error counting yesterday posts for guild %s: %v", gID, err)
			} else {
				guildStat.YesterdayPosts = yesterdayCount
			}

			// 近3日数据
			last3DaysCount, err := CountPostsInTimeRange(db, tableNames, threeDaysAgo.Unix(), now.Unix())
			if err != nil {
				log.Printf("Error counting last 3 days posts for guild %s: %v", gID, err)
			} else {
				guildStat.Last3DaysPosts = last3DaysCount
			}

			// 近7日数据
			last7DaysCount, err := CountPostsInTimeRange(db, tableNames, sevenDaysAgo.Unix(), now.Unix())
			if err != nil {
				log.Printf("Error counting last 7 days posts for guild %s: %v", gID, err)
			} else {
				guildStat.Last7DaysPosts = last7DaysCount
			}

			// 总数
			totalCount, err := GetTotalPostCountFromTables(db, tableNames)
			if err != nil {
				log.Printf("Error counting total posts for guild %s: %v", gID, err)
			} else {
				guildStat.TotalPosts = totalCount
			}

			statsChan <- guildStat
		}(guildID, mapping)
	}

	// 等待所有查询完成
	go func() {
		wg.Wait()
		close(statsChan)
		close(errorsChan)
	}()

	// 收集结果
	for stat := range statsChan {
		result.GuildStats[stat.GuildID] = stat
		result.SourceGuilds = append(result.SourceGuilds, stat.GuildID)

		// 累加全局统计
		result.TotalPosts += stat.TotalPosts
		result.TodayPosts += stat.TodayPosts
		result.YesterdayPosts += stat.YesterdayPosts
		result.Last3DaysPosts += stat.Last3DaysPosts
		result.Last7DaysPosts += stat.Last7DaysPosts
	}

	// 收集错误
	for err := range errorsChan {
		result.Errors = append(result.Errors, err)
	}

	return result, nil
}

// GetGlobalLatestPosts 获取全局最新帖子
// func GetGlobalLatestPosts(guildMappings map[string]model.GuildMapping, threadConfigs map[string]model.ThreadGuildConfig, count int) ([]model.Post, error) {
// 	var allPosts []model.Post
// 	var mu sync.Mutex
// 	var wg sync.WaitGroup

// 	for guildID, mapping := range guildMappings {
// 		wg.Add(1)
// 		go func(gID string, gMapping model.GuildMapping) {
// 			defer wg.Done()

// 			// 获取数据库路径
// 			dbPath := gMapping.Database
// 			if dbPath == "" {
// 				if threadConfig, ok := threadConfigs[gID]; ok {
// 					dbPath = threadConfig.Database
// 				}
// 			}

// 			if dbPath == "" {
// 				return
// 			}

// 			// 获取表名列表
// 			var tableNames []string
// 			if len(gMapping.DataBaseTableNameMapping) > 0 {
// 				for tableName := range gMapping.DataBaseTableNameMapping {
// 					tableNames = append(tableNames, tableName)
// 				}
// 			} else if threadConfig, ok := threadConfigs[gID]; ok && threadConfig.TableName != "" && threadConfig.TableName != "all_posts" {
// 				tableNames = append(tableNames, threadConfig.TableName)
// 			}

// 			// 连接数据库
// 			db, err := InitDB(dbPath)
// 			if err != nil {
// 				log.Printf("Error connecting to database for guild %s: %v", gID, err)
// 				return
// 			}
// 			defer db.Close()

// 			if len(tableNames) == 0 {
// 				tableNames, err = GetAllTableNames(db)
// 				if err != nil {
// 					log.Printf("Error getting all table names for guild %s: %v", gID, err)
// 					return
// 				}
// 			}

// 			if len(tableNames) == 0 {
// 				return
// 			}

// 			// 获取最新帖子
// 			posts, err := GetLatestPosts(db, tableNames, count*2) // 获取更多以便后续筛选
// 			if err != nil {
// 				log.Printf("Error getting latest posts for guild %s: %v", gID, err)
// 				return
// 			}

// 			// 添加到结果中
// 			mu.Lock()
// 			allPosts = append(allPosts, posts...)
// 			mu.Unlock()
// 		}(guildID, mapping)
// 	}

// 	wg.Wait()

// 	// 按时间戳排序并限制数量
// 	if len(allPosts) > 0 {
// 		// 按时间戳降序排序
// 		for i := 0; i < len(allPosts)-1; i++ {
// 			for j := i + 1; j < len(allPosts); j++ {
// 				if allPosts[i].Timestamp < allPosts[j].Timestamp {
// 					allPosts[i], allPosts[j] = allPosts[j], allPosts[i]
// 				}
// 			}
// 		}

// 		// 限制返回数量
// 		if len(allPosts) > count {
// 			allPosts = allPosts[:count]
// 		}
// 	}

// 	return allPosts, nil
// }

// GetGlobalPostsInLast24Hours 获取全局过去24小时内的最新帖子
func GetGlobalPostsInLast24Hours(guildMappings map[string]model.GuildMapping, threadConfigs map[string]model.ThreadGuildConfig) ([]model.Post, error) {
	var allPosts []model.Post
	var mu sync.Mutex
	var wg sync.WaitGroup

	for guildID, mapping := range guildMappings {
		wg.Add(1)
		go func(gID string, gMapping model.GuildMapping) {
			defer wg.Done()

			// 获取数据库路径
			dbPath := gMapping.Database
			if dbPath == "" {
				if threadConfig, ok := threadConfigs[gID]; ok {
					dbPath = threadConfig.Database
				}
			}

			if dbPath == "" {
				return
			}

			// 获取表名列表
			var tableNames []string
			if len(gMapping.DataBaseTableNameMapping) > 0 {
				for tableName := range gMapping.DataBaseTableNameMapping {
					tableNames = append(tableNames, tableName)
				}
			} else if threadConfig, ok := threadConfigs[gID]; ok && threadConfig.TableName != "" && threadConfig.TableName != "all_posts" {
				tableNames = append(tableNames, threadConfig.TableName)
			}

			// 连接数据库
			db, err := InitDB(dbPath)
			if err != nil {
				log.Printf("Error connecting to database for guild %s: %v", gID, err)
				return
			}
			defer db.Close()

			if len(tableNames) == 0 {
				tableNames, err = GetAllTableNames(db)
				if err != nil {
					log.Printf("Error getting all table names for guild %s: %v", gID, err)
					return
				}
			}

			if len(tableNames) == 0 {
				return
			}

			// 获取过去24小时内的帖子
			posts, err := GetPostsInLast24Hours(db, tableNames)
			if err != nil {
				log.Printf("Error getting posts from last 24 hours for guild %s: %v", gID, err)
				return
			}

			// 添加到结果中
			mu.Lock()
			allPosts = append(allPosts, posts...)
			mu.Unlock()
		}(guildID, mapping)
	}

	wg.Wait()

	// 按时间戳降序排序
	if len(allPosts) > 0 {
		for i := 0; i < len(allPosts)-1; i++ {
			for j := i + 1; j < len(allPosts); j++ {
				if allPosts[i].Timestamp < allPosts[j].Timestamp {
					allPosts[i], allPosts[j] = allPosts[j], allPosts[i]
				}
			}
		}
	}

	return allPosts, nil
}

// GetServerStats 获取特定服务器的统计数据
func GetServerStats(guildID string, guildMappings map[string]model.GuildMapping, threadConfigs map[string]model.ThreadGuildConfig) (*GuildStatInfo, error) {
	// 获取服务器配置
	mapping, hasMappingConfig := guildMappings[guildID]
	threadConfig, hasThreadConfig := threadConfigs[guildID]

	// 获取数据库路径
	dbPath := ""
	if hasMappingConfig && mapping.Database != "" {
		dbPath = mapping.Database
	} else if hasThreadConfig && threadConfig.Database != "" {
		dbPath = threadConfig.Database
	}

	if dbPath == "" {
		return nil, fmt.Errorf("服务器 %s 没有配置数据库路径", guildID)
	}

	// 获取表名列表
	// 获取表名列表
	var tableNames []string
	if hasMappingConfig && len(mapping.DataBaseTableNameMapping) > 0 {
		for tableName := range mapping.DataBaseTableNameMapping {
			tableNames = append(tableNames, tableName)
		}
	} else if hasThreadConfig && threadConfig.TableName != "" && threadConfig.TableName != "all_posts" {
		tableNames = append(tableNames, threadConfig.TableName)
	}

	// 连接数据库
	db, err := InitDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 如果没有从配置中获取到具体的表名，则从数据库中读取所有表
	if len(tableNames) == 0 {
		tableNames, err = GetAllTableNames(db)
		if err != nil {
			return nil, fmt.Errorf("获取表名失败: %w", err)
		}
	}

	if len(tableNames) == 0 {
		return nil, fmt.Errorf("服务器 %s 的数据库中没有找到任何数据表", guildID)
	}

	// 计算时间范围
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	if now.Hour() < 4 {
		todayStart = todayStart.AddDate(0, 0, -1)
	}
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	threeDaysAgo := todayStart.AddDate(0, 0, -3)
	sevenDaysAgo := todayStart.AddDate(0, 0, -7)

	// 查询统计数据
	guildStat := &GuildStatInfo{
		GuildID:      guildID,
		DatabasePath: dbPath,
		TableNames:   tableNames,
	}

	// 各时间段统计
	guildStat.TodayPosts, _ = CountPostsInTimeRange(db, tableNames, todayStart.Unix(), now.Unix())
	guildStat.YesterdayPosts, _ = CountPostsInTimeRange(db, tableNames, yesterdayStart.Unix(), todayStart.Unix())
	guildStat.Last3DaysPosts, _ = CountPostsInTimeRange(db, tableNames, threeDaysAgo.Unix(), now.Unix())
	guildStat.Last7DaysPosts, _ = CountPostsInTimeRange(db, tableNames, sevenDaysAgo.Unix(), now.Unix())
	guildStat.TotalPosts, _ = GetTotalPostCountFromTables(db, tableNames)

	return guildStat, nil
}

// GetTotalPostCountFromTables 从指定的表中获取总帖子数
func GetTotalPostCountFromTables(db *sql.DB, tableNames []string) (int, error) {
	totalCount := 0
	for _, tableName := range tableNames {
		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM "` + tableName + `"`).Scan(&count)
		if err != nil {
			return 0, err
		}
		totalCount += count
	}
	return totalCount, nil
}

// Contains 检查切片中是否包含指定元素
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
