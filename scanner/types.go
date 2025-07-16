package scanner

import (
	"database/sql"

	"github.com/bwmarrin/discordgo"
)

// ThreadChunk 表示线程分片
type ThreadChunk struct {
	Threads []*discordgo.Channel
	Index   int
}

// GuildConfig 定义了服务器的配置结构
type GuildConfig struct {
	Name     string `json:"name"`
	GuildsID string `json:"guilds_id"`
	Data     map[string]struct {
		ChannelID string   `json:"channel_id"`
		ThreadIDs []string `json:"thread_id"`
	} `json:"data"`
}

// PartitionTask 封装了单个分区扫描任务所需的所有信息
type PartitionTask struct {
	GuildConfig        GuildConfig
	Key                string
	DB                 *sql.DB
	IsFullScan         bool
	ScanType           string
	TotalPartitions    int
	PartitionsDone     *int64
	TotalNewPostsFound *int64
}
