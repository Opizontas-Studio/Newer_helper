package utils

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

type LogLevel string

const (
	Info  LogLevel = "INFO"
	Warn  LogLevel = "WARN"
	Error LogLevel = "ERROR"
)

type DiscordEmbedField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DiscordEmbed struct {
	Title  string              `json:"title"`
	Color  int                 `json:"color"`
	Fields []DiscordEmbedField `json:"fields"`
}

func getColor(level LogLevel) int {
	switch level {
	case Info:
		return 3066993 // Green
	case Warn:
		return 15105570 // Orange
	case Error:
		return 15158332 // Red
	default:
		return 3447003 // Blue
	}
}

func sendLog(s *discordgo.Session, channelID string, level LogLevel, module, operation, extraInfo string) error {
	embedFields := []*discordgo.MessageEmbedField{}
	if module != "" {
		embedFields = append(embedFields, &discordgo.MessageEmbedField{Name: "模块", Value: module})
	}
	if operation != "" {
		embedFields = append(embedFields, &discordgo.MessageEmbedField{Name: "操作", Value: operation})
	}
	if extraInfo != "" {
		embedFields = append(embedFields, &discordgo.MessageEmbedField{Name: "附加信息", Value: extraInfo})
	}

	embed := &discordgo.MessageEmbed{
		Title:  string(level) + " Log",
		Color:  getColor(level),
		Fields: embedFields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: time.Now().Format(time.RFC1123),
		},
	}

	_, err := s.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		return fmt.Errorf("failed to send log to discord: %w", err)
	}

	return nil
}

func LogInfo(s *discordgo.Session, channelID, module, operation, extraInfo string) error {
	return sendLog(s, channelID, Info, module, operation, extraInfo)
}

func LogWarn(s *discordgo.Session, channelID, module, operation, extraInfo string) error {
	return sendLog(s, channelID, Warn, module, operation, extraInfo)
}

func LogError(s *discordgo.Session, channelID, module, operation, extraInfo string) error {
	return sendLog(s, channelID, Error, module, operation, extraInfo)
}
