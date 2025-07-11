package latest_posts

import (
	"discord-bot/config"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	tagMappingDir = "data/tag_mapping"
)

// LoadTagMapping loads the tag mapping for a specific guild.
func LoadTagMapping(guildID string) (map[string]map[string]string, error) {
	filePath := filepath.Join(tagMappingDir, fmt.Sprintf("%s_config.json", guildID))
	file, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No tag mapping file, not an error.
		}
		return nil, fmt.Errorf("failed to read tag mapping file: %w", err)
	}

	var mapping map[string]map[string]string
	if err := json.Unmarshal(file, &mapping); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tag mapping: %w", err)
	}
	return mapping, nil
}

// CreateLatestPostsEmbed creates a new embed for the latest posts
func CreateLatestPostsEmbed(guildID string) (*discordgo.MessageEmbed, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	threadGuildConfig, ok := cfg.ThreadConfig[guildID]
	if !ok || threadGuildConfig.Database == "" {
		log.Printf("No database path configured for guild %s", guildID)
		return nil, nil
	}
	dbPath := threadGuildConfig.Database

	dbMapping, err := utils.LoadDatabaseMapping()
	if err != nil {
		log.Printf("Error loading database mapping: %v", err)
		return nil, nil
	}

	guildMapping, ok := dbMapping[guildID]
	if !ok {
		log.Printf("No database mapping found for guild %s", guildID)
		return nil, nil
	}

	var tableNames []string
	if len(guildMapping.DataBaseTableNameMapping) > 0 {
		for tableName := range guildMapping.DataBaseTableNameMapping {
			tableNames = append(tableNames, tableName)
		}
	} else if threadGuildConfig.TableName != "" {
		tableNames = append(tableNames, threadGuildConfig.TableName)
	}

	if len(tableNames) == 0 {
		log.Printf("No tables configured for leaderboard in guild %s", guildID)
		return nil, nil
	}

	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Printf("Error initializing database for guild %s at %s: %v", guildID, dbPath, err)
		return nil, nil
	}
	defer db.Close()

	posts, err := database.GetLatestPosts(db, tableNames, 12)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest posts from db: %w", err)
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Timestamp > posts[j].Timestamp
	})

	if len(posts) > 12 {
		posts = posts[:12]
	}

	if len(posts) == 0 {
		return nil, nil // No posts, no embed
	}

	tagMapping, err := LoadTagMapping(guildID)
	if err != nil {
		return nil, err
	}

	latestCardsEmbed := &discordgo.MessageEmbed{
		Title: "📑 最新卡片",
		Color: 0x0099ff,
	}

	for _, post := range posts {
		value := fmt.Sprintf("> %s · <t:%d:R>", post.Author, post.Timestamp)
		if post.ID != "" {
			value += fmt.Sprintf("\n> <#%s>\n", post.ID)
		}
		if post.Tags != "" && tagMapping != nil {
			tagIDs := strings.Split(post.Tags, ",")
			var tagInfo []string
			for _, tagID := range tagIDs {
				for categoryName, category := range tagMapping {
					if tagName, ok := category[tagID]; ok {
						tagInfo = append(tagInfo, fmt.Sprintf("%s: %s", categoryName, tagName))
						break // Assume tag IDs are unique across categories
					}
				}
			}

			if len(tagInfo) > 0 {
				value += " · `" + strings.Join(tagInfo, ", ") + "`"
			}
		}
		latestCardsEmbed.Fields = append(latestCardsEmbed.Fields, &discordgo.MessageEmbedField{
			Name:   post.Title,
			Value:  value,
			Inline: false,
		})
	}

	return latestCardsEmbed, nil
}
