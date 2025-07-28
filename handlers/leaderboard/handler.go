package leaderboard

import (
	"discord-bot/handlers/leaderboard/latest_posts"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func HandleNewCardsInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b model.Bot) {
	// 1. 解析选项
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	scope := "current"
	if opt, ok := optionMap["scope"]; ok {
		scope = opt.StringValue()
	}

	targetGuildID := i.GuildID
	if opt, ok := optionMap["server_id"]; ok {
		targetGuildID = opt.StringValue()
		scope = "server" // 如果提供了server_id，则将范围覆盖为server
	}

	// 2. 权限检查
	config := b.GetConfig()
	permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, nil, nil, config.DeveloperUserIDs, config.SuperAdminRoleIDs)

	switch scope {
	case "global":
		if permissionLevel != utils.DeveloperPermission {
			utils.SendEphemeralResponse(s, i, "您没有权限查看全局排行榜。")
			return
		}
		targetGuildID = "global" // 特殊ID用于全局排行榜
	case "server":
		// 检查是否有权查看特定服务器
		targetServerConfig, ok := config.ServerConfigs[targetGuildID]
		if !ok {
			utils.SendEphemeralResponse(s, i, "找不到目标服务器的配置。")
			return
		}
		// 允许开发者或目标服务器的管理员
		isDeveloper := permissionLevel == utils.DeveloperPermission

		member, err := s.GuildMember(targetGuildID, i.Member.User.ID)
		isTargetAdmin := false
		if err == nil {
			targetPermissionLevel := utils.CheckPermission(member.Roles, i.Member.User.ID, targetServerConfig.AdminRoleIDs, nil, config.DeveloperUserIDs, config.SuperAdminRoleIDs)
			if targetPermissionLevel == utils.AdminPermission || targetPermissionLevel == utils.SuperAdminPermission {
				isTargetAdmin = true
			}
		}

		if !isDeveloper && !isTargetAdmin {
			utils.SendEphemeralResponse(s, i, "您没有权限查看该服务器的排行榜。")
			return
		}
	default: // current scope
		currentServerConfig, ok := config.ServerConfigs[i.GuildID]
		if !ok {
			utils.SendEphemeralResponse(s, i, "当前服务器配置未找到。")
			return
		}
		permissionLevel = utils.CheckPermission(i.Member.Roles, i.Member.User.ID, currentServerConfig.AdminRoleIDs, nil, config.DeveloperUserIDs, config.SuperAdminRoleIDs)
		if permissionLevel != utils.SuperAdminPermission && permissionLevel != utils.DeveloperPermission && permissionLevel != utils.AdminPermission {
			utils.SendEphemeralResponse(s, i, "您没有权限使用此命令。")
			return
		}
	}

	// 立即发送一个临时的确认消息
	utils.SendEphemeralResponse(s, i, "正在生成排行榜...")

	// 3. 构建排行榜
	embeds, err := buildLeaderboardEmbeds(targetGuildID, config)
	if err != nil {
		content := fmt.Sprintf("创建排行榜时出错: %v", err)
		utils.SendEphemeralResponse(s, i, content) // 告知用户错误
		return
	}
	if len(embeds) == 0 {
		utils.SendEphemeralResponse(s, i, "无法生成排行榜，可能是因为没有数据。")
		return
	}

	// 4. 检查现有状态并发送/更新消息
	states, _ := utils.LoadLeaderboardState()
	if states == nil {
		states = make(map[string]model.LeaderboardState)
	}
	key := targetGuildID

	var message *discordgo.Message
	if state, ok := states[key]; ok {
		// 如果状态存在，编辑现有消息
		message, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel: state.ChannelID,
			ID:      state.MessageID,
			Embeds:  &embeds,
		})
		if err != nil {
			log.Printf("Error editing leaderboard message, attempting to send a new one: %v", err)
			// 如果编辑失败（例如消息被删除），则尝试发送新消息
			message, err = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
				Embeds: embeds,
			})
		}
	} else {
		// 如果状态不存在，发送新消息
		message, err = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
			Embeds: embeds,
		})
	}

	if err != nil {
		log.Printf("Error sending or editing leaderboard message: %v", err)
		utils.SendEphemeralResponse(s, i, "发送排行榜消息时出错。")
		return
	}

	// 5. 保存或更新状态
	states[key] = model.LeaderboardState{
		GuildID:   key,
		ChannelID: message.ChannelID,
		MessageID: message.ID,
	}
	if err := utils.SaveLeaderboardState(states); err != nil {
		log.Printf("Error saving leaderboard state: %v", err)
		utils.SendEphemeralResponse(s, i, "保存排行榜状态时出错。")
	}

	// 最终确认
	finalContent := fmt.Sprintf("排行榜已在 <#%s> 更新。", message.ChannelID)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &finalContent,
	})
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
	embeds, err := buildLeaderboardEmbeds(guildID, config)
	if err != nil {
		log.Printf("Failed to build leaderboard embeds for guild %s: %v", guildID, err)
		return
	}
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

func buildLeaderboardEmbeds(targetGuildID string, cfg *model.Config) ([]*discordgo.MessageEmbed, error) {
	if targetGuildID == "global" {
		return buildGlobalLeaderboardEmbeds(cfg)
	}
	return buildSingleGuildLeaderboardEmbeds(targetGuildID, cfg)
}

