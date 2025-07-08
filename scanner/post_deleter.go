package scanner

import (
	"discord-bot/model"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	postDir             = "data/new_post/"
	trackerFile         = "data/post_scan_tracker.json"
	activeScanThreshold = 3
	maxFileAge          = 48 * time.Hour
)

type ScanTracker struct {
	Files map[string]ScanInfo `json:"files"`
	mu    sync.Mutex
}

type ScanInfo struct {
	ScanCount int `json:"scan_count"`
}

// LoadScanTracker loads the scan tracker data from a JSON file.
func LoadScanTracker() (*ScanTracker, error) {
	data, err := os.ReadFile(trackerFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &ScanTracker{Files: make(map[string]ScanInfo)}, nil
		}
		return nil, err
	}
	var tracker ScanTracker
	if err := json.Unmarshal(data, &tracker); err != nil {
		return nil, err
	}
	if tracker.Files == nil {
		tracker.Files = make(map[string]ScanInfo)
	}
	return &tracker, nil
}

// Save saves the scan tracker data to a JSON file.
func (st *ScanTracker) Save() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	data, err := json.MarshalIndent(st.Files, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(trackerFile, data, 0644)
}

// CheckDeletedPosts scans for deleted posts based on the scan type.
func CheckDeletedPosts(s *discordgo.Session, logChannelID string, scanType string) {
	tracker, err := LoadScanTracker()
	if err != nil {
		log.Printf("Error loading scan tracker: %v", err)
		return
	}

	files, err := os.ReadDir(postDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error reading post directory %s: %v", postDir, err)
		}
		return
	}

	var wg sync.WaitGroup
	workerLimit := 10 // Limit to 10 concurrent workers
	guard := make(chan struct{}, workerLimit)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			log.Printf("Error getting file info for %s: %v", file.Name(), err)
			continue
		}

		// Filter by file age
		if time.Since(info.ModTime()) > maxFileAge {
			tracker.mu.Lock()
			delete(tracker.Files, file.Name())
			tracker.mu.Unlock()
			continue
		}

		// Filter by scan type
		scanInfo := tracker.Files[file.Name()]
		isDegraded := scanInfo.ScanCount >= activeScanThreshold

		shouldScan := (scanType == "active" && !isDegraded) || (scanType == "degraded" && isDegraded)

		if !shouldScan {
			continue
		}

		wg.Add(1)
		guard <- struct{}{} // Acquire a worker slot
		go func(file fs.DirEntry, filePath string) {
			defer func() {
				<-guard // Release the worker slot
				wg.Done()
			}()
			processPostFile(s, file, filePath, tracker)
		}(file, filepath.Join(postDir, file.Name()))
	}
	wg.Wait()

	if err := tracker.Save(); err != nil {
		log.Printf("Error saving scan tracker: %v", err)
	}
}

func processPostFile(s *discordgo.Session, file fs.DirEntry, filePath string, tracker *ScanTracker) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading post file %s: %v", filePath, err)
		return
	}

	var posts []model.Post
	if err := json.Unmarshal(fileData, &posts); err != nil {
		log.Printf("Error unmarshalling posts from %s: %v", filePath, err)
		return
	}

	var validPosts []model.Post
	var deletedCount int
	for _, post := range posts {
		// Using s.Channel to check for existence, as a deleted post's channel is also deleted.
		_, err := s.Channel(post.ID)
		if err == nil {
			validPosts = append(validPosts, post)
		} else {
			deletedCount++
		}
	}

	if deletedCount > 0 {
		log.Printf("Found %d deleted posts in %s", deletedCount, file.Name())
		if len(validPosts) == 0 {
			if err := os.Remove(filePath); err != nil {
				log.Printf("Error removing empty post file %s: %v", filePath, err)
			} else {
				log.Printf("Removed empty post file %s", file.Name())
				tracker.mu.Lock()
				delete(tracker.Files, file.Name())
				tracker.mu.Unlock()
			}
		} else {
			newData, err := json.MarshalIndent(validPosts, "", "  ")
			if err != nil {
				log.Printf("Error marshalling updated posts for %s: %v", file.Name(), err)
			} else {
				if err := os.WriteFile(filePath, newData, 0644); err != nil {
					log.Printf("Error writing updated posts to %s: %v", filePath, err)
				}
			}
		}
	}

	// Increment scan count for active scans
	tracker.mu.Lock()
	scanInfo := tracker.Files[file.Name()]
	if scanInfo.ScanCount < activeScanThreshold {
		scanInfo.ScanCount++
		tracker.Files[file.Name()] = scanInfo
		log.Printf("Incremented scan count for %s to %d", file.Name(), scanInfo.ScanCount)
	}
	tracker.mu.Unlock()
}
