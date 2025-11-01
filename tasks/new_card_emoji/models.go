package new_card_emoji

import (
	"sync"
	"time"
)

// PostTimerInfo 存储单个帖子的计时器信息
type PostTimerInfo struct {
	PostID      string    // 帖子ID（Discord线程ID）
	GuildID     string    // 服务器ID
	ChannelID   string    // 频道ID
	CreatedAt   time.Time // 帖子创建时间
	Timer72h    *time.Timer
	Timer144h   *time.Timer
	Sent72h     bool // 72小时是否已发送
	Sent144h    bool // 144小时是否已发送
}

// SentRecord 已发送记录（用于持久化）
type SentRecord struct {
	PostID    string    `json:"post_id"`
	GuildID   string    `json:"guild_id"`
	ChannelID string    `json:"channel_id"`
	CreatedAt int64     `json:"created_at"` // Unix时间戳
	Sent72h   bool      `json:"sent_72h"`
	Sent144h  bool      `json:"sent_144h"`
	SentAt72h int64     `json:"sent_at_72h,omitempty"`  // 72h发送时间
	SentAt144h int64    `json:"sent_at_144h,omitempty"` // 144h发送时间
}

// TimerState 计时器状态（用于持久化）
type TimerState struct {
	Records map[string]*SentRecord `json:"records"` // key: postID
	mu      sync.RWMutex           `json:"-"`
}

// NewTimerState 创建新的计时器状态
func NewTimerState() *TimerState {
	return &TimerState{
		Records: make(map[string]*SentRecord),
	}
}

// GetRecord 获取帖子记录（线程安全）
func (ts *TimerState) GetRecord(postID string) (*SentRecord, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	record, exists := ts.Records[postID]
	return record, exists
}

// SetRecord 设置帖子记录（线程安全）
func (ts *TimerState) SetRecord(postID string, record *SentRecord) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.Records[postID] = record
}

// UpdateSent72h 更新72小时发送状态（线程安全）
func (ts *TimerState) UpdateSent72h(postID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if record, exists := ts.Records[postID]; exists {
		record.Sent72h = true
		record.SentAt72h = time.Now().Unix()
	}
}

// UpdateSent144h 更新144小时发送状态（线程安全）
func (ts *TimerState) UpdateSent144h(postID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if record, exists := ts.Records[postID]; exists {
		record.Sent144h = true
		record.SentAt144h = time.Now().Unix()
	}
}

// QueueItem 队列项
type QueueItem struct {
	PostID    string
	GuildID   string
	ChannelID string // 帖子所在的频道ID
	Trigger   string // "72h" 或 "144h"
}

// MessageQueue 消息队列（线程安全）
type MessageQueue struct {
	items []*QueueItem
	mu    sync.Mutex
}

// NewMessageQueue 创建新的消息队列
func NewMessageQueue() *MessageQueue {
	return &MessageQueue{
		items: make([]*QueueItem, 0),
	}
}

// Enqueue 添加到队列尾部
func (q *MessageQueue) Enqueue(item *QueueItem) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 去重：检查是否已存在
	for _, existing := range q.items {
		if existing.PostID == item.PostID {
			return // 已存在，不重复添加
		}
	}

	q.items = append(q.items, item)
}

// Dequeue 从队列头部取出
func (q *MessageQueue) Dequeue() *QueueItem {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item
}

// Size 获取队列大小
func (q *MessageQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
