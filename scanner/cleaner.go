package scanner

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

func CleanOldPosts(s *discordgo.Session, logChannelID string, done <-chan struct{}) {
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
		select {
		case <-done:
			log.Println("CleanOldPosts cancelled.")
			return
		default:
		}

		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := fmt.Sprintf("%s%s", postDir, file.Name())
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			utils.LogError(s, logChannelID, "CleanPosts", "ReadFile", fmt.Sprintf("Error reading post file %s: %v", filePath, err))
			continue
		}

		var posts []model.Post
		if err := json.Unmarshal(fileData, &posts); err != nil {
			utils.LogError(s, logChannelID, "CleanPosts", "Unmarshal", fmt.Sprintf("Error unmarshalling posts from %s: %v", filePath, err))
			continue
		}

		var newPosts []model.Post
		var removedPosts []string
		for _, post := range posts {
			if post.Timestamp >= sevenDaysAgo {
				newPosts = append(newPosts, post)
			} else {
				removedPosts = append(removedPosts, post.ID)
			}
		}

		if len(removedPosts) > 0 {
			utils.LogInfo(s, logChannelID, "CleanPosts", "Remove", fmt.Sprintf("Removing %d old posts from %s", len(removedPosts), filePath))
			if len(newPosts) == 0 {
				if err := os.Remove(filePath); err != nil {
					utils.LogError(s, logChannelID, "CleanPosts", "RemoveFile", fmt.Sprintf("Error removing empty post file %s: %v", filePath, err))
				} else {
					utils.LogInfo(s, logChannelID, "CleanPosts", "RemoveFile", fmt.Sprintf("Removed empty post file %s", filePath))
				}
			} else {
				jsonData, err := json.MarshalIndent(newPosts, "", "  ")
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
