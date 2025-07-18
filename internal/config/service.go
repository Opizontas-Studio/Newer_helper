package config

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

// Service 配置服务，提供配置管理功能
type Service struct {
	config atomic.Value // 存储 *Config
	mu     sync.RWMutex
}

// NewService 创建新的配置服务
func NewService() *Service {
	return &Service{}
}

// Load 加载配置
func (s *Service) Load() error {
	config, err := Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	s.config.Store(config)
	log.Println("Configuration loaded successfully")
	return nil
}

// Get 获取当前配置
func (s *Service) Get() *Config {
	if config := s.config.Load(); config != nil {
		return config.(*Config)
	}
	return nil
}

// Reload 重新加载配置
func (s *Service) Reload() error {
	log.Println("Reloading configuration...")

	newConfig, err := Load()
	if err != nil {
		log.Printf("Failed to reload config: %v", err)
		return fmt.Errorf("failed to reload config: %w", err)
	}

	// 保留现有的服务器配置
	oldConfig := s.Get()
	if oldConfig != nil {
		newConfig.ServerConfigs = oldConfig.ServerConfigs
	}

	s.config.Store(newConfig)
	log.Println("Configuration reloaded successfully")
	return nil
}

// UpdateServerConfig 更新服务器配置
func (s *Service) UpdateServerConfig(guildID string, serverConfig ServerConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config := s.Get()
	if config != nil {
		config.ServerConfigs[guildID] = serverConfig
	}
}

// GetServerConfig 获取服务器配置
func (s *Service) GetServerConfig(guildID string) (ServerConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config := s.Get()
	if config == nil {
		return ServerConfig{}, false
	}

	serverConfig, exists := config.ServerConfigs[guildID]
	return serverConfig, exists
}

// GetAllServerConfigs 获取所有服务器配置
func (s *Service) GetAllServerConfigs() map[string]ServerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config := s.Get()
	if config == nil {
		return make(map[string]ServerConfig)
	}

	// 返回副本以防止外部修改
	configs := make(map[string]ServerConfig)
	for k, v := range config.ServerConfigs {
		configs[k] = v
	}
	return configs
}

// GetBotToken 获取机器人令牌
func (s *Service) GetBotToken() string {
	config := s.Get()
	if config == nil {
		return ""
	}
	return config.BotToken
}

// GetLogChannelID 获取日志频道ID
func (s *Service) GetLogChannelID() string {
	config := s.Get()
	if config == nil {
		return ""
	}
	return config.LogChannelID
}

// GetDeveloperUserIDs 获取开发者用户ID列表
func (s *Service) GetDeveloperUserIDs() []string {
	config := s.Get()
	if config == nil {
		return []string{}
	}
	return config.DeveloperUserIDs
}

// GetSuperAdminRoleIDs 获取超级管理员角色ID列表
func (s *Service) GetSuperAdminRoleIDs() []string {
	config := s.Get()
	if config == nil {
		return []string{}
	}
	return config.SuperAdminRoleIDs
}

// IsInitialScanDisabled 检查是否禁用初始扫描
func (s *Service) IsInitialScanDisabled() bool {
	config := s.Get()
	if config == nil {
		return false
	}
	return config.App.DisableInitialScan
}

// GetDatabaseConfig 获取数据库配置
func (s *Service) GetDatabaseConfig() DatabaseConfig {
	config := s.Get()
	if config == nil {
		return DatabaseConfig{}
	}
	return config.Database
}

// GetPunishConfig 获取惩罚系统配置
func (s *Service) GetPunishConfig() *PunishConfig {
	config := s.Get()
	if config == nil {
		return &PunishConfig{}
	}
	return &config.Punish
}

// GetTaskConfig 获取任务配置
func (s *Service) GetTaskConfig() map[string]*TaskConfig {
	config := s.Get()
	if config == nil {
		return make(map[string]*TaskConfig)
	}
	return config.Tasks
}

// GetRollCardConfig 获取抽卡配置
func (s *Service) GetRollCardConfig() RollCardConfig {
	config := s.Get()
	if config == nil {
		return make(RollCardConfig)
	}
	return config.RollCard
}

// GetThreadConfig 获取线程配置
func (s *Service) GetThreadConfig() ThreadConfig {
	config := s.Get()
	if config == nil {
		return make(ThreadConfig)
	}
	return config.Thread
}
