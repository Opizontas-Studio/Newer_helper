package personalnav

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"newer_helper/bot"
	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"

	"github.com/bwmarrin/discordgo"
)

const (
	componentSelectSlot           = "personal_nav:slot"
	componentSelectAreaPrefix     = "personal_nav:area:"
	componentSubmitAreaPrefix     = "personal_nav:submit_area:"
	componentRefreshSelect        = "personal_nav:refresh"
	componentDeleteSelect         = "personal_nav:delete"
	maxNavigationSlots            = 3
	maxLatestPostsToDisplay       = 10
	maxMyWorksToDisplay           = 999 // 实际移除限制，由自动分页处理
	embedColorPrimary         int = 0x5865F2
	embedColorHighlight       int = 0xFEE75C
	embedColorSecondary       int = 0x57F287
)

type channelChoice struct {
	TableName   string
	ChannelID   string
	ChannelName string
	PostCount   int
}

// HandlePersonalNavCommand is the entry point for /personal-nav.
func HandlePersonalNavCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	if !validateUsageContext(s, i) {
		utils.SendEphemeralResponse(s, i, "该命令只能在您管理的频道或您创建的子区内使用。")
		return
	}

	guildID := i.GuildID
	requesterID := i.Member.User.ID
	targetUserID := requesterID

	optionMap := map[string]*discordgo.ApplicationCommandInteractionDataOption{}
	for _, opt := range i.ApplicationCommandData().Options {
		optionMap[opt.Name] = opt
	}

	if userIDOpt, ok := optionMap["userid"]; ok {
		specifiedUserID := strings.TrimSpace(userIDOpt.StringValue())
		if specifiedUserID != "" {
			isDev := utils.CheckPermission(nil, requesterID, nil, nil, b.GetConfig().DeveloperUserIDs, nil) == utils.DeveloperPermission
			if isDev {
				targetUserID = specifiedUserID
				log.Printf("personal-nav: developer %s is acting on behalf of user %s", requesterID, targetUserID)
			} else {
				utils.SendEphemeralResponse(s, i, "您没有权限使用 `userid` 参数。")
				return
			}
		}
	}

	actionOpt, ok := optionMap["action"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "缺少操作类型。")
		return
	}

	input := ""
	if opt, ok := optionMap["input"]; ok {
		input = strings.TrimSpace(opt.StringValue())
	}

	action := actionOpt.StringValue()
	log.Printf("personal-nav: received action=%s guild=%s user=%s channel=%s", action, guildID, targetUserID, i.ChannelID)

	switch action {
	case "create":
		handleCreateAction(s, i, b, targetUserID)
	case "refresh":
		handleRefreshAction(s, i, b, input, targetUserID)
	case "delete":
		handleDeleteAction(s, i, b, input, targetUserID)
	default:
		utils.SendEphemeralResponse(s, i, "未知的操作类型。")
	}
}

func handleCreateAction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, userID string) {
	guildID := i.GuildID
	log.Printf("personal-nav: handleCreateAction guild=%s user=%s", guildID, userID)

	log.Printf("personal-nav: loading existing navigation entries...")
	navigations, err := database.GetPersonalNavigations(userID, guildID)
	if err != nil {
		log.Printf("Failed to load personal navigations for %s/%s: %v", guildID, userID, err)
		utils.SendEphemeralResponse(s, i, "读取导航数据时出错。")
		return
	}
	log.Printf("personal-nav: existing navigations count=%d", len(navigations))

	slot := findFirstAvailableSlot(navigations)
	if slot == 0 {
		data := buildSlotSelectionResponse(navigations, userID)
		if data == nil {
			utils.SendEphemeralResponse(s, i, "无法加载现有导航。")
			return
		}

		log.Printf("personal-nav: all slots occupied, prompting override selection.")
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: data,
		})
		if err != nil {
			log.Printf("Failed to send slot selection response: %v", err)
		}
		return
	}

	data, err := buildAreaSelectionResponse(b, guildID, userID, slot)
	if err != nil {
		utils.SendEphemeralResponse(s, i, err.Error())
		return
	}
	if data == nil {
		utils.SendEphemeralResponse(s, i, "暂未在任何子区找到属于您的作品。")
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
	if err != nil {
		log.Printf("Failed to send area selection menu: %v", err)
	}
}

