package handlers

import (
	"database/sql"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	if err := database.SaveNewPost(cfg, post, guildID, t.ParentID); err != nil {
		utils.LogError(s, logChannelID, "NewPost", "SavePost", fmt.Sprintf("Error saving post: %v", err))
	} else {
		utils.LogInfo(s, logChannelID, "NewPost", "Save", fmt.Sprintf("Successfully saved new post <#%s> to channel <#%s>", post.ID, post.ChannelID))
		if rollCardGuildConfig, ok := cfg.RollCardConfigs[guildID]; ok {
			go utils.PushNewCard(s, guildID, post, &rollCardGuildConfig)
		}
	}
}
func HandleThreadDelete(s *discordgo.Session, t *discordgo.ThreadDelete, cfg *model.Config) {
	logChannelID := cfg.LogChannelID
	guildID := t.GuildID

	threadGuildConfig, ok := cfg.ThreadConfig[guildID]
	if !ok {
		utils.LogWarn(s, logChannelID, "ThreadDelete", "Config", fmt.Sprintf("Thread config not found for guild %s", guildID))
		return
	}

	dbPath := threadGuildConfig.Database
	if dbPath == "" {
		utils.LogWarn(s, logChannelID, "ThreadDelete", "DBPath", fmt.Sprintf("Thread database path is empty for guild %s", guildID))
		return
	}

	tableName := threadGuildConfig.TableName
	if tableName == "" {
		utils.LogWarn(s, logChannelID, "ThreadDelete", "TableName", fmt.Sprintf("Table name is empty for guild %s", guildID))
		return
	}

	// Ensure the directory exists before opening the database
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		utils.LogError(s, logChannelID, "ThreadDelete", "Mkdir", fmt.Sprintf("Error creating directory %s: %v", dir, err))
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
