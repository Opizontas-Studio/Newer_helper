package services

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

// discordService 实现 DiscordService 接口
type discordService struct {
	session *discordgo.Session
	token   string
}

// NewDiscordService 创建新的Discord服务
func NewDiscordService(token string) (DiscordService, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	// 设置必要的意图
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentMessageContent
	session.StateEnabled = false

	return &discordService{
		session: session,
		token:   token,
	}, nil
}

// GetSession 获取Discord会话
func (d *discordService) GetSession() *discordgo.Session {
	return d.session
}

// Connect 连接到Discord
func (d *discordService) Connect() error {
	log.Println("Connecting to Discord...")
	err := d.session.Open()
	if err != nil {
		return err
	}
	log.Println("Successfully connected to Discord")
	return nil
}

// Close 关闭Discord连接
func (d *discordService) Close() error {
	log.Println("Closing Discord connection...")
	return d.session.Close()
}