func handleRefreshAction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, messageID, userID string) {
	guildID := i.GuildID

	// Direct refresh via message id.
	if messageID != "" {
		nav, err := database.GetPersonalNavigationByMessageID(guildID, messageID)
		if err != nil {
			log.Printf("Failed to locate navigation by message id %s: %v", messageID, err)
			utils.SendEphemeralResponse(s, i, "查询导航记录时出错。")
			return
		}
		if nav == nil || nav.UserID != userID {
			utils.SendEphemeralResponse(s, i, "找不到与该消息关联的导航，或您没有权限。")
			return
		}
		if err := refreshNavigation(s, b, i, *nav); err != nil {
			utils.SendEphemeralResponse(s, i, err.Error())
			return
		}
		utils.SendEphemeralResponse(s, i, "导航已刷新。")
		return
	}

	navigations, err := database.GetPersonalNavigations(userID, guildID)
	if err != nil {
		log.Printf("Failed to query navigations for refresh: %v", err)
		utils.SendEphemeralResponse(s, i, "读取导航数据时出错。")
		return
	}
	if len(navigations) == 0 {
		utils.SendEphemeralResponse(s, i, "您还没有创建任何导航。")
		return
	}

	log.Printf("personal-nav: prompting user %s to select navigation for refresh (count=%d)", userID, len(navigations))

	data := buildNavigationSelectionResponse(navigations, "选择需要刷新的导航", componentRefreshSelect, userID)
	if data == nil {
		utils.SendEphemeralResponse(s, i, "无法加载导航列表。")
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
	if err != nil {
		log.Printf("Failed to send refresh selection menu: %v", err)
	}
}

func handleDeleteAction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, messageID, userID string) {
	guildID := i.GuildID

	if messageID != "" {
		nav, err := database.GetPersonalNavigationByMessageID(guildID, messageID)
		if err != nil {
			log.Printf("Failed to locate navigation by message id %s: %v", messageID, err)
			utils.SendEphemeralResponse(s, i, "查询导航记录时出错。")
			return
		}
		if nav == nil || nav.UserID != userID {
			utils.SendEphemeralResponse(s, i, "找不到与该消息关联的导航，或您没有权限。")
			return
		}
		if err := deleteNavigation(s, *nav); err != nil {
			utils.SendEphemeralResponse(s, i, err.Error())
			return
		}
		utils.SendEphemeralResponse(s, i, "导航已删除。")
		return
	}

	navigations, err := database.GetPersonalNavigations(userID, guildID)
	if err != nil {
		log.Printf("Failed to query navigations for deletion: %v", err)
		utils.SendEphemeralResponse(s, i, "读取导航数据时出错。")
		return
	}
	if len(navigations) == 0 {
		utils.SendEphemeralResponse(s, i, "当前没有可以删除的导航。")
		return
	}

	log.Printf("personal-nav: prompting user %s to select navigation for delete (count=%d)", userID, len(navigations))

	data := buildNavigationSelectionResponse(navigations, "选择需要删除的导航", componentDeleteSelect, userID)
	if data == nil {
		utils.SendEphemeralResponse(s, i, "无法加载导航列表。")
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
	if err != nil {
		log.Printf("Failed to send delete selection menu: %v", err)
	}
}

func findFirstAvailableSlot(existing []model.PersonalNavigation) int {
	occupied := make(map[int]bool, len(existing))
	for _, nav := range existing {
		occupied[nav.NavID] = true
	}
	for slot := 1; slot <= maxNavigationSlots; slot++ {
		if !occupied[slot] {
			return slot
		}
	}
	return 0
}

func buildSlotSelectionResponse(navigations []model.PersonalNavigation, userID string) *discordgo.InteractionResponseData {
	if len(navigations) == 0 {
		return nil
	}

	sort.Slice(navigations, func(i, j int) bool {
		return navigations[i].NavID < navigations[j].NavID
	})

	options := make([]discordgo.SelectMenuOption, 0, len(navigations))
	for _, nav := range navigations {
		label := fmt.Sprintf("导航 %d", nav.NavID)
		if nav.ChannelName != "" {
			label = fmt.Sprintf("导航 %d · %s", nav.NavID, nav.ChannelName)
		}
		desc := fmt.Sprintf("位于 <#%s>", nav.MessageChannelID)
		if nav.MessageChannelID == "" {
			desc = "频道未知"
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:       truncateString(label, 100),
			Value:       strconv.Itoa(nav.NavID),
			Description: truncateString(desc, 100),
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "导航上限已满",
		Description: "请选择一个导航槽位进行覆盖。",
		Color:       embedColorHighlight,
	}

	return &discordgo.InteractionResponseData{
		Flags:  discordgo.MessageFlagsEphemeral,
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    fmt.Sprintf("%s:%s", componentSelectSlot, userID),
						Placeholder: "选择需要覆盖的导航",
						MinValues:   &[]int{1}[0],
						MaxValues:   1,
						Options:     options,
					},
				},
			},
		},
	}
}

