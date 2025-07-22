package bot

import (
	"database/sql"
	"discord-bot/handlers/leaderboard"
	"discord-bot/model"
	"discord-bot/scanner"
	"discord-bot/tasks"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// BotProvider defines the methods the scheduler needs from the Bot.
type BotProvider interface {
	GetConfig() *model.Config
	GetDB() *sql.DB
	GetSession() *discordgo.Session
	GetPresetCooldowns() map[string]time.Time
	GetCooldownMutex() *sync.Mutex
	ActiveScanCount() *int
}

// Scheduler manages all scheduled tasks.
type Scheduler struct {
	bot                         BotProvider
	done                        chan struct{}
	wg                          sync.WaitGroup
	cooldownTicker              *time.Ticker
	leaderboardUpdateTicker     *time.Ticker
	postScanTicker              *time.Ticker
	punishmentStatsUpdateTicker *time.Ticker
}

// NewScheduler creates a new scheduler.
func NewScheduler(bot BotProvider) *Scheduler {
	return &Scheduler{
		bot:  bot,
		done: make(chan struct{}),
	}
}

// Start begins all scheduled tasks.
func (s *Scheduler) Start() {
	s.wg.Add(5) // 5 background goroutines

	// Start the initial scan
	go s.runInitialScan()

	// Start the role remover scheduler
	go s.startRoleRemover()

	// Start the channel cleaner scheduler
	go s.startChannelCleaner()

	// Start other scheduled tasks
	go s.startScheduledTasks()

	// Start daily tasks
	go s.startDailyTasks()
}

// Stop terminates all scheduled tasks gracefully.
func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler...")
	close(s.done)
	s.wg.Wait()
	log.Println("Scheduler stopped.")
}

func (s *Scheduler) cleanupCooldowns() {
	s.bot.GetCooldownMutex().Lock()
	defer s.bot.GetCooldownMutex().Unlock()

	for id, t := range s.bot.GetPresetCooldowns() {
		if time.Since(t) > 1*time.Hour {
			delete(s.bot.GetPresetCooldowns(), id)
		}
	}
}

func (s *Scheduler) runInitialScan() {
	defer s.wg.Done()
	if !s.bot.GetConfig().DisableInitialScan {
		scanMode := "full"
		lockFile, err := os.ReadFile("data/scan_lock.json")
		if err == nil {
			var lockData map[string]interface{}
			if json.Unmarshal(lockFile, &lockData) == nil {
				if count, ok := lockData["active_scan_count"].(float64); ok {
					*s.bot.ActiveScanCount() = int(count)
				}
				if mode, ok := lockData["scan_mode"].(string); ok && mode == "full" {
					if timestamp, ok := lockData["timestamp"].(float64); ok {
						if time.Since(time.Unix(int64(timestamp), 0)) < 24*time.Hour {
							scanMode = "active"
							log.Println("Last full scan was less than 24 hours ago. Starting with active scan.")
						}
					}
				}
			}
		}
		scanner.Scan(s.bot.GetSession(), s.bot.GetConfig().LogChannelID, scanMode, "", s.done)
	} else {
		log.Println("Initial scan is disabled by environment variable.")
	}
}

func (s *Scheduler) startRoleRemover() {
	defer s.wg.Done()
	kickConfig, err := utils.LoadKickConfig("data/kick_config.json")
	if err != nil {
		log.Printf("Failed to load kick config for scanner: %v", err)
		return
	}
	timedTaskDB, err := database.InitTimedTaskDB(kickConfig.InitConfig.DBPath)
	if err != nil {
		log.Printf("Failed to initialize timed task DB: %v", err)
		return
	}
	scanner.StartRoleRemover(s.bot.GetSession(), timedTaskDB)
}

func (s *Scheduler) startChannelCleaner() {
	defer s.wg.Done()
	scanner.StartChannelCleaner(s.bot.GetSession(), s.bot.GetConfig(), s.done)
}

