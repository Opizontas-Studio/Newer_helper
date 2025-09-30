package utils

import (
	"sync"
	"time"
)

var (
	punishLocks = make(map[string]time.Time)
	punishMutex = &sync.Mutex{}
)

const punishLockDuration = 5 * time.Minute

// CheckAndSetPunishLock checks if a user is currently under a punish lock.
// If not locked, it sets a new lock and returns true.
// If locked, it returns false.
func CheckAndSetPunishLock(userID string) bool {
	punishMutex.Lock()
	defer punishMutex.Unlock()

	if lastPunishTime, ok := punishLocks[userID]; ok {
		if time.Since(lastPunishTime) < punishLockDuration {
			return false // Locked
		}
	}

	punishLocks[userID] = time.Now()
	return true // Not locked, new lock set
}

// ResetAllPunishLocks removes all active punish locks.
func ResetAllPunishLocks() {
	punishMutex.Lock()
	defer punishMutex.Unlock()
	punishLocks = make(map[string]time.Time)
}

// AdminActionRateLimiter implementation
var (
	adminActionRecords = make(map[string]map[string][]time.Time)
	adminActionMutex   = &sync.Mutex{}
)

func init() {
	go cleanupAdminActions()
}

// CheckAndIncrementAdminAction checks and increments the action count for a given admin and action type.
// It returns true if the action is allowed, false otherwise.
func CheckAndIncrementAdminAction(adminID, actionType string, limit int, duration time.Duration) bool {
	if limit < 0 {
		return true // Unlimited
	}

	adminActionMutex.Lock()
	defer adminActionMutex.Unlock()

	now := time.Now()
	if _, ok := adminActionRecords[adminID]; !ok {
		adminActionRecords[adminID] = make(map[string][]time.Time)
	}

	// Clean up old timestamps
	var recentActions []time.Time
	for _, t := range adminActionRecords[adminID][actionType] {
		if now.Sub(t) < duration {
			recentActions = append(recentActions, t)
		}
	}
	adminActionRecords[adminID][actionType] = recentActions

	if len(recentActions) >= limit {
		return false // Limit exceeded
	}

	adminActionRecords[adminID][actionType] = append(adminActionRecords[adminID][actionType], now)
	return true
}

func cleanupAdminActions() {
	for {
		time.Sleep(1 * time.Hour) // Cleanup every hour
		adminActionMutex.Lock()
		now := time.Now()
		for adminID, actions := range adminActionRecords {
			for actionType, timestamps := range actions {
				var recentTimestamps []time.Time
				for _, t := range timestamps {
					if now.Sub(t) < 24*time.Hour {
						recentTimestamps = append(recentTimestamps, t)
					}
				}
				if len(recentTimestamps) == 0 {
					delete(adminActionRecords[adminID], actionType)
				} else {
					adminActionRecords[adminID][actionType] = recentTimestamps
				}
			}
			if len(adminActionRecords[adminID]) == 0 {
				delete(adminActionRecords, adminID)
			}
		}
		adminActionMutex.Unlock()
	}
}