func buildNavigationSelectionResponse(navigations []model.PersonalNavigation, title, customID, userID string) *discordgo.InteractionResponseData {
	if len(navigations) == 0 {
		return nil
	}

	sort.Slice(navigations, func(i, j int) bool {
		return navigations[i].NavID < navigations[j].NavID
	})

	options := make([]discordgo.SelectMenuOption, 0, len(navigations))
	for _, nav := range navigations {
		label := fmt.Sprintf("导航 %d", nav.NavID)
		if nav.ChannelName != "" {
			label = fmt.Sprintf("导航 %d · %s", nav.NavID, nav.ChannelName)
		}
		desc := fmt.Sprintf("<#%s>", nav.MessageChannelID)
		if nav.MessageChannelID == "" {
			desc = "创建位置未知"
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:       truncateString(label, 100),
			Value:       strconv.Itoa(nav.NavID),
			Description: truncateString(desc, 100),
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: "请选择一个导航槽位。",
		Color:       embedColorPrimary,
	}

	return &discordgo.InteractionResponseData{
		Flags:  discordgo.MessageFlagsEphemeral,
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    fmt.Sprintf("%s:%s", customID, userID),
						Placeholder: "选择一个导航",
						MinValues:   &[]int{1}[0],
						MaxValues:   1,
						Options:     options,
					},
				},
			},
		},
	}
}

func buildAreaSelectionResponse(b *bot.Bot, guildID, userID string, navID int) (*discordgo.InteractionResponseData, error) {
	choices, err := buildChannelChoices(b, guildID, userID)
	if err != nil {
		return nil, err
	}
	if len(choices) == 0 {
		log.Printf("personal-nav: user %s has no channel choices in guild %s", userID, guildID)
		return nil, nil
	}

	log.Printf("personal-nav: user %s has %d channel choices in guild %s", userID, len(choices), guildID)

	options := make([]discordgo.SelectMenuOption, 0, len(choices))
	for _, choice := range choices {
		label := fmt.Sprintf("%s · %d", choice.ChannelName, choice.PostCount)
		options = append(options, discordgo.SelectMenuOption{
			Label:       truncateString(label, 100),
			Value:       choice.TableName,
			Description: truncateString(fmt.Sprintf("频道: %s", choice.ChannelName), 100),
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("选择渲染区域 · 导航槽 %d", navID),
		Description: "仅显示您在其中发布过作品的分区，请选择一个或多个需要生成导航的分区。",
		Color:       embedColorPrimary,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "提示：最多同时保存 3 个导航，再次选择会覆盖旧导航。",
		},
	}

	maxValues := len(options)
	if maxValues > 25 {
		maxValues = 25
	}

	return &discordgo.InteractionResponseData{
		Flags:  discordgo.MessageFlagsEphemeral,
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    fmt.Sprintf("%s%d:%s", componentSelectAreaPrefix, navID, userID),
						Placeholder: "选择一个或多个分区",
						MinValues:   &[]int{1}[0],
						MaxValues:   maxValues,
						Options:     options,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "提交选择",
						Style:    discordgo.PrimaryButton,
						CustomID: fmt.Sprintf("%s%d:%s", componentSubmitAreaPrefix, navID, userID),
					},
				},
			},
		},
	}, nil
}

