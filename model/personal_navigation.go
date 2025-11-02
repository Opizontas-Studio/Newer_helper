package model

// PersonalNavigation represents a user's personal navigation entry.
type PersonalNavigation struct {
	ID                   int64  // 全局唯一自增ID
	UserID               string
	GuildID              string
	NavID                int
	ChannelID            string `gorm:"type:TEXT"`
	TableName            string `gorm:"type:TEXT"`
	ChannelName          string `gorm:"type:TEXT"`
	MessageChannelID     string
	MessageIDMyWorks     string // 可存储多个消息ID，用逗号分隔（如 "id1,id2,id3"）
	MessageIDTopWorks    string
	MessageIDLatestWorks string
	UpdateMode           string `gorm:"type:TEXT;default:'edit'"` // 更新方式："edit" (修改消息) 或 "delete" (删除更新)
}
