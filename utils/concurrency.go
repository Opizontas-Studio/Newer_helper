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