func buildChannelChoices(b *bot.Bot, guildID, userID string) ([]channelChoice, error) {
	cfg := b.GetConfig()
	guildTask, ok := cfg.TaskConfig[guildID]
	if !ok {
		return nil, fmt.Errorf("未配置该服务器的分区信息。")
	}

	dbPath := fmt.Sprintf("data/%s.db", guildID)
	db, err := database.InitDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败。")
	}
	defer db.Close()

	log.Printf("personal-nav: scanning channel choices for user %s in guild %s", userID, guildID)

	var choices []channelChoice
	for name, channelTask := range guildTask.Data {
		if len(channelTask.ChannelID) < 4 {
			continue
		}
		tableName := fmt.Sprintf("%s_%s", name, channelTask.ChannelID[len(channelTask.ChannelID)-4:])
		count, err := database.CountPostsByAuthorInTable(db, tableName, userID)
		if err != nil {
			log.Printf("Failed to count posts for user %s table %s: %v", userID, tableName, err)
			continue
		}
		if count == 0 {
			log.Printf("personal-nav: skip table %s for user %s (no posts)", tableName, userID)
			continue
		}
		choices = append(choices, channelChoice{
			TableName:   tableName,
			ChannelID:   channelTask.ChannelID,
			ChannelName: name,
			PostCount:   count,
		})
	}

	log.Printf("personal-nav: total channel choices gathered=%d", len(choices))

	sort.Slice(choices, func(i, j int) bool {
		return choices[i].ChannelName < choices[j].ChannelName
	})

	return choices, nil
}

func validateUsageContext(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	userID := i.Member.User.ID
	channel, err := s.State.Channel(i.ChannelID)
	if err != nil {
		channel, err = s.Channel(i.ChannelID)
		if err != nil {
			log.Printf("Failed to fetch channel %s for personal nav command: %v", i.ChannelID, err)
			return false
		}
	}

	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err == nil && perms&discordgo.PermissionManageChannels != 0 {
		return true
	}

	switch channel.Type {
	case discordgo.ChannelTypeGuildPublicThread,
		discordgo.ChannelTypeGuildPrivateThread,
		discordgo.ChannelTypeGuildNewsThread:
		if channel.OwnerID == userID {
			return true
		}
		if channel.ParentID != "" {
			perms, err = s.UserChannelPermissions(userID, channel.ParentID)
			if err == nil && perms&discordgo.PermissionManageChannels != 0 {
				return true
			}
		}
	default:
		// Already checked direct manage permission.
	}
	return false
}

func truncateString(input string, maxLen int) string {
	if len([]rune(input)) <= maxLen {
		return input
	}
	runes := []rune(input)
	return string(runes[:maxLen])
}

func postURL(guildID, channelID, threadID string) string {
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, channelID, threadID)
}

// buildSafeDescription 构建一个带长度限制的 description，确保不超过 Discord 的限制
func buildSafeDescription(prefix string, lines []string, fallback string, maxLength int) string {
	if len(lines) == 0 {
		return prefix + fallback
	}

	// 从完整内容开始，逐步减少行数直到满足长度限制
	for numLines := len(lines); numLines > 0; numLines-- {
		currentLines := lines[:numLines]
		description := prefix + strings.Join(currentLines, "\n")

		if len(description) <= maxLength {
			// 如果被截断了，添加提示
			if numLines < len(lines) {
				truncated := fmt.Sprintf("\n\n_（显示前 %d 个，共 %d 个）_", numLines, len(lines))
				if len(description)+len(truncated) <= maxLength {
					description += truncated
				}
			}
			log.Printf("personal-nav: buildSafeDescription used %d/%d lines, length=%d/%d", numLines, len(lines), len(description), maxLength)
			return description
		}
	}

	// 如果连一行都放不下，返回后备文本
	log.Printf("personal-nav: WARNING - even single line exceeds limit, using fallback")
	return prefix + fallback
}

