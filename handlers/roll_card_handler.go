package handlers

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
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

	poolName := optionMap["pool"].StringValue()
	count := 1 // Default count
	if opt, ok := optionMap["count"]; ok {
		count = int(opt.IntValue())
	}
	tagID := ""
	if opt, ok := optionMap["tag"]; ok {
		tagID = opt.StringValue()
	}

	rollCard(s, i, b, poolName, tagID, count)
}

// HandleRollAgain handles the "roll again" button interaction.
func HandleRollAgain(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, poolName, tagID string) {
	rollCard(s, i, b, poolName, tagID, 1)
}

// rollCard is the core logic for fetching posts and sending the response.
func rollCard(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, poolName, tagID string, count int) {
	guildID := i.GuildID
	rollCardGuildConfig, ok := b.Config.RollCardConfigs[guildID]
	if !ok {
		log.Printf("Could not find roll card config for guild: %s", guildID)
		sendEphemeralResponse(s, i, "This server is not configured for rollcard.")
		return
	}

	posts, err := getPosts(&rollCardGuildConfig, poolName, tagID, count)
	if err != nil {
		log.Printf("Error getting posts for guild %s: %v", guildID, err)
		sendEphemeralResponse(s, i, err.Error())
		return
	}

	if len(posts) == 0 {
		sendEphemeralResponse(s, i, fmt.Sprintf("The pool '%s' is empty.", poolName))
		return
	}

	tagMapping, err := utils.LoadTagMapping(rollCardGuildConfig.TagMappingFile)
	if err != nil {
		log.Printf("Could not load tag mapping file %s: %v", rollCardGuildConfig.TagMappingFile, err)
	}

	embeds := buildEmbeds(posts, tagMapping, i.Member.User.Username)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Here are your cards from the '%s' pool!", poolName),
			Embeds:  embeds,
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "再来一抽",
							Style:    discordgo.PrimaryButton,
							CustomID: "roll_again:" + poolName + ":" + tagID,
						},
					},
				},
			},
		},
	})
}

// getPosts retrieves random posts from the database based on the pool, tag, and count.
func getPosts(config *model.RollCardGuildConfig, poolName, tagID string, count int) ([]model.Post, error) {
	db, err := utils.InitDB(config.Database)
	if err != nil {
		return nil, fmt.Errorf("error accessing card database")
	}
	defer db.Close()

	var posts []model.Post
	if poolName == "all-server-roll" {
		if tagID != "" {
			posts, err = utils.GetRandomPostsByTagFromAllTables(db, tagID, count)
		} else {
			posts, err = utils.GetRandomPostsFromAllTables(db, count)
		}
		if err != nil {
			return nil, fmt.Errorf("error retrieving cards from all pools: %w", err)
		}
	} else {
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

		if tagID != "" {
			posts, err = utils.GetRandomPostsByTag(db, tableName, tagID, count)
		} else {
			posts, err = utils.GetRandomPosts(db, tableName, count)
		}
		if err != nil {
			return nil, fmt.Errorf("error retrieving cards from the pool '%s': %w", poolName, err)
		}
	}
	return posts, nil
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
			embed.Image = &discordgo.MessageEmbedImage{URL: post.CoverImageURL}
		}
		embeds = append(embeds, embed)
	}
	return embeds
}

// sendEphemeralResponse sends a simple, ephemeral message back to the user.
func sendEphemeralResponse(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// getTagNames converts a comma-separated string of tag IDs to a string of tag names.
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
			names = append(names, trimmedID) // Keep original ID if not found
		}
	}
	if len(names) == 0 {
		return "无"
	}
	return strings.Join(names, ", ")
}
