package commands

import (
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

type GuildConfig struct {
	Name    string `json:"name"`
	GuildID string `json:"guilds_id"`
	Data    map[string]struct {
		ChannelID string   `json:"channel_id"`
		ThreadIDs []string `json:"thread_id"`
	} `json:"data"`
}

func ScanForums(s *discordgo.Session) {
	// Read the configuration file
	file, err := os.ReadFile("data/task_config.json")
	if err != nil {
		log.Printf("Error reading config file: %v", err)
		return
	}

	var config map[string]GuildConfig
	if err := json.Unmarshal(file, &config); err != nil {
		log.Printf("Error unmarshalling config: %v", err)
		return
	}

	for guildID, guildConfig := range config {
		db, err := utils.InitDB(fmt.Sprintf("./data/%s.db", guildID))
		if err != nil {
			log.Printf("Error initializing database for guild %s: %v", guildID, err)
			continue
		}
		defer db.Close()

		for key, channelConfig := range guildConfig.Data {
			channelID := channelConfig.ChannelID
			tableName := fmt.Sprintf("%s_%s", key, channelID[len(channelID)-4:])
			// Get the threads (forum posts) from the channel
			threads, err := s.ThreadsActive(channelConfig.ChannelID)
			if err != nil {
				log.Printf("Error getting threads for channel %s: %v", channelConfig.ChannelID, err)
				continue
			}
			// Also get archived (closed) threads
			var before *time.Time
			for {
				// Limit is 100, the max allowed by the API
				archivedThreads, err := s.ThreadsArchived(channelConfig.ChannelID, before, 100)
				if err != nil {
					log.Printf("Error getting archived threads for channel %s: %v", channelConfig.ChannelID, err)
					break
				}

				if len(archivedThreads.Threads) == 0 {
					break
				}

				threads.Threads = append(threads.Threads, archivedThreads.Threads...)

				if !archivedThreads.HasMore {
					break
				}

				// Set 'before' to the timestamp of the last thread we received to paginate.
				lastThread := archivedThreads.Threads[len(archivedThreads.Threads)-1]
				if lastThread.ThreadMetadata == nil {
					log.Printf("Archived thread %s has no metadata, stopping pagination.", lastThread.ID)
					break
				}
				before = &lastThread.ThreadMetadata.ArchiveTimestamp
			}

			// Create a map for quick lookup of existing thread IDs
			existingThreads := make(map[string]bool)
			for _, id := range channelConfig.ThreadIDs {
				existingThreads[id] = true
			}

			for _, thread := range threads.Threads {
				if _, exists := existingThreads[thread.ID]; exists {
					continue // Skip already scanned threads
				}

				// Get the first message of the thread
				firstMessage, err := s.ChannelMessage(thread.ID, thread.ID)
				if err != nil {
					log.Printf("Error getting first message for thread %s: %v", thread.ID, err)
					continue
				}

				// Extract tags
				var tagNames []string
				if thread.AppliedTags != nil {
					for _, tagID := range thread.AppliedTags {
						// In a real scenario, you might need to fetch tag names from the channel's available tags
						tagNames = append(tagNames, string(tagID))
					}
				}

				// Truncate content to 300 characters
				content := firstMessage.Content
				if len(content) > 300 {
					content = content[:300]
				}

				var coverImageURL string
				if len(firstMessage.Attachments) > 0 {
					coverImageURL = firstMessage.Attachments[0].URL
				}

				post := model.Post{
					ID:            thread.ID,
					Title:         thread.Name,
					Author:        firstMessage.Author.Username,
					AuthorID:      firstMessage.Author.ID,
					Content:       content,
					Tags:          strings.Join(tagNames, ","),
					MessageCount:  thread.MessageCount,
					Timestamp:     firstMessage.Timestamp.Unix(),
					CoverImageURL: coverImageURL,
				}
				if err := utils.InsertPost(db, post, tableName); err != nil {
					log.Printf("Error inserting post %s into database: %v", post.ID, err)
				} else {
					fmt.Printf("Successfully saved post: %s to table %s\n", post.ID, tableName)
				}
			}
		}
	}
}
func HandleThreadCreate(s *discordgo.Session, t *discordgo.ThreadCreate) {
	// Read the configuration file to find the table name
	file, err := os.ReadFile("data/task_config.json")
	if err != nil {
		log.Printf("Error reading config file: %v", err)
		return
	}

	var config map[string]GuildConfig
	if err := json.Unmarshal(file, &config); err != nil {
		log.Printf("Error unmarshalling config: %v", err)
		return
	}

	var tableName string
	var guildID string
	for gID, guildConfig := range config {
		for key, channelConfig := range guildConfig.Data {
			if channelConfig.ChannelID == t.ParentID {
				tableName = fmt.Sprintf("%s_%s", key, t.ParentID[len(t.ParentID)-4:])
				guildID = gID
				break
			}
		}
		if tableName != "" {
			break
		}
	}

	if tableName == "" {
		log.Printf("Could not find matching channel config for channel ID: %s", t.ParentID)
		return
	}

	db, err := utils.InitDB(fmt.Sprintf("./data/%s.db", guildID))
	if err != nil {
		log.Printf("Error initializing database for guild %s: %v", guildID, err)
		return
	}
	defer db.Close()

	// Get the first message of the thread
	firstMessage, err := s.ChannelMessage(t.ID, t.ID)
	if err != nil {
		log.Printf("Error getting first message for thread %s: %v", t.ID, err)
		return
	}

	// Extract tags
	var tagNames []string
	if t.AppliedTags != nil {
		for _, tagID := range t.AppliedTags {
			// In a real scenario, you might need to fetch tag names from the channel's available tags
			tagNames = append(tagNames, string(tagID))
		}
	}

	// Truncate content to 300 characters
	content := firstMessage.Content
	if len(content) > 300 {
		content = content[:300]
	}

	var coverImageURL string
	if len(firstMessage.Attachments) > 0 {
		coverImageURL = firstMessage.Attachments[0].URL
	}

	post := model.Post{
		ID:            t.ID,
		Title:         t.Name,
		Author:        firstMessage.Author.Username,
		AuthorID:      firstMessage.Author.ID,
		Content:       content,
		Tags:          strings.Join(tagNames, ","),
		MessageCount:  1, // A new thread starts with one message
		Timestamp:     firstMessage.Timestamp.Unix(),
		CoverImageURL: coverImageURL,
	}
	if err := utils.InsertPost(db, post, tableName); err != nil {
		log.Printf("Error inserting post %s into database: %v", post.ID, err)
	} else {
		fmt.Printf("Successfully saved new post: %s to table %s\n", post.ID, tableName)
	}
}
