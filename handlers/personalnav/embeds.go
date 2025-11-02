package personalnav

import (
	"fmt"
	"log"
	"newer_helper/model"
	"newer_helper/utils"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

// buildEmbeds 是构建所有个人导航 Embeds 的主函数。
// 它负责协调生成“我的作品”、“最高热度作品”和“最新作品”三个部分的 Embeds。
func buildEmbeds(guildID string, channelInfos []ChannelChoice, latestPosts []model.Post, topPosts []model.Post, navigationID int64, navSlot int) (myWorksEmbeds []*discordgo.MessageEmbed, topWorks, latest *discordgo.MessageEmbed) {
	// 1. 将用户的帖子按其所属的分区进行分组
	postsByPartition := groupPostsByPartition(latestPosts, channelInfos)

	// 2. 构建页脚文本，用于所有 Embeds
	var footerText string
	if navigationID > 0 {
		footerText = fmt.Sprintf("导航 ID: %d | 槽位: %d", navigationID, navSlot)
	} else {
		// 在导航记录首次创建、还没有数据库ID时显示
		footerText = fmt.Sprintf("导航槽位: %d (新建中...)", navSlot)
	}

	// 3. 为每个分区构建“我的作品”Embed。由于内容可能很长，此函数可能会返回多个分页的 Embeds。
	for _, ci := range channelInfos {
		posts := postsByPartition[ci.TableName]
		partitionEmbeds := buildPartitionEmbeds(ci.ChannelName, ci.ChannelID, guildID, posts, len(posts), footerText)
		myWorksEmbeds = append(myWorksEmbeds, partitionEmbeds...)
	}

	// 4. 构建“最高热度作品”Embed
	displayTop := topPosts
	if len(displayTop) > maxLatestPostsToDisplay {
		displayTop = displayTop[:maxLatestPostsToDisplay]
	}
	topLines := make([]string, 0, len(displayTop))
	for _, post := range displayTop {
		topLines = append(topLines, formatPostLineWithStats(guildID, post))
	}

	const maxEmbedDescriptionLength = 3800 // Discord Embed 描述限制为 4096，这里使用一个更安全的值
	topDescription := buildSafeDescription(
		"根据消息数量 (MessageCount) 排序。\n\n",
		topLines,
		"暂无数据。",
		maxEmbedDescriptionLength,
	)

	topWorks = &discordgo.MessageEmbed{
		Title:       "🔥 最高热度作品",
		Description: topDescription,
		Color:       embedColorHighlight,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: footerText,
		},
	}

	// 5. 构建“最新作品”Embed
	displayRecent := latestPosts
	if len(displayRecent) > maxLatestPostsToDisplay {
		displayRecent = displayRecent[:maxLatestPostsToDisplay]
	}
	latestLines := make([]string, 0, len(displayRecent))
	for _, post := range displayRecent {
		latestLines = append(latestLines, formatPostLineWithDate(guildID, post))
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
		Footer: &discordgo.MessageEmbedFooter{
			Text: footerText,
		},
	}

	return myWorksEmbeds, topWorks, latest
}

// groupPostsByPartition 是一个辅助函数，用于将帖子列表按其所属的分区表名 (TableName) 进行分组。
func groupPostsByPartition(posts []model.Post, channelInfos []ChannelChoice) map[string][]model.Post {
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

// buildPartitionEmbeds 为单个分区构建一个或多个“我的作品”Embed。
// 如果作品列表过长，它会自动将内容拆分成多个 Embed 以避免超出 Discord 的字数限制。
func buildPartitionEmbeds(partitionName, channelID, guildID string, posts []model.Post, totalCount int, footerText string) []*discordgo.MessageEmbed {
	const maxDescriptionLength = 4000 // Discord description 限制为 4096，保留安全边距

	// 如果该分区没有作品，则返回一个提示性的 Embed
	if len(posts) == 0 {
		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("📁 我的作品 - %s (%d篇)", partitionName, totalCount),
			Description: fmt.Sprintf("频道：<#%s>\n\n暂无作品记录。", channelID),
			Color:       embedColorPrimary,
			Timestamp:   time.Now().Format(time.RFC3339),
			Footer: &discordgo.MessageEmbedFooter{
				Text: footerText,
			},
		}
		return []*discordgo.MessageEmbed{embed}
	}

	// 将每个作品格式化为一行文本
	lines := make([]string, 0, len(posts))
	for _, post := range posts {
		lines = append(lines, formatPostLine(guildID, post))
	}

	var embeds []*discordgo.MessageEmbed
	var currentLines []string
	channelPrefix := fmt.Sprintf("频道：<#%s>\n\n", channelID)

	// 遍历所有行，将它们打包到 Embeds 中，同时确保不超过长度限制
	for _, line := range lines {
		var testValue string
		if len(currentLines) == 0 {
			testValue = channelPrefix + line
		} else {
			testValue = channelPrefix + strings.Join(currentLines, "\n") + "\n" + line
		}

		// 如果添加下一行会超长，则将当前行的集合打包成一个 Embed，然后开始新的页面
		if len(testValue) > maxDescriptionLength && len(currentLines) > 0 {
			embeds = append(embeds, createPartitionEmbed(partitionName, channelID, totalCount, currentLines, len(embeds)+1, 0, footerText))
			currentLines = []string{line}
		} else {
			currentLines = append(currentLines, line)
		}
	}

	// 将剩余的行打包成最后一个 Embed
	if len(currentLines) > 0 {
		totalPages := len(embeds) + 1
		embeds = append(embeds, createPartitionEmbed(partitionName, channelID, totalCount, currentLines, len(embeds)+1, totalPages, footerText))
	}

	// 如果产生了多个分页，则更新所有 Embed 的标题以包含页码信息 (e.g., "第1/3页")
	if len(embeds) > 1 {
		for i := range embeds {
			embeds[i].Title = fmt.Sprintf("📁 我的作品 - %s (第%d/%d页)", partitionName, i+1, len(embeds))
		}
	}

	return embeds
}

