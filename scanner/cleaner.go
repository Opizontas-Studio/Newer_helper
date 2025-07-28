package scanner

import (
	"context"
	"database/sql"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

// CleanOldPosts iterates through all configured databases and deletes posts older than a week.
func CleanOldPosts(s *discordgo.Session, cfg *model.Config, ctx context.Context) {
	sevenDaysAgo := time.Now().Add(-8 * 24 * time.Hour).Unix()
	logChannelID := cfg.LogChannelID

	log.Println("Starting cleanup of old posts from databases...")

	for guildID, guildConfig := range cfg.ThreadConfig {
		select {
		case <-ctx.Done():
			log.Println("CleanOldPosts cancelled.")
			return
		default:
		}

		dbPath := guildConfig.Database
		tableName := guildConfig.TableName

		if dbPath == "" || tableName == "" {
			utils.LogWarn(s, logChannelID, "CleanDBPosts", "Config", fmt.Sprintf("Database path or table name is empty for guild %s, skipping.", guildID))
			continue
		}

		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			utils.LogError(s, logChannelID, "CleanDBPosts", "DBOpen", fmt.Sprintf("Error opening database %s for guild %s: %v", dbPath, guildID, err))
			continue
		}
		defer db.Close()

		rowsAffected, err := database.DeletePostsOlderThan(db, tableName, sevenDaysAgo)
		if err != nil {
			utils.LogError(s, logChannelID, "CleanDBPosts", "Delete", fmt.Sprintf("Error cleaning old posts from %s (table: %s): %v", dbPath, tableName, err))
			continue
		}

		if rowsAffected > 0 {
			utils.LogInfo(s, logChannelID, "CleanDBPosts", "Success", fmt.Sprintf("Successfully deleted %d old posts from %s (table: %s)", rowsAffected, dbPath, tableName))
		}
	}

	log.Println("Finished cleanup of old posts from databases.")
}
