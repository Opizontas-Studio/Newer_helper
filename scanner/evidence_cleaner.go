package scanner

import (
	"discord-bot/model"
	"discord-bot/utils"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/bwmarrin/discordgo"
)

// CleanOldEvidence removes files from the evidence path that are older than the configured max age.
func CleanOldEvidence(s *discordgo.Session, cfg *model.Config) {
	cleanerConfig := cfg.EvidenceCleaner
	logChannelID := cfg.LogChannelID

	if cleanerConfig.Path == "" {
		log.Println("Evidence cleaner path is not configured. Skipping.")
		return
	}

	maxAge := time.Duration(cleanerConfig.MaxAgeDays) * 24 * time.Hour
	cutoffTime := time.Now().Add(-maxAge)

	log.Printf("Starting cleanup of old evidence files in %s (older than %d days)...", cleanerConfig.Path, cleanerConfig.MaxAgeDays)

	files, err := os.ReadDir(cleanerConfig.Path)
	if err != nil {
		if os.IsNotExist(err) {
			utils.LogWarn(s, logChannelID, "CleanOldEvidence", "DirNotExist", fmt.Sprintf("Evidence directory %s does not exist. Skipping.", cleanerConfig.Path))
		} else {
			utils.LogError(s, logChannelID, "CleanOldEvidence", "ReadDir", fmt.Sprintf("Error reading evidence directory %s: %v", cleanerConfig.Path, err))
		}
		return
	}

	deletedCount := 0
	for _, file := range files {
		if file.IsDir() {
			continue // Skip directories
		}

		filePath := filepath.Join(cleanerConfig.Path, file.Name())
		info, err := file.Info()
		if err != nil {
			utils.LogWarn(s, logChannelID, "CleanOldEvidence", "Stat", fmt.Sprintf("Could not get file info for %s: %v", filePath, err))
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			err := os.Remove(filePath)
			if err != nil {
				utils.LogError(s, logChannelID, "CleanOldEvidence", "Delete", fmt.Sprintf("Failed to delete old evidence file %s: %v", filePath, err))
			} else {
				log.Printf("Deleted old evidence file: %s", filePath)
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		utils.LogInfo(s, logChannelID, "CleanOldEvidence", "Success", fmt.Sprintf("Successfully deleted %d old evidence files from %s.", deletedCount, cleanerConfig.Path))
	}

	log.Println("Finished cleanup of old evidence files.")
}
