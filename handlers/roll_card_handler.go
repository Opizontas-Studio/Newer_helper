package handlers

import (
	"discord-bot/bot"
	"discord-bot/utils"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"discord-bot/model"

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
	tagID := ""
	if opt, ok := optionMap["tag"]; ok {
		tagID = opt.StringValue()
	}

	var posts []model.Post
	var tagMapping map[string]map[string]string

	if poolName == "all-server-roll" {
		// "All-server" means all pools within the *current* guild.
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

		if tagID != "" {
			posts, err = utils.GetRandomPostsByTagFromAllTables(db, tagID, count)
		} else {
			posts, err = utils.GetRandomPostsFromAllTables(db, count)
		}
		if err != nil {
			log.Printf("Error getting random posts from all tables in %s: %v", rollCardGuildConfig.Database, err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error retrieving cards from the pool.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		tagMapping, _ = loadTagMapping(rollCardGuildConfig.TagMappingFile)
	} else {
		// Handle single pool roll (existing logic)
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

		if tagID != "" {
			posts, err = utils.GetRandomPostsByTag(db, tableName, tagID, count)
		} else {
			posts, err = utils.GetRandomPosts(db, tableName, count)
		}
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
		tagMapping, _ = loadTagMapping(rollCardGuildConfig.TagMappingFile)
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

	embeds := []*discordgo.MessageEmbed{}
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
				Text: fmt.Sprintf("由 %s 于 %s 抽取", i.Member.User.Username, time.Now().Format("2006-01-02 15:04:05")),
			},
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
	data, err := os.ReadFile(file)
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
