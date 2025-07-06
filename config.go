package main

import (
	"discord-bot/model"
	"encoding/json"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// LoadConfig 从环境变量和JSON文件加载配置
func LoadConfig() *model.Config {
	// 加载.env文件
	err := godotenv.Load()
	if err != nil {
		log.Println("提示: 未找到.env文件，将依赖环境变量")
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("错误: 未设置BOT_TOKEN环境变量")
	}

	logChannelID := os.Getenv("LOG_CHANNEL_ID")
	if logChannelID == "" {
		log.Println("警告: 未设置LOG_CHANNEL_ID环境变量，日志功能将不可用")
	}

	// 从JSON文件加载预设消息
	file, err := os.ReadFile("messages.json")
	if err != nil {
		log.Fatalf("读取messages.json文件错误: %v", err)
	}

	var serverConfigs map[string]model.ServerConfig
	if err := json.Unmarshal(file, &serverConfigs); err != nil {
		log.Fatalf("解析messages.json文件错误: %v", err)
	}

	disableInitialScan := os.Getenv("DISABLE_INITIAL_SCAN") == "true"

	return &model.Config{
		BotToken:           token,
		LogChannelID:       logChannelID,
		ServerConfigs:      serverConfigs,
		DisableInitialScan: disableInitialScan,
	}
}
