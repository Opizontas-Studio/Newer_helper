package scanner

import (
	"encoding/json"
	"log"
	"newer_helper/utils/database"
	"os"
	"sync"

	"github.com/bwmarrin/discordgo"
)

const (
	threadConfigPath = "data/thread_config.json"
)

// ThreadConfig defines the structure for each entry in the thread config file.
type ThreadConfig struct {
	Database  string `json:"database"`
	TableName string `json:"tableName"`
}

// CheckDeletedPosts scans for deleted posts based on the new thread_config.json
func CheckDeletedPosts(s *discordgo.Session, logChannelID string) {
	configData, err := os.ReadFile(threadConfigPath)
	if err != nil {
		log.Printf("Error reading thread config file: %v", err)
		return
	}

	var threadConfigs map[string]ThreadConfig
	if err := json.Unmarshal(configData, &threadConfigs); err != nil {
		log.Printf("Error unmarshalling thread config: %v", err)
		return
	}

	var wg sync.WaitGroup
	workerLimit := 10 // Limit to 10 concurrent workers
	guard := make(chan struct{}, workerLimit)

	for threadID, config := range threadConfigs {
		wg.Add(1)
		guard <- struct{}{} // Acquire a worker slot

		go func(threadID string, cfg ThreadConfig) {
			defer func() {
				<-guard // Release the worker slot
				wg.Done()
			}()
			processDatabase(s, cfg.Database, cfg.TableName)
		}(threadID, config)
	}

	wg.Wait()
}

func processDatabase(s *discordgo.Session, dbPath, tableName string) {
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Printf("Error opening database %s: %v", dbPath, err)
		return
	}
	defer db.Close()

	posts, err := database.GetAllPosts(db, tableName)
	if err != nil {
		log.Printf("Error getting posts from %s, table %s: %v", dbPath, tableName, err)
		return
	}

	var deletedCount int
	for _, post := range posts {
		_, err := s.Channel(post.ID)
		if err != nil {
			// Assuming error means channel (post) is deleted
			if err := database.DeletePost(db, tableName, post.ID); err != nil {
				log.Printf("Error deleting post %s from %s: %v", post.ID, dbPath, err)
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		log.Printf("Found and deleted %d posts from %s (table: %s)", deletedCount, dbPath, tableName)
	}
}
