package rollcard

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// HandleRollCardInteraction handles the initial slash command for rolling cards.
func HandleRollCardInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var poolNames []string
	if opt, ok := optionMap["pool"]; ok {
		poolNames = []string{opt.StringValue()}
	} else {
		// If no pool is specified, try to get user's preferred pools
		userID := i.Member.User.ID
		guildID := i.GuildID
		preferredPools, err := database.GetUserPreferredPools(userID, guildID)
		if err != nil {
			log.Printf("Error getting user preferred pools for user %s in guild %s: %v", userID, guildID, err)
			utils.SendEphemeralResponse(s, i, "获取您的偏好卡池时出错 ")
			return
		}
		if len(preferredPools) == 0 {
			utils.SendEphemeralResponse(s, i, "您没有指定卡池，也没有设置默认卡池。请使用 `/set-default-roll` 设置或在抽卡时指定一个卡池。")
			return
		}
		poolNames = preferredPools
	}

	count := 1 // Default count
	if opt, ok := optionMap["count"]; ok {
		count = int(opt.IntValue())
	}
	tagID := ""
	if opt, ok := optionMap["tag"]; ok {
		tagID = opt.StringValue()
	}
	var excludeTags []string
	if opt, ok := optionMap["exclude_tag1"]; ok {
		if tagValue := opt.StringValue(); tagValue != "" {
			excludeTags = append(excludeTags, tagValue)
		}
	}
	if opt, ok := optionMap["exclude_tag2"]; ok {
		if tagValue := opt.StringValue(); tagValue != "" {
			excludeTags = append(excludeTags, tagValue)
		}
	}
	if opt, ok := optionMap["exclude_tag3"]; ok {
		if tagValue := opt.StringValue(); tagValue != "" {
			excludeTags = append(excludeTags, tagValue)
		}
	}

	if tagID != "" {
		for _, excludedTag := range excludeTags {
			if tagID == excludedTag {
				utils.SendEphemeralResponse(s, i, "错误：您不能在包含和排除中选择同一个标签 ")
				return
			}
		}
	}

	rollCard(s, i, b, poolNames, tagID, count, excludeTags)
}

// HandleRollAgain handles the "roll again" button interaction.
func HandleRollAgain(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, poolName, tagID string, excludeTags []string) {
	rollCard(s, i, b, []string{poolName}, tagID, 1, excludeTags)
}

// rollCard is the core logic for fetching posts and sending the response.
func rollCard(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, poolNames []string, tagID string, count int, excludeTags []string) {
	userID := i.Member.User.ID
	guildID := i.GuildID
	log.Printf("Executing rollCard for user %s in guild %s. Pools: %v, Tag: %s, ExcludeTags: %v, Count: %d", userID, guildID, poolNames, tagID, excludeTags, count)

	rollCardGuildConfig, ok := b.GetConfig().RollCardConfigs[guildID]
	if !ok {
		log.Printf("Could not find roll card config for guild: %s", guildID)
		utils.SendEphemeralResponse(s, i, "This server is not configured for rollcard.")
		return
	}

	posts, err := getPosts(&rollCardGuildConfig, poolNames, tagID, count, excludeTags)
	if err != nil {
		log.Printf("Error getting posts for guild %s: %v", guildID, err)
		utils.SendEphemeralResponse(s, i, err.Error())
		return
	}

	if len(posts) == 0 {
		utils.SendEphemeralResponse(s, i, "卡池为空，或未找到符合条件的卡片 ")
		return
	}

	tagMapping, err := utils.LoadTagMapping(rollCardGuildConfig.TagMappingFile)
	if err != nil {
		log.Printf("Could not load tag mapping file %s: %v", rollCardGuildConfig.TagMappingFile, err)
	}

	embeds := buildEmbeds(posts, tagMapping, i.Member.User.Username)

	var content string

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Embeds:  embeds,
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "再来一抽",
							Style:    discordgo.PrimaryButton,
							CustomID: fmt.Sprintf("roll_again:%s:%s:%s", strings.Join(poolNames, ";"), tagID, strings.Join(excludeTags, ",")),
						},
					},
				},
			},
		},
	})
}

// getPosts retrieves random posts from the database based on the pools, tag, and count.
func getPosts(config *model.RollCardGuildConfig, poolNames []string, tagID string, count int, excludeTags []string) ([]model.Post, error) {
	db, err := database.InitDB(config.Database)
	if err != nil {
		return nil, fmt.Errorf("error accessing card database")
	}
	defer db.Close()

	// Handle the special case for "all-server-roll"
	if len(poolNames) == 1 && poolNames[0] == "all-server-roll" {
		if tagID != "" || len(excludeTags) > 0 {
			return database.GetRandomPostsByTagFromAllTables(db, tagID, count, excludeTags)
		}
		return database.GetRandomPostsFromAllTables(db, count)
	}

	var tableNames []string
	for _, poolName := range poolNames {
		var tableName string
		for key, value := range config.DataBaseTableNameMapping {
			if value == poolName {
				tableName = key
				break
			}
		}
		if tableName == "" {
			return nil, fmt.Errorf("invalid pool name: %s", poolName)
		}
		tableNames = append(tableNames, tableName)
	}

	// log.Printf("Resolved pool names %v to table names %v for guild %s", poolNames, tableNames, config.GuildID)

	if len(tableNames) == 0 {
		return nil, fmt.Errorf("no valid pools selected")
	}

	if tagID != "" || len(excludeTags) > 0 {
		return database.GetRandomPostsByTagFromMultipleTables(db, tableNames, tagID, count, excludeTags)
	}
	return database.GetRandomPostsFromMultipleTables(db, tableNames, count)
}

