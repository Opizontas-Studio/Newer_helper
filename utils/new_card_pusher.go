package utils

import (
	"fmt"
	"log"
	"newer_helper/model"
	"newer_helper/utils/database"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const maxPushedMessages = 5

// PushNewCard 发送新卡通知并维护消息列表。
func PushNewCard(s *discordgo.Session, guildID string, newPost model.Post, rollCardConfig *model.RollCardGuildConfig) {
	config, err := LoadNewCardPushConfig(guildID)
	if err != nil {
		log.Printf("加载服务器 %s 的新卡推送配置时出错: %v", guildID, err)
		return
	}

	if len(config.PushChannelIDs) == 0 {
		return
	}

	// 连接数据库检查作者是否是新人。
	db, err := database.InitDB(rollCardConfig.Database)
	if err != nil {
		log.Printf("初始化新卡推送数据库时出错: %v", err)
		return
	}
	defer db.Close()

	postCount, err := database.CountPostsByAuthorID(db, newPost.AuthorID)
	if err != nil {
		log.Printf("检查作者 %s 的帖子数量时出错: %v", newPost.AuthorID, err)
		// 如果出错，则在不检查新作者的情况下继续
	}
	isNewAuthor := postCount == 1

	tagMapping, err := LoadTagMapping(rollCardConfig.TagMappingFile)
	if err != nil {
		log.Printf("无法为服务器 %s 加载标签映射文件 %s: %v", rollCardConfig.TagMappingFile, guildID, err)
		// 即使没有标签，我们仍然可以继续
	}

	embed := buildNewCardEmbed(newPost, tagMapping, isNewAuthor)

	for _, channelID := range config.PushChannelIDs {
		go func(chID string) {
			// 1. 发送新卡通知。
			_, err := s.ChannelMessageSendEmbed(chID, embed)
			if err != nil {
				log.Printf("向频道 %s 发送新卡推送消息时出错: %v", chID, err)
				return
			}

			// 2. 获取最近的消息以清理旧消息。
			messages, err := s.ChannelMessages(chID, 100, "", "", "")
			if err != nil {
				log.Printf("从推送频道 %s 获取消息时出错: %v", chID, err)
				return
			}

			// 3. 筛选机器人发送的、不在白名单中的消息。
			var botMessages []*discordgo.Message
			whitelistSet := make(map[string]struct{})
			if whitelisted, ok := config.WhitelistedMessages[chID]; ok {
				for _, id := range whitelisted {
					whitelistSet[id] = struct{}{}
				}
			}

			for _, msg := range messages {
				if msg.Author.ID == s.State.User.ID {
					if _, isWhitelisted := whitelistSet[msg.ID]; !isWhitelisted {
						botMessages = append(botMessages, msg)
					}
				}
			}

			// 4. 如果消息数量超过限制，则删除最旧的消息。
			if len(botMessages) > maxPushedMessages {
				sort.Slice(botMessages, func(i, j int) bool {
					return botMessages[i].Timestamp.Before(botMessages[j].Timestamp)
				})

				messagesToDelete := len(botMessages) - maxPushedMessages
				for i := 0; i < messagesToDelete; i++ {
					err := s.ChannelMessageDelete(chID, botMessages[i].ID)
					if err != nil {
						log.Printf("删除旧的推送消息 %s 时出错: %v", botMessages[i].ID, err)
					}
				}
			}
		}(channelID)
	}
}

func buildNewCardEmbed(post model.Post, tagMapping map[string]map[string]string, isNewAuthor bool) *discordgo.MessageEmbed {
	description := fmt.Sprintf("**%s**\n%s", post.Title, post.Content)
	// Discord 的描述限制为 4096 个字符。
	if len(description) > 4096 {
		description = description[:4093] + "..."
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "作者", Value: post.Author, Inline: true},
		{Name: "传送门", Value: fmt.Sprintf("<#%s>", post.ID), Inline: true},
	}

	if post.Tags != "" {
		tagNames := getTagNames(post.Tags, tagMapping)
		fields = append(fields, &discordgo.MessageEmbedField{Name: "标签", Value: tagNames, Inline: false})
	}

	title := "✨ 新卡速递 ✨"
	color := 0x0099ff // 默认蓝色
	if isNewAuthor {
		title = "✨ 新人首卡 ✨"
		color = 0xFFEB3B // 黄色
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields:      fields,
		Timestamp:   time.Unix(post.Timestamp, 0).Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "新卡通知",
		},
	}
}

// getTagNames 将逗号分隔的标签 ID 字符串转换为标签名称字符串。
func getTagNames(tagIDs string, tagMapping map[string]map[string]string) string {
	if tagMapping == nil || tagIDs == "" {
		return "无"
	}
	ids := strings.Split(tagIDs, ",")
	var names []string
	for _, id := range ids {
		trimmedID := strings.TrimSpace(id)
		found := false
		for _, category := range tagMapping {
			if name, ok := category[trimmedID]; ok {
				names = append(names, name)
				found = true
				break
			}
		}
		if !found {
			names = append(names, trimmedID) // 如果未找到，则保留原始 ID
		}
	}
	if len(names) == 0 {
		return "无"
	}
	return strings.Join(names, ", ")
}
