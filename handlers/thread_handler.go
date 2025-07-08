package handlers

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

func HandleThreadCreate(s *discordgo.Session, t *discordgo.ThreadCreate, cfg *model.Config) {
	logChannelID := cfg.LogChannelID
	// Collect all allowed channel IDs into a map for quick lookup
	allowedChannels := make(map[string]bool)
	for _, guildConfig := range cfg.TaskConfig {
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
	runes := []rune(content)
	if len(runes) > 512 {
		content = string(runes[:512])
	}

	var coverImageURL string
	if len(firstMessage.Attachments) > 0 {
		coverImageURL = firstMessage.Attachments[0].URL
	}

	post := model.Post{
		ID:            t.ID,
		ChannelID:     t.ParentID,
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
	filePath := fmt.Sprintf("data/new_post/%s.json", guildID)
	var posts []model.Post

	if err := os.MkdirAll("data/new_post", 0755); err != nil {
		utils.LogError(s, logChannelID, "NewPost", "MkdirAll", fmt.Sprintf("Error creating directory: %v", err))
		return
	}

	// Read existing data from the file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			utils.LogError(s, logChannelID, "NewPost", "ReadFile", fmt.Sprintf("Error reading post file %s: %v", filePath, err))
			return
		}
		// File doesn't exist, posts will be an empty slice
	} else {
		// File exists, unmarshal the data
		if err := json.Unmarshal(fileData, &posts); err != nil {
			utils.LogError(s, logChannelID, "NewPost", "Unmarshal", fmt.Sprintf("Error unmarshalling posts from %s: %v. Initializing new slice.", filePath, err))
			// Keep posts as an empty slice
		}
	}

	// Append the new post and sort by timestamp
	posts = append(posts, post)

	// Marshal the updated data back to JSON
	jsonData, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		utils.LogError(s, logChannelID, "NewPost", "Marshal", fmt.Sprintf("Error marshalling posts to JSON: %v", err))
		return
	}

	// Write the JSON back to the file
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		utils.LogError(s, logChannelID, "NewPost", "WriteFile", fmt.Sprintf("Error writing posts to file %s: %v", filePath, err))
	} else {
		utils.LogInfo(s, logChannelID, "NewPost", "Save", fmt.Sprintf("Successfully saved new post <#%s> to channel <#%s> in `%s`", post.ID, post.ChannelID, filePath))
	}
}
