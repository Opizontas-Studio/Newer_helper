package model

// PersonalNavigation represents a user's personal navigation entry.
type PersonalNavigation struct {
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
}
