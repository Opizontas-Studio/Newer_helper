package commands

import (
	"discord-bot/model"
	"discord-bot/utils"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func HandleThreadCreate(s *discordgo.Session, t *discordgo.ThreadCreate, logChannelID string) {
	// Load and parse task_config.json to get the list of monitored channels
	configFile, err := os.ReadFile("data/task_config.json")
	if err != nil {
		utils.LogError(s, logChannelID, "NewPost", "ReadFile", fmt.Sprintf("Error reading task_config.json: %v", err))
		return
	}

	var tasks map[string]struct {
		Data map[string]struct {
			ChannelID string `json:"channel_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(configFile, &tasks); err != nil {
		utils.LogError(s, logChannelID, "NewPost", "Unmarshal", fmt.Sprintf("Error unmarshalling task_config.json: %v", err))
		return
	}

	// Collect all allowed channel IDs into a map for quick lookup
	allowedChannels := make(map[string]bool)
	for _, guildConfig := range tasks {
		for _, channelConfig := range guildConfig.Data {
			allowedChannels[channelConfig.ChannelID] = true
		}
	}

	// Check if the thread was created in one of the monitored channels
	if _, ok := allowedChannels[t.ParentID]; !ok {
		return
	}

	// Get the first message of the thread
	firstMessage, err := s.ChannelMessage(t.ID, t.ID)
	if err != nil {
		var restErr *discordgo.RESTError
		if errors.As(err, &restErr) && restErr.Response != nil && restErr.Response.StatusCode == 404 {
			utils.LogWarn(s, logChannelID, "NewPost", "GetMessage", fmt.Sprintf("我们收到了来自 discord API 对应帖子 <#%s> 的 404 返回， waiting 30s to retry...", t.ID))
			time.Sleep(30 * time.Second)
			firstMessage, err = s.ChannelMessage(t.ID, t.ID)
		}

		if err != nil {
			utils.LogError(s, logChannelID, "NewPost", "GetMessage", fmt.Sprintf("Error getting first message for thread <#%s>: %v", t.ID, err))
			return
		}
	}

	// Extract tags
	var tagNames []string
	if t.AppliedTags != nil {
		for _, tagID := range t.AppliedTags {
			tagNames = append(tagNames, string(tagID))
		}
	}

	// Truncate content to 512 characters
	content := firstMessage.Content
	if len(content) > 512 {
		content = content[:512]
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
		Timestamp:     firstMessage.Timestamp.Unix(),
		CoverImageURL: coverImageURL,
	}

	// Define file path and data structure
	guildID := t.GuildID
	channelID := t.ParentID
	filePath := fmt.Sprintf("data/new_post/%s.json", guildID)
	var channelPosts map[string][]model.Post

	if err := os.MkdirAll("data/new_post", 0755); err != nil {
		utils.LogError(s, logChannelID, "NewPost", "MkdirAll", fmt.Sprintf("Error creating directory: %v", err))
		return
	}

	// Read existing data from the file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, initialize a new map
			channelPosts = make(map[string][]model.Post)
		} else {
			utils.LogError(s, logChannelID, "NewPost", "ReadFile", fmt.Sprintf("Error reading post file %s: %v", filePath, err))
			return
		}
	} else {
		// File exists, unmarshal the data
		if err := json.Unmarshal(fileData, &channelPosts); err != nil {
			utils.LogError(s, logChannelID, "NewPost", "Unmarshal", fmt.Sprintf("Error unmarshalling posts from %s: %v. Initializing new map.", filePath, err))
			channelPosts = make(map[string][]model.Post)
		}
	}

	// Append the new post
	channelPosts[channelID] = append(channelPosts[channelID], post)

	// Marshal the updated data back to JSON
	jsonData, err := json.MarshalIndent(channelPosts, "", "  ")
	if err != nil {
		utils.LogError(s, logChannelID, "NewPost", "Marshal", fmt.Sprintf("Error marshalling posts to JSON: %v", err))
		return
	}

	// Write the JSON back to the file
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		utils.LogError(s, logChannelID, "NewPost", "WriteFile", fmt.Sprintf("Error writing posts to file %s: %v", filePath, err))
	} else {
		utils.LogInfo(s, logChannelID, "NewPost", "Save", fmt.Sprintf("Successfully saved new post <#%s> to channel <#%s> in `%s`", post.ID, channelID, filePath))
	}
}

func CleanOldPosts(s *discordgo.Session, logChannelID string) {
	const postDir = "data/new_post/"
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour).Unix()

	files, err := os.ReadDir(postDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		utils.LogError(s, logChannelID, "CleanPosts", "ReadDir", fmt.Sprintf("Error reading post directory %s: %v", postDir, err))
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := fmt.Sprintf("%s%s", postDir, file.Name())
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			utils.LogError(s, logChannelID, "CleanPosts", "ReadFile", fmt.Sprintf("Error reading post file %s: %v", filePath, err))
			continue
		}

		var channelPosts map[string][]model.Post
		if err := json.Unmarshal(fileData, &channelPosts); err != nil {
			utils.LogError(s, logChannelID, "CleanPosts", "Unmarshal", fmt.Sprintf("Error unmarshalling posts from %s: %v", filePath, err))
			continue
		}

		dirty := false
		for channelID, posts := range channelPosts {
			var newPosts []model.Post
			var removedPosts []string
			for _, post := range posts {
				if post.Timestamp >= sevenDaysAgo {
					newPosts = append(newPosts, post)
				} else {
					dirty = true
					removedPosts = append(removedPosts, post.ID)
				}
			}
			if dirty {
				channelPosts[channelID] = newPosts
				utils.LogInfo(s, logChannelID, "CleanPosts", "Remove", fmt.Sprintf("Removing old posts %v from channel %s in %s", removedPosts, channelID, filePath))
			}
		}

		if dirty {
			allChannelsEmpty := true
			for _, posts := range channelPosts {
				if len(posts) > 0 {
					allChannelsEmpty = false
					break
				}
			}

			if allChannelsEmpty {
				if err := os.Remove(filePath); err != nil {
					utils.LogError(s, logChannelID, "CleanPosts", "RemoveFile", fmt.Sprintf("Error removing empty post file %s: %v", filePath, err))
				} else {
					utils.LogInfo(s, logChannelID, "CleanPosts", "RemoveFile", fmt.Sprintf("Removed empty post file %s", filePath))
				}
			} else {
				jsonData, err := json.MarshalIndent(channelPosts, "", "  ")
				if err != nil {
					utils.LogError(s, logChannelID, "CleanPosts", "Marshal", fmt.Sprintf("Error marshalling cleaned posts to JSON for %s: %v", filePath, err))
					continue
				}
				if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
					utils.LogError(s, logChannelID, "CleanPosts", "WriteFile", fmt.Sprintf("Error writing cleaned posts to file %s: %v", filePath, err))
				}
			}
		}
	}
}