// buildEmbeds creates a slice of MessageEmbeds from the given posts.
func buildEmbeds(posts []model.Post, tagMapping map[string]map[string]string, username string) []*discordgo.MessageEmbed {
	embeds := make([]*discordgo.MessageEmbed, 0, len(posts))
	for _, post := range posts {
		tags := getTagNames(post.Tags, tagMapping)
		embed := &discordgo.MessageEmbed{
			Title:       post.Title,
			Description: post.Content,
			Author: &discordgo.MessageEmbedAuthor{
				Name: post.Author,
			},
			Color: 0x0099ff,
			Fields: []*discordgo.MessageEmbedField{
				{Name: "🏷️ 标签", Value: tags, Inline: true},
				{Name: "💬 消息数量", Value: strconv.Itoa(post.MessageCount), Inline: true},
				{Name: "🔗 链接", Value: fmt.Sprintf("<#%s>", post.ID), Inline: true},
			},
			Timestamp: time.Unix(post.Timestamp, 0).Format(time.RFC3339),
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("由 %s 于 %s 抽取", username, time.Now().Format("2006-01-02 15:04:05")),
			},
		}
		if post.CoverImageURL != "" {
			embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: post.CoverImageURL}
		}
		embeds = append(embeds, embed)
	}
	return embeds
}

// getTagNames converts a comma-separated string of tag IDs to a string of tag names.
func getTagNames(tagIDs string, tagMapping map[string]map[string]string) string {
	if tagMapping == nil || tagIDs == "" {
		return "无或全局模式不兼容"
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
			names = append(names, trimmedID) // Keep original ID if not found
		}
	}
	if len(names) == 0 {
		return "无"
	}
	return strings.Join(names, ", ")
}

func HandleRollCardAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, config *model.Config) {
	data := i.ApplicationCommandData()
	var choices []*discordgo.ApplicationCommandOptionChoice

	var focusedOption discordgo.ApplicationCommandInteractionDataOption
	var poolName string
	for _, opt := range data.Options {
		if opt.Focused {
			focusedOption = *opt
		}
		if opt.Name == "pool" {
			poolName = opt.StringValue()
		}
	}

	guildID := i.GuildID
	rollCardGuildConfig, ok := config.RollCardConfigs[guildID]
	if !ok {
		return // or handle error
	}

	switch focusedOption.Name {
	case "pool":
		// Add the static option for all-server roll if it matches the user input or the input is empty
		if strings.Contains(strings.ToLower("全区抽卡"), strings.ToLower(focusedOption.StringValue())) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  "全区抽卡",
				Value: "all-server-roll",
			})
		}
		for _, name := range rollCardGuildConfig.DataBaseTableNameMapping {
			if strings.Contains(strings.ToLower(name), strings.ToLower(focusedOption.StringValue())) {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  name,
					Value: name,
				})
			}
		}
	case "tag", "exclude_tag1", "exclude_tag2", "exclude_tag3":
		if poolName == "" {
			// If no pool is selected, we cannot suggest tags.
			return
		}

		// Load the tag mapping file associated with the guild.
		tagMapping, err := utils.LoadTagMapping(rollCardGuildConfig.TagMappingFile)
		if err != nil {
			log.Printf("Error loading tag mapping for guild %s: %v", guildID, err)
			return
		}

		// Filter tags based on the selected pool.
		if poolName == "all-server-roll" {
			// For all-server-roll, show all tags from all categories.
			for _, tags := range tagMapping {
				for id, name := range tags {
					if strings.Contains(strings.ToLower(name), strings.ToLower(focusedOption.StringValue())) {
						choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
							Name:  name,
							Value: id,
						})
					}
				}
			}
		} else if tags, ok := tagMapping[poolName]; ok {
			// For a specific pool, only show tags from that category.
			for id, name := range tags {
				if strings.Contains(strings.ToLower(name), strings.ToLower(focusedOption.StringValue())) {
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  name,
						Value: id,
					})
				}
			}
		}
	}

	if len(choices) > 25 {
		choices = choices[:25]
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}

func HandleRollCardComponent(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, customID string) {
	if strings.HasPrefix(customID, "roll_again:") {
		parts := strings.SplitN(customID, ":", 4)
		if len(parts) >= 2 {
			// poolNames are joined by ";", so we split by that
			poolNames := strings.Split(parts[1], ";")
			tagID := ""
			if len(parts) >= 3 {
				tagID = parts[2]
			}
			var excludeTags []string
			if len(parts) >= 4 && parts[3] != "" {
				excludeTags = strings.Split(parts[3], ",")
			}
			// Call rollCard directly with the parsed data
			rollCard(s, i, b, poolNames, tagID, 1, excludeTags)
		}
	}
}
