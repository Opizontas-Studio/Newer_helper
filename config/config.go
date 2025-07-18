package config

import (
	"discord-bot/internal/config"
	"discord-bot/model"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func getEnvAsSlice(name string) []string {
	valStr := os.Getenv(name)
	if valStr == "" {
		return []string{}
	}
	return strings.Split(valStr, ",")
}

// convertPresetMessages 转换预设消息类型
func convertPresetMessages(messages []config.PresetMessage) []model.PresetMessage {
	result := make([]model.PresetMessage, len(messages))
	for i, msg := range messages {
		result[i] = model.PresetMessage{
			ID:          msg.ID,
			Name:        msg.Name,
			Value:       msg.Value,
			Description: msg.Description,
			Type:        msg.Type,
		}
	}
	return result
}

// convertTopChannels 转换回顶频道配置
func convertTopChannels(channels map[string]*config.TopChannelConfig) map[string]*model.TopChannelConfig {
	if channels == nil {
		return nil
	}
	result := make(map[string]*model.TopChannelConfig)
	for k, v := range channels {
		if v != nil {
			result[k] = &model.TopChannelConfig{
				ChannelID:          v.ChannelID,
				MessageLimit:       v.MessageLimit,
				ExcludedMessageIDs: v.ExcludedMessageIDs,
			}
		}
	}
	return result
}

// 全局配置服务实例
var (
	configService *config.Service
)

// Initialize 初始化配置系统
func Initialize() error {
	// 确保加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Println("Info: .env file not found, relying on environment variables")
	}

	configService = config.NewService()

	// 加载配置
	if err := configService.Load(); err != nil {
		return err
	}

	log.Println("Configuration system initialized successfully")
	return nil
}

// Load loads the configuration
func Load() (*model.Config, error) {
	if configService == nil {
		if err := Initialize(); err != nil {
			return nil, err
		}
	}

	// 直接从新的配置服务构建model.Config
	return buildLegacyConfig(configService), nil
}

// buildLegacyConfig 从新配置服务构建旧格式配置
func buildLegacyConfig(service *config.Service) *model.Config {
	newCfg := service.Get()
	if newCfg == nil {
		return nil
	}

	// 转换任务配置
	taskConfig := make(map[string]struct {
		Data map[string]struct {
			ChannelID string `json:"channel_id"`
			TableName string `json:"table_name"`
		} `json:"data"`
	})

	for guildID, task := range newCfg.Tasks {
		taskData := make(map[string]struct {
			ChannelID string `json:"channel_id"`
			TableName string `json:"table_name"`
		})

		for name, channel := range task.Channels {
			taskData[name] = struct {
				ChannelID string `json:"channel_id"`
				TableName string `json:"table_name"`
			}{
				ChannelID: channel.ChannelID,
				TableName: name,
			}
		}

		taskConfig[guildID] = struct {
			Data map[string]struct {
				ChannelID string `json:"channel_id"`
				TableName string `json:"table_name"`
			} `json:"data"`
		}{
			Data: taskData,
		}
	}

	// 转换抽卡配置
	rollCardConfig := model.RollCardConfig{}
	for guildID, rollCard := range newCfg.RollCard {
		rollCardConfig[guildID] = model.RollCardGuildConfig{
			Name:                     rollCard.Name,
			GuildID:                  rollCard.GuildID,
			Database:                 rollCard.Database,
			TagMappingFile:           rollCard.TagMappingFile,
			DataBaseTableNameMapping: rollCard.TableNameMapping,
		}
	}

	// 转换线程配置
	threadConfig := model.ThreadConfig{}
	for guildID, thread := range newCfg.Thread {
		threadConfig[guildID] = model.ThreadGuildConfig{
			Database:  thread.Database,
			TableName: thread.TableName,
		}
	}

	// 转换ServerConfigs类型
	serverConfigs := make(map[string]model.ServerConfig)
	for k, v := range newCfg.ServerConfigs {
		serverConfigs[k] = model.ServerConfig{
			Name:           v.Name,
			GuildID:        v.GuildID,
			AdminRoleIDs:   v.AdminRoleIDs,
			UserRoleIDs:    v.UserRoleIDs,
			PresetMessages: convertPresetMessages(v.PresetMessages),
			TopChannels:    convertTopChannels(v.TopChannels),
		}
	}

	return &model.Config{
		BotToken:           newCfg.BotToken,
		LogChannelID:       newCfg.LogChannelID,
		DeveloperUserIDs:   newCfg.DeveloperUserIDs,
		SuperAdminRoleIDs:  newCfg.SuperAdminRoleIDs,
		DisableInitialScan: newCfg.App.DisableInitialScan,
		ServerConfigs:      serverConfigs,
		RollCardConfigs:    rollCardConfig,
		TaskConfig:         taskConfig,
		ThreadConfig:       threadConfig,
	}
}

// GetService 获取配置服务实例
func GetService() *config.Service {
	return configService
}

// GetConfig 获取配置（直接从服务获取）
func GetConfig() *config.Config {
	if configService == nil {
		return nil
	}
	return configService.Get()
}

// Reload 重新加载配置
func Reload() error {
	if configService != nil {
		return configService.Reload()
	}
	return Initialize()
}
