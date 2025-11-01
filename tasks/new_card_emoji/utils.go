package new_card_emoji

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	StateFilePath = "data/new_card_emoji_state.json"
	EmojiMapPath  = "data/emoji_mapping.json"
)

// LoadState 从文件加载计时器状态
func LoadState() (*TimerState, error) {
	state := NewTimerState()

	// 确保目录存在
	dir := filepath.Dir(StateFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}

	// 如果文件不存在，返回空状态
	if _, err := os.Stat(StateFilePath); os.IsNotExist(err) {
		return state, nil
	}

	data, err := os.ReadFile(StateFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %v", err)
	}

	// 解析JSON
	var rawState struct {
		Records map[string]*SentRecord `json:"records"`
	}
	if err := json.Unmarshal(data, &rawState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %v", err)
	}

	state.Records = rawState.Records
	if state.Records == nil {
		state.Records = make(map[string]*SentRecord)
	}

	return state, nil
}

// SaveState 保存计时器状态到文件
func SaveState(state *TimerState) error {
	state.mu.RLock()
	defer state.mu.RUnlock()

	// 确保目录存在
	dir := filepath.Dir(StateFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(StateFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %v", err)
	}

	return nil
}

// LoadEmojiMapping 加载emoji映射
func LoadEmojiMapping() (map[string][]string, error) {
	data, err := os.ReadFile(EmojiMapPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read emoji mapping file: %v", err)
	}

	var emojiMap map[string][]string
	if err := json.Unmarshal(data, &emojiMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal emoji mapping: %v", err)
	}

	return emojiMap, nil
}

// GetRandomEmoji 根据GuildID随机选择一个emoji
func GetRandomEmoji(guildID string) (string, error) {
	emojiMap, err := LoadEmojiMapping()
	if err != nil {
		return "", err
	}

	emojis, ok := emojiMap[guildID]
	if !ok || len(emojis) == 0 {
		return "", fmt.Errorf("no emojis found for guild %s", guildID)
	}

	// 使用当前时间作为随机种子
	rand.Seed(time.Now().UnixNano())
	randomIndex := rand.Intn(len(emojis))
	return emojis[randomIndex], nil
}

// CheckThreadArchived 检查线程是否已归档
func CheckThreadArchived(s *discordgo.Session, threadID string) (bool, error) {
	channel, err := s.Channel(threadID)
	if err != nil {
		return false, fmt.Errorf("failed to get channel info: %v", err)
	}

	// 检查是否是线程
	if channel.Type != discordgo.ChannelTypeGuildPublicThread &&
		channel.Type != discordgo.ChannelTypeGuildPrivateThread {
		return false, fmt.Errorf("channel %s is not a thread", threadID)
	}

	// 检查是否归档
	if channel.ThreadMetadata != nil {
		return channel.ThreadMetadata.Archived, nil
	}

	return false, nil
}

// CheckThreadLocked 检查线程是否已锁定
func CheckThreadLocked(s *discordgo.Session, threadID string) (bool, error) {
	channel, err := s.Channel(threadID)
	if err != nil {
		return false, fmt.Errorf("failed to get channel info: %v", err)
	}

	// 检查是否是线程
	if channel.Type != discordgo.ChannelTypeGuildPublicThread &&
		channel.Type != discordgo.ChannelTypeGuildPrivateThread {
		return false, fmt.Errorf("channel %s is not a thread", threadID)
	}

	// 检查是否锁定
	if channel.ThreadMetadata != nil {
		return channel.ThreadMetadata.Locked, nil
	}

	return false, nil
}

// CheckThreadStatus 检查线程状态（锁定和归档）- 优化版本，只调用一次API
func CheckThreadStatus(s *discordgo.Session, threadID string) (locked bool, archived bool, err error) {
	channel, err := s.Channel(threadID)
	if err != nil {
		return false, false, fmt.Errorf("failed to get channel info: %v", err)
	}

	// 检查是否是线程
	if channel.Type != discordgo.ChannelTypeGuildPublicThread &&
		channel.Type != discordgo.ChannelTypeGuildPrivateThread {
		return false, false, fmt.Errorf("channel %s is not a thread", threadID)
	}

	// 检查状态
	if channel.ThreadMetadata != nil {
		return channel.ThreadMetadata.Locked, channel.ThreadMetadata.Archived, nil
	}

	return false, false, nil
}

// UnarchiveThread 解除线程归档
func UnarchiveThread(s *discordgo.Session, threadID string) error {
	_, err := s.ChannelEditComplex(threadID, &discordgo.ChannelEdit{
		Archived: boolPtr(false),
	})
	if err != nil {
		return fmt.Errorf("failed to unarchive thread: %v", err)
	}

	log.Printf("[NewCardEmoji] Unarchived thread %s", threadID)
	return nil
}

// SendEmojiToThread 发送emoji到线程
func SendEmojiToThread(s *discordgo.Session, threadID, guildID string) error {
	emoji, err := GetRandomEmoji(guildID)
	if err != nil {
		return fmt.Errorf("failed to get random emoji: %v", err)
	}

	_, err = s.ChannelMessageSend(threadID, emoji)
	if err != nil {
		return fmt.Errorf("failed to send emoji message: %v", err)
	}

	log.Printf("[NewCardEmoji] Sent emoji to thread %s: %s", threadID, emoji)
	return nil
}

// CleanupOldRecords 清理超过150小时的旧记录
func CleanupOldRecords(state *TimerState) int {
	state.mu.Lock()
	defer state.mu.Unlock()

	cutoffTime := time.Now().Add(-150 * time.Hour).Unix()
	deletedCount := 0

	for postID, record := range state.Records {
		if record.CreatedAt < cutoffTime {
			delete(state.Records, postID)
			log.Printf("[NewCardEmoji] Cleaned up old record for post %s (created at: %d)", postID, record.CreatedAt)
			deletedCount++
		}
	}

	if deletedCount > 0 {
		log.Printf("[NewCardEmoji] Cleaned up %d old records (older than 150 hours)", deletedCount)
	}

	return deletedCount
}

// boolPtr 返回布尔值的指针
func boolPtr(b bool) *bool {
	return &b
}
