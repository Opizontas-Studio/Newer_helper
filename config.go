package main

import (
	"discord-bot/model"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// LoadConfig 从环境变量和数据库加载配置
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

	disableInitialScan := os.Getenv("DISABLE_INITIAL_SCAN") == "true"

	cfg := &model.Config{
		BotToken:           token,
		LogChannelID:       logChannelID,
		DisableInitialScan: disableInitialScan,
		ServerConfigs:      make(map[string]model.ServerConfig),
	}

	return cfg
}
