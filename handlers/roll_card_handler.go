package handlers

import (
	"discord-bot/bot"
	"discord-bot/utils"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func HandleRollCardInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
	// 1. Get options
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

	// 2. Get guild-specific config
	guildID := i.GuildID
	rollCardGuildConfig, ok := b.Config.RollCardConfigs[guildID]
	if !ok {
		log.Printf("Could not find roll card config for guild: %s", guildID)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This server is not configured for rollcard.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// 3. Find table name from pool name
	var tableName string
	for key, value := range rollCardGuildConfig.DataBaseTableNameMapping {
		if value == poolName {
			tableName = key
			break
		}
	}

	if tableName == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Invalid pool name: %s", poolName),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// 4. Open the specific database for the guild
	db, err := utils.InitDB(rollCardGuildConfig.Database)
	if err != nil {
		log.Printf("Error opening database %s for guild %s: %v", rollCardGuildConfig.Database, guildID, err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error accessing card database.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	defer db.Close()

	// 5. Get random posts
	posts, err := utils.GetRandomPosts(db, tableName, count)
	if err != nil {
		log.Printf("Error getting random posts from table %s: %v", tableName, err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error retrieving cards from the pool.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if len(posts) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("The pool '%s' is empty.", poolName),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// 6. Load tag mapping
	tagMapping, err := loadTagMapping(rollCardGuildConfig.TagMappingFile)
	if err != nil {
		log.Printf("Error loading tag mapping file %s: %v", rollCardGuildConfig.TagMappingFile, err)
		// Continue without tag mapping
	}

	// 7. Format and send response
	embeds := []*discordgo.MessageEmbed{}
	for _, post := range posts {
		tags := getTagNames(post.Tags, tagMapping)
		embed := &discordgo.MessageEmbed{
			Title:       post.Title,
			Description: post.Content,
			Author: &discordgo.MessageEmbedAuthor{
				Name: post.Author,
			},
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Tags", Value: tags, Inline: true},
				{Name: "Message Count", Value: strconv.Itoa(post.MessageCount), Inline: true},
				{Name: "Link", Value: fmt.Sprintf("<#%s>", post.ID), Inline: true},
			},
			Timestamp: time.Unix(post.Timestamp, 0).Format(time.RFC3339),
		}
		if post.CoverImageURL != "" {
			embed.Image = &discordgo.MessageEmbedImage{URL: post.CoverImageURL}
		}
		embeds = append(embeds, embed)
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Here are your cards from the '%s' pool!", poolName),
			Embeds:  embeds,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func loadTagMapping(file string) (map[string]map[string]string, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var mapping map[string]map[string]string
	err = json.Unmarshal(data, &mapping)
	if err != nil {
		return nil, err
	}
	return mapping, nil
}

func getTagNames(tagIDs string, tagMapping map[string]map[string]string) string {
	if tagMapping == nil {
		return tagIDs
	}
	ids := strings.Split(tagIDs, ",")
	var names []string
	for _, id := range ids {
		found := false
		for _, category := range tagMapping {
			if name, ok := category[id]; ok {
				names = append(names, name)
				found = true
				break
			}
		}
		if !found {
			names = append(names, id) // Keep original ID if not found
		}
	}
	return strings.Join(names, ", ")
}
