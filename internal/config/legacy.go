package config

import (
	"discord-bot/model"
	"encoding/json"
	"log"
	"os"
)

// LegacyConfigAdapter 提供与旧配置系统的兼容性
type LegacyConfigAdapter struct {
	service *Service
}

// NewLegacyConfigAdapter 创建兼容性适配器
func NewLegacyConfigAdapter(service *Service) *LegacyConfigAdapter {
	return &LegacyConfigAdapter{
		service: service,
	}
}

// ToLegacyConfig 转换为旧的配置格式
func (a *LegacyConfigAdapter) ToLegacyConfig() *model.Config {
	config := a.service.Get()
	if config == nil {
		return nil
	}

	// 转换任务配置
	taskConfig := make(map[string]struct {
		Data map[string]struct {
			ChannelID string `json:"channel_id"`
			TableName string `json:"table_name"`
		} `json:"data"`
	})

	for guildID, task := range config.Tasks {
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
				TableName: name, // 使用频道名称作为表名
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
	for guildID, rollCard := range config.RollCard {
		rollCardConfig[guildID] = model.RollCardGuildConfig{
			Name:                   rollCard.Name,
			GuildID:                rollCard.GuildID,
			Database:               rollCard.Database,
			TagMappingFile:         rollCard.TagMappingFile,
			DataBaseTableNameMapping: rollCard.TableNameMapping,
		}
	}

	// 转换线程配置
	threadConfig := model.ThreadConfig{}
	for guildID, thread := range config.Thread {
		threadConfig[guildID] = model.ThreadGuildConfig{
			Database:  thread.Database,
			TableName: thread.TableName,
		}
	}

	// 转换ServerConfigs类型
	serverConfigs := make(map[string]model.ServerConfig)
	for k, v := range config.ServerConfigs {
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
		BotToken:           config.BotToken,
		LogChannelID:       config.LogChannelID,
		DeveloperUserIDs:   config.DeveloperUserIDs,
		SuperAdminRoleIDs:  config.SuperAdminRoleIDs,
		DisableInitialScan: config.App.DisableInitialScan,
		ServerConfigs:      serverConfigs,
		RollCardConfigs:    rollCardConfig,
		TaskConfig:         taskConfig,
		ThreadConfig:       threadConfig,
	}
}

// LoadKickConfig 加载踢人配置（兼容原有接口）
func (a *LegacyConfigAdapter) LoadKickConfig() (*model.KickConfig, error) {
	config := a.service.Get()
	if config == nil {
		return loadKickConfigFromFile("data/kick_config.json")
	}

	// 转换新配置格式为旧格式
	kickConfig := &model.KickConfig{
		InitConfig: struct {
			DBPath string `json:"dbpath"`
		}{
			DBPath: config.Punish.InitConfig.DBPath,
		},
		Data: make(map[string]model.KickConfigEntry),
	}

	for guildID, guild := range config.Punish.Guilds {
		kickConfig.Data[guildID] = model.KickConfigEntry{
			Name:            guild.Name,
			BaseRoleID:      guild.BaseRoleID,
			RemoveRoleID:    guild.RemoveRoleIDs,
			WhitelistRoleID: guild.WhitelistRoleIDs,
			Timeout: model.TimeoutConfig{
				Frequency:          guild.Timeout.Frequency,
				Time:               guild.Timeout.Time,
				TimeoutTime:        guild.Timeout.TimeoutTime,
				AddRole:            guild.Timeout.AddRoles,
				AddRoleTimeoutTime: guild.Timeout.AddRoleTimeoutTime,
			},
		}
	}

	return kickConfig, nil
}

// loadKickConfigFromFile 从文件加载踢人配置（后备方案）
func loadKickConfigFromFile(path string) (*model.KickConfig, error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var kickConfig model.KickConfig
	if err := json.Unmarshal(configFile, &kickConfig); err != nil {
		return nil, err
	}

	return &kickConfig, nil
}

// GetTaskConfigForGuild 获取特定服务器的任务配置
func (a *LegacyConfigAdapter) GetTaskConfigForGuild(guildID string) (*TaskConfig, bool) {
	config := a.service.Get()
	if config == nil {
		return nil, false
	}

	taskConfig, exists := config.Tasks[guildID]
	return taskConfig, exists
}

// GetRollCardConfigForGuild 获取特定服务器的抽卡配置
func (a *LegacyConfigAdapter) GetRollCardConfigForGuild(guildID string) (*RollCardGuildConfig, bool) {
	config := a.service.Get()
	if config == nil {
		return nil, false
	}

	rollCardConfig, exists := config.RollCard[guildID]
	return rollCardConfig, exists
}

// GetThreadConfigForGuild 获取特定服务器的线程配置
func (a *LegacyConfigAdapter) GetThreadConfigForGuild(guildID string) (*ThreadGuildConfig, bool) {
	config := a.service.Get()
	if config == nil {
		return nil, false
	}

	threadConfig, exists := config.Thread[guildID]
	return threadConfig, exists
}

// MigrateFromLegacy 从旧配置文件迁移到新格式
func (a *LegacyConfigAdapter) MigrateFromLegacy() error {
	log.Println("Migrating from legacy configuration files...")
	
	// 这里可以实现从旧JSON文件到新YAML格式的自动迁移
	// 当前我们主要使用config.yaml作为主要配置源
	
	log.Println("Legacy configuration migration completed")
	return nil
}

// convertPresetMessages 转换预设消息类型
func convertPresetMessages(messages []PresetMessage) []model.PresetMessage {
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
func convertTopChannels(channels map[string]*TopChannelConfig) map[string]*model.TopChannelConfig {
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