package leaderboard

import (
	"discord-bot/handlers/leaderboard/latest_posts"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
)

func HandleNewCardsInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b model.Bot) {
	// 权限检查
	config := b.GetConfig()
	serverConfig, ok := config.ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Guild config not found for guild ID: %s", i.GuildID)
		utils.SendEphemeralResponse(s, i, "服务器配置未找到 ")
		return
	}
	permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, config.DeveloperUserIDs, config.SuperAdminRoleIDs)
	if permissionLevel != utils.SuperAdminPermission && permissionLevel != utils.DeveloperPermission {
		utils.SendEphemeralResponse(s, i, "您没有权限使用此命令 ")
		return
	}

	// 检查是否已存在排行榜
	states, err := utils.LoadLeaderboardState()
	if err != nil {
		utils.SendEphemeralResponse(s, i, "加载排行榜状态时出错 ")
		log.Printf("Error loading leaderboard states: %v", err)
		return
	}

	if state, ok := states[i.GuildID]; ok && state.MessageID != "" {
		// 如果当前服务器的排行榜已存在，则更新
		UpdateLeaderboard(b, i.GuildID)
		utils.SendEphemeralResponse(s, i, "已更新现有的排行榜 ")
		return
	}

	// 如果不存在，创建一个新的
	config = b.GetConfig()
	embeds := buildLeaderboardEmbeds(i.GuildID, config)
	if len(embeds) == 0 {
		utils.SendEphemeralResponse(s, i, "创建排行榜时出错, 无法生成 embeds ")
		return
	}

	message, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Embeds: embeds,
	})
	if err != nil {
		log.Printf("Error sending leaderboard message: %v", err)
		utils.SendEphemeralResponse(s, i, "创建排行榜时出错 ")
		return
	}
	// 保存排行榜状态
	states[i.GuildID] = model.LeaderboardState{
		GuildID:   i.GuildID,
		ChannelID: i.ChannelID,
		MessageID: message.ID,
	}
	if err := utils.SaveLeaderboardState(states); err != nil {
		log.Printf("Error saving leaderboard state: %v", err)
	}

	utils.SendEphemeralResponse(s, i, "已成功创建排行榜，将每 10 分钟自动更新 ")
}

func UpdateLeaderboard(b model.Bot, guildID string) {
	states, err := utils.LoadLeaderboardState()
	if err != nil {
		log.Printf("Error loading leaderboard state for update: %v", err)
		return
	}

	state, ok := states[guildID]
	if !ok {
		log.Printf("No leaderboard state found for guild %s", guildID)
		return
	}

	config := b.GetConfig()
	embeds := buildLeaderboardEmbeds(guildID, config)
	if len(embeds) == 0 {
		log.Printf("Failed to build leaderboard embeds for guild %s", guildID)
		return
	}
	// 只更新第一个消息（排行榜统计）
	_, err = b.GetSession().ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel: state.ChannelID,
		ID:      state.MessageID,
		Embeds:  &embeds, // 发送所有embeds
	})
	if err != nil {
		log.Printf("Failed to edit leaderboard message for guild %s: %v", guildID, err)
	}
}