// createPartitionEmbed 是一个内部辅助函数，用于创建单个“我的作品”分区 Embed。
func createPartitionEmbed(partitionName, channelID string, totalCount int, lines []string, pageNum, totalPages int, footerText string) *discordgo.MessageEmbed {
	title := fmt.Sprintf("📁 我的作品 - %s (%d篇)", partitionName, totalCount)
	if totalPages > 1 {
		title = fmt.Sprintf("📁 我的作品 - %s (第%d/%d页)", partitionName, pageNum, totalPages)
	}

	description := fmt.Sprintf("频道：<#%s>\n\n%s", channelID, strings.Join(lines, "\n"))

	// 安全截断，以防万一描述文本仍然超过 Discord 的最终限制
	if len(description) > 4096 {
		log.Printf("personal-nav: WARNING - description exceeds 4096 chars (%d), truncating", len(description))
		maxLen := 4090
		// 确保不会在多字节字符中间截断
		for maxLen > 0 && maxLen < len(description) {
			if utf8.ValidString(description[:maxLen]) {
				break
			}
			maxLen--
		}
		description = description[:maxLen] + "\n..."
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       embedColorPrimary,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: footerText,
		},
	}
}

// buildSafeDescription 构建一个能安全放入 Embed 描述字段的字符串。
// 它会尝试包含尽可能多的行，如果内容过长，则会截断并添加一个“已省略”的提示。
func buildSafeDescription(prefix string, lines []string, fallback string, maxLength int) string {
	if len(lines) == 0 {
		return prefix + fallback
	}

	// 从包含所有行开始，逐步减少行数，直到总长度符合限制
	for numLines := len(lines); numLines > 0; numLines-- {
		currentLines := lines[:numLines]
		description := prefix + strings.Join(currentLines, "\n")

		if len(description) <= maxLength {
			// 如果发生了截断，添加一个提示
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

	log.Printf("personal-nav: WARNING - even single line exceeds limit, using fallback")
	return prefix + fallback
}

// formatPostLine 格式化单行作品信息，用于“我的作品”列表。
// 格式: [标题](链接) · 💬 消息数 · <t:时间戳:R> (相对时间)
func formatPostLine(guildID string, post model.Post) string {
	title := post.Title
	if strings.TrimSpace(title) == "" {
		title = "未命名作品"
	}
	return fmt.Sprintf("[%s](%s) · 💬 %d · <t:%d:R>", utils.TruncateString(title, 70), post.URL(guildID), post.MessageCount, post.Timestamp)
}

// formatPostLineWithStats 格式化单行作品信息，包含消息数，用于“最高热度作品”。
// 格式: [标题](链接)\n> 💬 消息数 · <t:时间戳:R> (相对时间)
func formatPostLineWithStats(guildID string, post model.Post) string {
	title := post.Title
	if strings.TrimSpace(title) == "" {
		title = "未命名作品"
	}
	return fmt.Sprintf("[%s](%s)\n> 💬 %d · <t:%d:R>", utils.TruncateString(title, 70), post.URL(guildID), post.MessageCount, post.Timestamp)
}

// formatPostLineWithDate 格式化单行作品信息，包含完整日期，用于“最新作品”。
// 格式: [标题](链接)\n> <t:时间戳:F> (完整日期时间)
func formatPostLineWithDate(guildID string, post model.Post) string {
	title := post.Title
	if strings.TrimSpace(title) == "" {
		title = "未命名作品"
	}
	return fmt.Sprintf("[%s](%s)\n> <t:%d:F>", utils.TruncateString(title, 70), post.URL(guildID), post.Timestamp)
}
