package services

import (
	"sync"
	"time"
)

// cooldownService 实现 CooldownService 接口
type cooldownService struct {
	cooldowns map[string]*CooldownEntry
	mu        sync.RWMutex
}

// NewCooldownService 创建新的冷却服务
func NewCooldownService() CooldownService {
	return &cooldownService{
		cooldowns: make(map[string]*CooldownEntry),
	}
}

// SetCooldown 设置冷却时间
func (c *cooldownService) SetCooldown(key string, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	expireTime := time.Now().Add(duration)
	c.cooldowns[key] = &CooldownEntry{
		ExpireTime: expireTime,
	}
}

// IsOnCooldown 检查是否在冷却期
func (c *cooldownService) IsOnCooldown(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.cooldowns[key]
	if !exists {
		return false
	}
	
	return !entry.IsExpired()
}

// GetCooldownRemaining 获取剩余冷却时间
func (c *cooldownService) GetCooldownRemaining(key string) time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.cooldowns[key]
	if !exists {
		return 0
	}
	
	return entry.GetRemainingTime()
}

// CleanupExpired 清理过期的冷却记录
func (c *cooldownService) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	cleanedCount := 0
	
	for key, entry := range c.cooldowns {
		if now.After(entry.ExpireTime) {
			delete(c.cooldowns, key)
			cleanedCount++
		}
	}
	
	if cleanedCount > 0 {
		// 这里可以添加日志，但为了避免频繁日志，我们只记录清理的数量
		// log.Printf("Cleaned up %d expired cooldown entries", cleanedCount)
	}
}

// GetCooldownCount 获取当前冷却记录数量
func (c *cooldownService) GetCooldownCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cooldowns)
}

// GetAllCooldowns 获取所有冷却记录的键（用于调试）
func (c *cooldownService) GetAllCooldowns() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	keys := make([]string, 0, len(c.cooldowns))
	for key := range c.cooldowns {
		keys = append(keys, key)
	}
	return keys
}

// ClearCooldown 清除特定的冷却记录
func (c *cooldownService) ClearCooldown(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cooldowns, key)
}

// ClearAllCooldowns 清除所有冷却记录
func (c *cooldownService) ClearAllCooldowns() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cooldowns = make(map[string]*CooldownEntry)
}