func buildSingleGuildLeaderboardEmbeds(guildID string, cfg *model.Config) ([]*discordgo.MessageEmbed, error) {
	var embeds []*discordgo.MessageEmbed
	now := time.Now()

	// 1. 获取统计数据
	dbMapping, err := utils.LoadDatabaseMapping()
	if err != nil {
		return nil, fmt.Errorf("无法加载数据库映射文件: %w", err)
	}

	stats, err := database.GetServerStats(guildID, dbMapping, cfg.ThreadConfig)
	if err != nil {
		return nil, fmt.Errorf("获取服务器数据时出错: %w", err)
	}

	// 2. 构建主-embed
	guildName := guildID
	if threadConfig, ok := cfg.ThreadConfig[guildID]; ok && threadConfig.Name != "" {
		guildName = threadConfig.Name
	}
	leaderboardEmbed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🏆 %s - 新卡速递排行榜", guildName),
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
					stats.TodayPosts, stats.YesterdayPosts, stats.Last3DaysPosts, stats.Last7DaysPosts,
				),
				Inline: false,
			},
		},
	}
	embeds = append(embeds, leaderboardEmbed)

	// 3. 构建最新卡片-embed
	latestPostsEmbed, err := latest_posts.CreateLatestPostsEmbed(guildID)
	if err != nil {
		log.Printf("Error creating latest posts embed for %s: %v", guildID, err)
		// 不返回错误，只记录日志
	}
	if latestPostsEmbed != nil {
		embeds = append(embeds, latestPostsEmbed)
	}

	guildsDB, err := database.InitDB("data/guilds.db")
	if err != nil {
		log.Printf("Could not open guilds.db: %v", err)
	} else {
		defer guildsDB.Close()
		ad, err := database.GetRandomEnabledLeaderboardAd(guildsDB, guildID)
		if err == nil && ad != nil {
			var adEmbed discordgo.MessageEmbed
			if json.Unmarshal([]byte(ad.Content), &adEmbed) != nil {
				adEmbed = discordgo.MessageEmbed{
					Title:       "📜 服务器内广告",
					Description: ad.Content,
					Color:       0x7289DA,
				}
			}
			if ad.ImageURL != "" {
				adEmbed.Image = &discordgo.MessageEmbedImage{URL: ad.ImageURL}
			}
			// 将广告embed放在第二个位置
			embeds = append(embeds[:1], append([]*discordgo.MessageEmbed{&adEmbed}, embeds[1:]...)...)
		}
	}

	return embeds, nil
}

func buildGlobalLeaderboardEmbeds(cfg *model.Config) ([]*discordgo.MessageEmbed, error) {
	var embeds []*discordgo.MessageEmbed
	now := time.Now()

	// 1. 获取全局统计数据
	dbMapping, err := utils.LoadDatabaseMapping()
	if err != nil {
		return nil, fmt.Errorf("无法加载数据库映射文件: %w", err)
	}

	stats, err := database.GetGlobalStats(dbMapping, cfg.ThreadConfig)
	if err != nil {
		return nil, fmt.Errorf("获取全局数据时出错: %w", err)
	}

	// 2. 构建主-embed
	leaderboardEmbed := &discordgo.MessageEmbed{
		Title:       "🏆 全局新卡速递排行榜",
		Description: fmt.Sprintf("数据来源: %d 个服务器\n最后更新于 <t:%d:R>", len(stats.SourceGuilds), now.Unix()),
		Color:       0xFFD700, // Gold
		Timestamp:   now.Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "📊 全局数据统计",
				Value: fmt.Sprintf(
					"**今日新增**: %d\n"+
						"**昨日新增**: %d\n"+
						"**近3日新增**: %d\n"+
						"**近7日新增**: %d",
					stats.TodayPosts, stats.YesterdayPosts, stats.Last3DaysPosts, stats.Last7DaysPosts,
				),
				Inline: false,
			},
		},
	}
	if len(stats.Errors) > 0 {
		errorField := &discordgo.MessageEmbedField{
			Name:   "⚠️ 注意",
			Value:  fmt.Sprintf("有 %d 个服务器数据获取失败。", len(stats.Errors)),
			Inline: false,
		}
		leaderboardEmbed.Fields = append(leaderboardEmbed.Fields, errorField)
	}
	embeds = append(embeds, leaderboardEmbed)

	// 3. 构建最新卡片-embed
	latestPosts, err := database.GetGlobalLatestPosts(dbMapping, cfg.ThreadConfig, 10)
	if err != nil {
		log.Printf("Error creating global latest posts embed: %v", err)
	}

	if len(latestPosts) > 0 {
		latestPostsEmbed := &discordgo.MessageEmbed{
			Title: "📑 全局最新卡片",
			Color: 0x0099ff,
		}
		for _, post := range latestPosts {
			// 为了简单起见，全局的最新卡片列表不加载tag mapping
			value := fmt.Sprintf("> %s · <t:%d:R>\n> <#%s>", post.Author, post.Timestamp, post.ID)
			latestPostsEmbed.Fields = append(latestPostsEmbed.Fields, &discordgo.MessageEmbedField{
				Name:   post.Title,
				Value:  value,
				Inline: false,
			})
		}
		embeds = append(embeds, latestPostsEmbed)
	}

	return embeds, nil
}
