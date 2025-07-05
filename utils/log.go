package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type DiscordWebhookPayload struct {
	Embeds []DiscordEmbed `json:"embeds"`
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

func sendLog(webhookURL string, level LogLevel, module, operation, extraInfo string) error {
	embed := DiscordEmbed{
		Title: string(level) + " Log",
		Color: getColor(level),
		Fields: []DiscordEmbedField{
			{Name: "模块", Value: module},
			{Name: "操作", Value: operation},
			{Name: "附加信息", Value: extraInfo},
		},
	}

	payload := DiscordWebhookPayload{
		Embeds: []DiscordEmbed{embed},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send log to discord, status: %s, body: %s", resp.Status, string(body))
	}

	return nil
}

func LogInfo(webhookURL, module, operation, extraInfo string) error {
	return sendLog(webhookURL, Info, module, operation, extraInfo)
}

func LogWarn(webhookURL, module, operation, extraInfo string) error {
	return sendLog(webhookURL, Warn, module, operation, extraInfo)
}

func LogError(webhookURL, module, operation, extraInfo string) error {
	return sendLog(webhookURL, Error, module, operation, extraInfo)
}
