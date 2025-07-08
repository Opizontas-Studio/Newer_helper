package leaderboard

import (
	"database/sql"
	"discord-bot/model"
	"discord-bot/utils"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func HandleNewCardsInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b model.Bot) {
	// 权限检查
	config := b.GetConfig()
	serverConfig, ok := config.ServerConfigs[i.GuildID]
	if !ok {
		log.Printf("Guild config not found for guild ID: %s", i.GuildID)
		utils.SendErrorResponse(s, i, "服务器配置未找到。")
		return
	}
	permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, serverConfig.AdminRoleIDs, serverConfig.UserRoleIDs, config.DeveloperUserIDs, config.SuperAdminRoleIDs)
	if permissionLevel != utils.SuperAdminPermission && permissionLevel != utils.DeveloperPermission {
		utils.SendErrorResponse(s, i, "您没有权限使用此命令。")
		return
	}

	// 检查是否已存在排行榜
	state, err := utils.LoadLeaderboardState()
	if err == nil && state.MessageID != "" {
		// 如果文件存在且MessageID有效，则更新
		UpdateLeaderboard(b, state.GuildID)
		utils.SendSimpleResponse(s, i, "已更新现有的排行榜。")
		return
	} else if err == nil {
		// 如果文件存在但无效（例如空的JSON），则删除它
		log.Println("Found invalid leaderboard state file, deleting it.")
		_ = os.Remove(utils.LeaderboardStateFile)
	}

	// 如果不存在，创建一个新的
	message, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{buildLeaderboardEmbed(b, i.GuildID, b.GetDB())},
	})
	if err != nil {
		log.Printf("Error sending leaderboard message: %v", err)
		utils.SendErrorResponse(s, i, "创建排行榜时出错。")
		return
	}

	// 保存排行榜状态
	newState := model.LeaderboardState{
		GuildID:   i.GuildID,
		ChannelID: i.ChannelID,
		MessageID: message.ID,
	}
	if err := utils.SaveLeaderboardState(newState); err != nil {
		log.Printf("Error saving leaderboard state: %v", err)
	}

	utils.SendSimpleResponse(s, i, "已成功创建排行榜，将每 10 分钟自动更新。")
}

func UpdateLeaderboard(b model.Bot, guildID string) {
	state, err := utils.LoadLeaderboardState()
	if err != nil {
		log.Printf("Error loading leaderboard state for update: %v", err)
		return
	}

	embed := buildLeaderboardEmbed(b, guildID, b.GetDB())
	if embed == nil {
		log.Printf("Failed to build leaderboard embed for guild %s", guildID)
		return
	}
	embeds := []*discordgo.MessageEmbed{embed}
	_, err = b.GetSession().ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel: state.ChannelID,
		ID:      state.MessageID,
		Embeds:  &embeds,
	})
	if err != nil {
		log.Printf("Failed to edit leaderboard message for guild %s: %v", guildID, err)
	}
}

func buildLeaderboardEmbed(b model.Bot, guildID string, db *sql.DB) *discordgo.MessageEmbed {
	config := b.GetConfig()

	// 1. 从配置中获取所有相关的表名和映射
	var tableNames []string
	tableToChannel := make(map[string]string)
	channelToSection := make(map[string]string)
	if guildTasks, ok := config.TaskConfig[guildID]; ok {
		for sectionName, sectionData := range guildTasks.Data {
			if sectionData.TableName != "" {
				tableNames = append(tableNames, sectionData.TableName)
				tableToChannel[sectionData.TableName] = sectionData.ChannelID
			}
			channelToSection[sectionData.ChannelID] = sectionName
		}
	}

	if len(tableNames) == 0 {
		log.Printf("No tables configured for leaderboard in guild %s", guildID)
		return &discordgo.MessageEmbed{
			Title:       "🏆 新卡速递排行榜",
			Description: "错误：未配置任何用于统计的数据表。",
			Color:       0xff0000,
		}
	}

	// 2. 获取统计数据
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	sevenDaysAgo := today.AddDate(0, 0, -7)

	todayCount, err := utils.CountPostsInTimeRange(db, tableNames, today.Unix(), now.Unix())
	if err != nil {
		log.Printf("Error counting posts for today: %v", err)
	}
	yesterdayCount, err := utils.CountPostsInTimeRange(db, tableNames, yesterday.Unix(), today.Unix())
	if err != nil {
		log.Printf("Error counting posts for yesterday: %v", err)
	}
	last7DaysCount, err := utils.CountPostsInTimeRange(db, tableNames, sevenDaysAgo.Unix(), now.Unix())
	if err != nil {
		log.Printf("Error counting posts for last 7 days: %v", err)
	}

	// 3. 获取最新的12张卡片
	posts, err := utils.GetLatestPosts(db, tableNames, 12)
	if err != nil {
		log.Printf("Error getting latest posts: %v", err)
		return nil // or return an embed with error
	}

	// 4. 加载tag mapping
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

	// 5. 构建Embed
	embed := &discordgo.MessageEmbed{
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
						"**近7日新增**: %d",
					todayCount, yesterdayCount, last7DaysCount,
				),
				Inline: false,
			},
		},
	}

	if len(posts) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   " ", // Separator
			Value:  "**最新卡片**",
			Inline: false,
		})
	}

	for i := range posts {
		post := &posts[i]
		post.ChannelID = tableToChannel[post.TableName]

		sectionName, ok := channelToSection[post.ChannelID]
		if !ok {
			sectionName = "未知区"
		}

		var tagNames []string
		if tagMapping != nil {
			tagIDs := strings.Split(post.Tags, ",")
			if sectionTags, ok := tagMapping[sectionName]; ok {
				for _, tagID := range tagIDs {
					trimmedTagID := strings.TrimSpace(tagID)
					if tagName, ok := sectionTags[trimmedTagID]; ok {
						tagNames = append(tagNames, tagName)
					}
				}
			}
		}
		if len(tagNames) == 0 {
			tagNames = append(tagNames, "无标签")
		}

		postURL := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, post.ChannelID, post.ID)
		value := fmt.Sprintf("> %s · <t:%d:R>\n> [跳转帖子](%s)\n> %s · %s",
			post.Author,
			post.Timestamp,
			postURL,
			sectionName,
			strings.Join(tagNames, ", "),
		)
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   post.Title,
			Value:  value,
			Inline: false,
		})
	}

	return embed
}
