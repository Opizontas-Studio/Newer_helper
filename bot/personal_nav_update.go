package bot

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"newer_helper/handlers/personalnav"
	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"
)

// UpdateResult holds the statistics of a batch navigation update.
type UpdateResult struct {
	Total     int
	Success   int
	Failed    int
	Skipped   int      // 跳过的数量（如因帖子已归档）
	Errors    []string
	StartTime time.Time
	EndTime   time.Time
}

// Duration returns the time taken for the batch update.
func (r *UpdateResult) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

// UpdateAllNavigations performs a scheduled batch update of all personal navigations.
// This method is called by the scheduler and sends a summary to the log channel.
func (b *Bot) UpdateAllNavigations() {
	log.Println("Bot: Starting personal navigation auto-update...")

	result := &UpdateResult{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	// Fetch all navigation records
	navigations, err := database.GetAllPersonalNavigations()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to load navigations: %v", err)
		log.Printf("personal-nav: auto-update error: %s", errMsg)
		result.Errors = append(result.Errors, errMsg)
		result.EndTime = time.Now()
		b.sendUpdateSummary(result)
		return
	}

	result.Total = len(navigations)
	log.Printf("personal-nav: found %d navigations to update", result.Total)

	if result.Total == 0 {
		result.EndTime = time.Now()
		b.sendUpdateSummary(result)
		return
	}

	// Worker pool configuration
	const maxWorkers = 5
	var wg sync.WaitGroup
	navChan := make(chan model.PersonalNavigation, result.Total)
	resultChan := make(chan error, result.Total)

	// Start workers
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for nav := range navChan {
				log.Printf("personal-nav: [worker-%d] updating nav guild=%s user=%s nav=%d",
					workerID, nav.GuildID, nav.UserID, nav.NavID)

				err := personalnav.UpdateNavigationScheduled(b.Session, b.GetConfig(), nav)
				resultChan <- err

				// Small delay to avoid rate limiting
				time.Sleep(500 * time.Millisecond)
			}
		}(i)
	}

	// Dispatch work
	go func() {
		for _, nav := range navigations {
			navChan <- nav
		}
		close(navChan)
	}()

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for err := range resultChan {
		if err != nil {
			// 检查是否为归档错误（跳过的情况）
			if err == personalnav.ErrArchivedThread {
				result.Skipped++
				log.Printf("personal-nav: auto-update skipped (archived thread)")
			} else {
				result.Failed++
				errMsg := err.Error()
				result.Errors = append(result.Errors, errMsg)
				log.Printf("personal-nav: auto-update error: %s", errMsg)
			}
		} else {
			result.Success++
		}
	}

	result.EndTime = time.Now()
	log.Printf("personal-nav: auto-update completed - total=%d success=%d failed=%d skipped=%d duration=%s",
		result.Total, result.Success, result.Failed, result.Skipped, result.Duration())

	b.sendUpdateSummary(result)
}

func (b *Bot) sendUpdateSummary(result *UpdateResult) {
	// Build summary message
	var summary strings.Builder
	summary.WriteString("🔄 **个人导航定时更新完成**\n")
	summary.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	summary.WriteString(fmt.Sprintf("✅ **成功**: %d/%d\n", result.Success, result.Total))
	summary.WriteString(fmt.Sprintf("❌ **失败**: %d/%d\n", result.Failed, result.Total))
	if result.Skipped > 0 {
		summary.WriteString(fmt.Sprintf("⏭️ **跳过**: %d/%d (帖子已归档)\n", result.Skipped, result.Total))
	}
	summary.WriteString(fmt.Sprintf("⏱️ **耗时**: %s\n", result.Duration()))
	summary.WriteString("━━━━━━━━━━━━━━━━━━━━\n")

	if len(result.Errors) > 0 {
		// Limit error display to avoid overly long messages
		maxErrors := 5
		summary.WriteString("**失败详情**:\n")
		for i, err := range result.Errors {
			if i >= maxErrors {
				summary.WriteString(fmt.Sprintf("... 及其他 %d 个错误\n", len(result.Errors)-maxErrors))
				break
			}
			// Truncate long error messages
			errMsg := err
			if len(errMsg) > 100 {
				errMsg = errMsg[:97] + "..."
			}
			summary.WriteString(fmt.Sprintf("%d. %s\n", i+1, errMsg))
		}
	}

	// Send summary to log channel
	logChannelID := b.GetConfig().LogChannelID
	if logChannelID != "" {
		utils.LogInfo(b.Session, logChannelID, "PersonalNav", "AutoUpdate", summary.String())
	} else {
		log.Println("No log channel configured, summary not sent to Discord")
	}

	log.Printf("Bot: Personal navigation auto-update completed - %d/%d succeeded", result.Success, result.Total)
}