func buildEmbeds(guildID string, channelInfos []channelChoice, latestPosts []model.Post, topPosts []model.Post) (myWorksEmbeds []*discordgo.MessageEmbed, topWorks, latest *discordgo.MessageEmbed) {
	// 按分区分组作品
	postsByPartition := groupPostsByPartition(latestPosts, channelInfos)

	// 为每个分区构建 embed（可能多个）
	for _, ci := range channelInfos {
		posts := postsByPartition[ci.TableName]

		// 构建该分区的 embed（自动分页处理所有作品）
		partitionEmbeds := buildPartitionEmbeds(ci.ChannelName, ci.ChannelID, guildID, posts, len(posts))
		myWorksEmbeds = append(myWorksEmbeds, partitionEmbeds...)
	}

	displayTop := topPosts
	if len(displayTop) > maxLatestPostsToDisplay {
		displayTop = displayTop[:maxLatestPostsToDisplay]
	}
	topLines := make([]string, 0, len(displayTop))
	for _, post := range displayTop {
		var channelID string
		for _, ci := range channelInfos {
			if strings.HasPrefix(post.ID, ci.TableName) {
				channelID = ci.ChannelID
				break
			}
		}
		topLines = append(topLines, formatPostLineWithStats(guildID, channelID, post))
	}

	const maxEmbedDescriptionLength = 3800 // Discord 限制 4096，使用 3800 作为安全阈值
	topDescription := buildSafeDescription(
		"根据消息数量 (MessageCount) 排序。\n\n",
		topLines,
		"暂无数据。",
		maxEmbedDescriptionLength,
	)

	topWorks = &discordgo.MessageEmbed{
		Title:       "🔥 最高消息作品",
		Description: topDescription,
		Color:       embedColorHighlight,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	displayRecent := latestPosts
	if len(displayRecent) > maxLatestPostsToDisplay {
		displayRecent = displayRecent[:maxLatestPostsToDisplay]
	}
	latestLines := make([]string, 0, len(displayRecent))
	for _, post := range displayRecent {
		var channelID string
		for _, ci := range channelInfos {
			if strings.HasPrefix(post.ID, ci.TableName) {
				channelID = ci.ChannelID
				break
			}
		}
		latestLines = append(latestLines, formatPostLineWithDate(guildID, channelID, post))
	}

	latestDescription := buildSafeDescription(
		"按时间倒序展示最新作品。\n\n",
		latestLines,
		"暂无数据。",
		maxEmbedDescriptionLength,
	)

	latest = &discordgo.MessageEmbed{
		Title:       "🆕 最新作品",
		Description: latestDescription,
		Color:       embedColorSecondary,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return myWorksEmbeds, topWorks, latest
}

func formatPostLine(guildID, channelID string, post model.Post) string {
	title := post.Title
	if strings.TrimSpace(title) == "" {
		title = "未命名作品"
	}
	return fmt.Sprintf("[%s](%s) · 💬 %d · <t:%d:R>", truncateString(title, 70), postURL(guildID, channelID, post.ID), post.MessageCount, post.Timestamp)
}

func formatPostLineWithStats(guildID, channelID string, post model.Post) string {
	title := post.Title
	if strings.TrimSpace(title) == "" {
		title = "未命名作品"
	}
	return fmt.Sprintf("[%s](%s)\n> 💬 %d · <t:%d:R>", truncateString(title, 70), postURL(guildID, channelID, post.ID), post.MessageCount, post.Timestamp)
}

func formatPostLineWithDate(guildID, channelID string, post model.Post) string {
	title := post.Title
	if strings.TrimSpace(title) == "" {
		title = "未命名作品"
	}
	return fmt.Sprintf("[%s](%s)\n> <t:%d:F>", truncateString(title, 70), postURL(guildID, channelID, post.ID), post.Timestamp)
}

// groupPostsByPartition 按分区分组作品
func groupPostsByPartition(posts []model.Post, channelInfos []channelChoice) map[string][]model.Post {
	result := make(map[string][]model.Post)

	for _, post := range posts {
		for _, ci := range channelInfos {
			if post.ChannelID == ci.ChannelID {
				result[ci.TableName] = append(result[ci.TableName], post)
				break
			}
		}
	}

	return result
}

// buildPartitionEmbeds 为单个分区构建一个或多个 embed（超过限制时拆分）
func buildPartitionEmbeds(partitionName, channelID, guildID string, posts []model.Post, totalCount int) []*discordgo.MessageEmbed {
	const maxDescriptionLength = 4000 // 优化阈值，Discord description 限制为 4096，保留96字符安全边距

	if len(posts) == 0 {
		// 没有作品，返回一个空的 embed
		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("📁 我的作品 - %s (%d个投稿)", partitionName, totalCount),
			Description: fmt.Sprintf("频道：<#%s>\n\n暂无作品记录。", channelID),
			Color:       embedColorPrimary,
			Timestamp:   time.Now().Format(time.RFC3339),
		}
		return []*discordgo.MessageEmbed{embed}
	}

	// 构建作品行
	lines := make([]string, 0, len(posts))
	for _, post := range posts {
		lines = append(lines, formatPostLine(guildID, channelID, post))
	}

	// 计算是否需要拆分
	var embeds []*discordgo.MessageEmbed
	var currentLines []string
	channelPrefix := fmt.Sprintf("频道：<#%s>\n\n", channelID)

	for _, line := range lines {
		// 模拟拼接后的内容来检查长度
		var testValue string
		if len(currentLines) == 0 {
			testValue = channelPrefix + line
		} else {
			testValue = channelPrefix + strings.Join(currentLines, "\n") + "\n" + line
		}

		// 如果加入这一行会超过限制，先保存当前的 embed
		if len(testValue) > maxDescriptionLength && len(currentLines) > 0 {
			embeds = append(embeds, createPartitionEmbed(partitionName, channelID, totalCount, currentLines, len(embeds)+1, 0))
			currentLines = []string{line}
		} else {
			currentLines = append(currentLines, line)
		}
	}

	// 添加最后一个 embed
	if len(currentLines) > 0 {
		totalPages := len(embeds) + 1
		embeds = append(embeds, createPartitionEmbed(partitionName, channelID, totalCount, currentLines, len(embeds)+1, totalPages))
	}

	// 如果有多页，需要更新之前的 embed 标题以显示页码
	if len(embeds) > 1 {
		for i := 0; i < len(embeds)-1; i++ {
			embeds[i].Title = fmt.Sprintf("📁 我的作品 - %s (第%d/%d页)", partitionName, i+1, len(embeds))
		}
		embeds[len(embeds)-1].Title = fmt.Sprintf("📁 我的作品 - %s (第%d/%d页)", partitionName, len(embeds), len(embeds))
	}

	return embeds
}

// createPartitionEmbed 创建一个分区 embed
func createPartitionEmbed(partitionName, channelID string, totalCount int, lines []string, pageNum, totalPages int) *discordgo.MessageEmbed {
	title := fmt.Sprintf("📁 我的作品 - %s (%d个投稿)", partitionName, totalCount)
	if totalPages > 1 {
		title = fmt.Sprintf("📁 我的作品 - %s (第%d/%d页)", partitionName, pageNum, totalPages)
	}

	// 构建 description：频道信息 + 作品列表
	description := fmt.Sprintf("频道：<#%s>\n\n%s", channelID, strings.Join(lines, "\n"))

	// 安全检查：确保不超过 Discord 限制（description 最大 4096 字符）
	if len(description) > 4096 {
		log.Printf("personal-nav: WARNING - description exceeds 4096 chars (%d), truncating", len(description))
		// 截断到 4090 字节（留出省略号的空间），并确保不破坏 UTF-8 字符
		maxLen := 4090
		for maxLen > 0 && maxLen < len(description) {
			// 检查是否在 UTF-8 字符边界上
			if utf8.ValidString(description[:maxLen]) {
				break
			}
			maxLen--
		}
		description = description[:maxLen] + "\n..."
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       embedColorPrimary,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return embed
}
