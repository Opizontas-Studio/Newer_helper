package handlers

import (
	"database/sql"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
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
	allowedChannels := make(map[string]bool)
	for _, guildConfig := range cfg.TaskConfig {
		for _, channelConfig := range guildConfig.Data {
			allowedChannels[channelConfig.ChannelID] = true
		}
	}

	if _, ok := allowedChannels[t.ParentID]; !ok {
		return
	}

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

	var tagNames []string
	if t.AppliedTags != nil {
		for _, tagID := range t.AppliedTags {
			tagNames = append(tagNames, string(tagID))
		}
	}

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

	guildID := t.GuildID
	filePath := fmt.Sprintf("data/new_post/%s.json", guildID)
	var posts []model.Post

	if err := os.MkdirAll("data/new_post", 0755); err != nil {
		utils.LogError(s, logChannelID, "NewPost", "MkdirAll", fmt.Sprintf("Error creating directory: %v", err))
		return
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			utils.LogError(s, logChannelID, "NewPost", "ReadFile", fmt.Sprintf("Error reading post file %s: %v", filePath, err))
			return
		}
	} else {
		if err := json.Unmarshal(fileData, &posts); err != nil {
			utils.LogError(s, logChannelID, "NewPost", "Unmarshal", fmt.Sprintf("Error unmarshalling posts from %s: %v. Initializing new slice.", filePath, err))
		}
	}

	posts = append(posts, post)
	const maxPosts = 128
	if len(posts) > maxPosts {
		posts = posts[len(posts)-maxPosts:]
	}

	jsonData, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		utils.LogError(s, logChannelID, "NewPost", "Marshal", fmt.Sprintf("Error marshalling posts to JSON: %v", err))
		return
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		utils.LogError(s, logChannelID, "NewPost", "WriteFile", fmt.Sprintf("Error writing posts to file %s: %v", filePath, err))
	} else {
		utils.LogInfo(s, logChannelID, "NewPost", "Save", fmt.Sprintf("Successfully saved new post <#%s> to channel <#%s> in `%s`", post.ID, post.ChannelID, filePath))
	}
}

func HandleThreadDelete(s *discordgo.Session, t *discordgo.ThreadDelete, cfg *model.Config) {
	logChannelID := cfg.LogChannelID

	guildID := t.GuildID
	channelID := t.ParentID

	taskGuildConfig, taskOk := cfg.TaskConfig[guildID]
	rollCardGuildConfig, rollCardOk := cfg.RollCardConfigs[guildID]

	if !taskOk || !rollCardOk {
		return
	}

	var areaName string
	for name, data := range taskGuildConfig.Data {
		if data.ChannelID == channelID {
			areaName = name
			break
		}
	}

	if areaName == "" {
		return
	}

	if len(channelID) < 4 {
		return
	}
	channelIDSuffix := channelID[len(channelID)-4:]
	tableName := fmt.Sprintf("%s_%s", areaName, channelIDSuffix)

	dbPath := rollCardGuildConfig.Database

	if dbPath == "" {
		utils.LogWarn(s, logChannelID, "ThreadDelete", "DBPath", fmt.Sprintf("DB path not found for guild %s", guildID))
		return
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		utils.LogError(s, logChannelID, "ThreadDelete", "DBOpen", fmt.Sprintf("Error opening database %s: %v", dbPath, err))
		return
	}
	defer db.Close()

	if err := database.DeletePost(db, tableName, t.ID); err != nil {
		utils.LogError(s, logChannelID, "ThreadDelete", "DeletePost", fmt.Sprintf("Error deleting post %s from table `%s` in db `%s`: %v", t.ID, tableName, dbPath, err))
		return
	}

	utils.LogInfo(s, logChannelID, "ThreadDelete", "Success", fmt.Sprintf("Successfully deleted post with ID `%s` from table `%s`", t.ID, tableName))
}