func buildLeaderboardEmbeds(guildID string, cfg *model.Config) []*discordgo.MessageEmbed {
	var embeds []*discordgo.MessageEmbed

	// 1. 从配置中获取数据库路径
	threadGuildConfig, ok := cfg.ThreadConfig[guildID]
	if !ok || threadGuildConfig.Database == "" {
		log.Printf("No database path configured for guild %s", guildID)
		return []*discordgo.MessageEmbed{{Title: "错误", Description: "当前服务器未配置数据库路径 ", Color: 0xff0000}}
	}
	dbPath := threadGuildConfig.Database

	// 2. 从独立的映射文件中加载数据库表映射
	dbMapping, err := utils.LoadDatabaseMapping()
	if err != nil {
		log.Printf("Error loading database mapping: %v", err)
		return []*discordgo.MessageEmbed{{Title: "错误", Description: "无法加载数据库映射文件 ", Color: 0xff0000}}
	}

	guildMapping, ok := dbMapping[guildID]
	if !ok {
		log.Printf("No database mapping found for guild %s", guildID)
		return []*discordgo.MessageEmbed{{Title: "错误", Description: "当前服务器未配置数据库映射 ", Color: 0xff0000}}
	}

	var tableNames []string
	if len(guildMapping.DataBaseTableNameMapping) > 0 {
		for tableName := range guildMapping.DataBaseTableNameMapping {
			tableNames = append(tableNames, tableName)
		}
	} else if threadGuildConfig.TableName != "" {
		// Fallback to using the table name from thread config if mapping is empty
		tableNames = append(tableNames, threadGuildConfig.TableName)
	}

	if len(tableNames) == 0 {
		log.Printf("No tables configured for leaderboard in guild %s", guildID)
		return []*discordgo.MessageEmbed{{
			Title:       "🏆 新卡速递排行榜",
			Description: "错误：未配置任何用于统计的数据表 ",
			Color:       0xff0000,
		}}
	}

	// 3. 初始化特定于服务器的数据库连接
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Printf("Error initializing database for guild %s at %s: %v", guildID, dbPath, err)
		return []*discordgo.MessageEmbed{{Title: "错误", Description: "无法连接到服务器的数据库 ", Color: 0xff0000}}
	}
	defer db.Close()

	// Open guilds.db to get ad
	guildsDB, err := database.InitDB("data/guilds.db")
	if err != nil {
		log.Printf("Could not open guilds.db: %v", err)
	} else {
		defer guildsDB.Close()
		ad, err := database.GetRandomEnabledLeaderboardAd(guildsDB, guildID)
		if err != nil {
			log.Printf("Error getting leaderboard ad for guild %s: %v", guildID, err)
		}
		if ad != nil {
			var adEmbed discordgo.MessageEmbed
			// Try to unmarshal as an embed first
			err := json.Unmarshal([]byte(ad.Content), &adEmbed)
			if err != nil {
				adEmbed = discordgo.MessageEmbed{
					Title:       "📜 服务器内广告",
					Description: ad.Content,
					Color:       0x7289DA, // Discord Blurple
				}
			}
			if ad.ImageURL != "" {
				adEmbed.Image = &discordgo.MessageEmbedImage{
					URL: ad.ImageURL,
				}
			}
			embeds = append(embeds, &adEmbed)
		}
	}

	// 4. 获取统计数据
	now := time.Now()
	// 以凌晨 4:00 为分界线
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	if now.Hour() < 4 {
		// 如果当前时间在凌晨 4 点之前，则“今天”的开始时间是昨天的凌晨 4 点
		todayStart = todayStart.AddDate(0, 0, -1)
	}
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	threeDaysAgo := todayStart.AddDate(0, 0, -3)
	sevenDaysAgo := todayStart.AddDate(0, 0, -7)

	// 今日数据
	todayCount, err := database.CountPostsInTimeRange(db, tableNames, todayStart.Unix(), now.Unix())
	if err != nil {
		log.Printf("Error counting posts for today from DB: %v", err)
	}
	// 昨日数据
	yesterdayCount, err := database.CountPostsInTimeRange(db, tableNames, yesterdayStart.Unix(), todayStart.Unix())
	if err != nil {
		log.Printf("Error counting posts for yesterday from DB: %v", err)
	}
	// 近3日数据
	last3DaysCount, err := database.CountPostsInTimeRange(db, tableNames, threeDaysAgo.Unix(), now.Unix())
	if err != nil {
		log.Printf("Error counting posts for last 3 days from DB: %v", err)
	}
	last7DaysCount, err := database.CountPostsInTimeRange(db, tableNames, sevenDaysAgo.Unix(), now.Unix())
	if err != nil {
		log.Printf("Error counting posts for last 7 days from DB: %v", err)
	}

	// 5. 加载tag mapping
	tagMappingPath := fmt.Sprintf("data/tag_mapping/%s_config.json", guildID)
	tagMappingData, err := os.ReadFile(tagMappingPath)
	var tagMapping map[string]map[string]string
	if err == nil {
		if err := json.Unmarshal(tagMappingData, &tagMapping); err != nil {
			log.Printf("Error unmarshalling tag mapping data for guild %s: %v", guildID, err)
		}
	} else {
		log.Printf("Could not read tag mapping file for guild %s: %v", guildID, err)
	}

	// 6. 构建Embeds
	// Embed 1: 排行榜统计
	leaderboardEmbed := &discordgo.MessageEmbed{
		Title:       "🏆 新卡速递排行榜",
		Description: fmt.Sprintf("最后更新于 <t:%d:R>", now.Unix()),
		Color:       0x00ff00,
		Timestamp:   now.Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "📊 数据统计",
				Value: fmt.Sprintf(
					"**今日新增**: %d\n"+
						"**昨日新增**: %d\n"+
						"**近3日新增**: %d\n"+
						"**近7日新增**: %d",
					todayCount, yesterdayCount, last3DaysCount, last7DaysCount,
				),
				Inline: false,
			},
		},
	}

	// Add the main leaderboard embed
	embeds = append(embeds, leaderboardEmbed)

	// Embed 2: 最新卡片列表
	latestPostsEmbed, err := latest_posts.CreateLatestPostsEmbed(guildID)
	if err != nil {
		log.Printf("Error creating latest posts embed: %v", err)
	}
	if latestPostsEmbed != nil {
		embeds = append(embeds, latestPostsEmbed)
	}

	return embeds
}