func (s *Scheduler) startScheduledTasks() {
	defer s.wg.Done()
	s.cooldownTicker = time.NewTicker(1 * time.Hour)
	s.leaderboardUpdateTicker = time.NewTicker(10 * time.Minute)
	s.postScanTicker = time.NewTicker(30 * time.Minute)
	s.punishmentStatsUpdateTicker = time.NewTicker(1 * time.Hour)

	defer s.cooldownTicker.Stop()
	defer s.leaderboardUpdateTicker.Stop()
	defer s.postScanTicker.Stop()
	defer s.punishmentStatsUpdateTicker.Stop()

	for {
		select {
		case <-s.cooldownTicker.C:
			log.Println("Cleaning up preset cooldowns...")
			s.cleanupCooldowns()
		case <-s.leaderboardUpdateTicker.C:
			log.Println("Updating leaderboard...")
			s.updateLeaderboard()
		case <-s.postScanTicker.C:
			log.Println("Running post deletion scan...")
			scanner.CheckDeletedPosts(s.bot.GetSession(), s.bot.GetConfig().LogChannelID)
		case <-s.punishmentStatsUpdateTicker.C:
			log.Println("Updating punishment stats...")
			s.updatePunishmentStats()
		case <-s.done:
			return
		}
	}
}

func (s *Scheduler) updateLeaderboard() {
	states, err := utils.LoadLeaderboardState()
	if err != nil {
		log.Printf("Error loading leaderboard state for update: %v", err)
		return
	}

	var wg sync.WaitGroup
	workerLimit := 5 // Limit to 5 concurrent workers
	guard := make(chan struct{}, workerLimit)

	for guildID := range states {
		wg.Add(1)
		guard <- struct{}{} // Acquire a worker slot

		go func(guildID string) {
			defer func() {
				<-guard // Release the worker slot
				wg.Done()
			}()
			log.Printf("Updating leaderboard for guild: %s", guildID)
			leaderboard.UpdateLeaderboard(s.bot, guildID)
		}(guildID)
	}

	wg.Wait()
}

func (s *Scheduler) startDailyTasks() {
	defer s.wg.Done()
	runHours := []int{5, 13, 21} // 5 AM, 1 PM, 9 PM

	for {
		now := time.Now()
		var next time.Time

		for _, h := range runHours {
			t := time.Date(now.Year(), now.Month(), now.Day(), h, 0, 0, 0, now.Location())
			if now.Before(t) {
				next = t
				break
			}
		}

		if next.IsZero() {
			tomorrow := now.Add(24 * time.Hour)
			next = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), runHours[0], 0, 0, 0, now.Location())
		}

		log.Printf("Next daily task scheduled for: %v", next)
		select {
		case <-time.After(next.Sub(now)):
			s.runDailyScanTasks()
			s.runDailyPunishmentReport()
		case <-s.done:
			return
		}
	}
}

func (s *Scheduler) runDailyScanTasks() {
	log.Println("Starting scheduled active forum scan...")
	scanner.Scan(s.bot.GetSession(), s.bot.GetConfig().LogChannelID, "active", "", s.done)

	activeScanCount := s.bot.ActiveScanCount()
	*activeScanCount++
	log.Printf("Active scan count: %d", *activeScanCount)

	if *activeScanCount >= 21 { // 3 scans/day * 7 days
		log.Println("Active scan count reached 21. Starting full scan...")
		scanner.Scan(s.bot.GetSession(), s.bot.GetConfig().LogChannelID, "full", "", s.done)
		*activeScanCount = 0
	}

	// Persist the scan count
	lockData := make(map[string]interface{})
	lockFile, err := os.ReadFile("data/scan_lock.json")
	if err == nil {
		json.Unmarshal(lockFile, &lockData)
	}
	lockData["active_scan_count"] = *activeScanCount
	lockFile, err = json.Marshal(lockData)
	if err == nil {
		os.WriteFile("data/scan_lock.json", lockFile, 0644)
	}

	log.Println("Cleaning up old posts...")
	scanner.CleanOldPosts(s.bot.GetSession(), s.bot.GetConfig(), s.done)
}

func (s *Scheduler) updatePunishmentStats() {
	cfg := s.bot.GetConfig()
	if len(cfg.PunishmentStatsChannels) == 0 {
		return
	}

	for _, channelConfig := range cfg.PunishmentStatsChannels {
		go tasks.UpdatePunishmentStats(s.bot.GetSession(), s.bot.GetDB(), channelConfig, time.Hour)
	}
}

func (s *Scheduler) runDailyPunishmentReport() {
	log.Println("Running daily punishment report...")
	cfg := s.bot.GetConfig()
	if len(cfg.PunishmentStatsChannels) == 0 {
		return
	}

	for _, channelConfig := range cfg.PunishmentStatsChannels {
		go tasks.UpdatePunishmentStats(s.bot.GetSession(), s.bot.GetDB(), channelConfig, 24*time.Hour)
	}
}
