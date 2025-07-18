package database

import (
	"database/sql"
	"discord-bot/model"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// LegacyDatabaseAdapter 提供与旧数据库系统的兼容性
type LegacyDatabaseAdapter struct {
	service *Service
}

// NewLegacyDatabaseAdapter 创建兼容性适配器
func NewLegacyDatabaseAdapter(service *Service) *LegacyDatabaseAdapter {
	return &LegacyDatabaseAdapter{
		service: service,
	}
}

// InitDB 兼容旧的数据库初始化函数
func (a *LegacyDatabaseAdapter) InitDB(filepath string) (*sql.DB, error) {
	return a.service.pool.GetConnection(filepath)
}

// CreateGuildTables 兼容旧的创建服务器表函数
func (a *LegacyDatabaseAdapter) CreateGuildTables(db *sql.DB) error {
	return a.service.createGuildTables(db)
}

// InitUserDB 兼容旧的用户数据库初始化函数
func (a *LegacyDatabaseAdapter) InitUserDB() (*sql.DB, error) {
	db, err := a.service.pool.GetUserDB()
	if err != nil {
		return nil, err
	}

	if err := a.service.createUserTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

// LoadConfigFromDB 从数据库加载配置（兼容旧函数）
func (a *LegacyDatabaseAdapter) LoadConfigFromDB(db *sql.DB, cfg *model.Config) error {
	// 查询所有服务器配置
	rows, err := db.Query(`
		SELECT id, name, admin_role_ids, user_role_ids, preset_messages, top_channels
		FROM servers
	`)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("No server configurations found in database")
			return nil
		}
		return fmt.Errorf("failed to query server configurations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id              string
			name            string
			adminRoleIDs    sql.NullString
			userRoleIDs     sql.NullString
			presetMessages  sql.NullString
			topChannels     sql.NullString
		)

		if err := rows.Scan(&id, &name, &adminRoleIDs, &userRoleIDs, &presetMessages, &topChannels); err != nil {
			log.Printf("Failed to scan server configuration row: %v", err)
			continue
		}

		// 创建服务器配置
		serverConfig := model.ServerConfig{
			GuildID: id,
			Name:    name,
		}

		// 解析角色ID
		if adminRoleIDs.Valid {
			// 这里可以添加JSON解析逻辑
			// 当前简化实现
		}

		if userRoleIDs.Valid {
			// 这里可以添加JSON解析逻辑
			// 当前简化实现
		}

		// 加载预设消息
		if err := a.loadPresetMessages(db, id, &serverConfig); err != nil {
			log.Printf("Failed to load preset messages for guild %s: %v", id, err)
		}

		// 加载回顶频道配置
		if err := a.loadTopChannels(db, id, &serverConfig); err != nil {
			log.Printf("Failed to load top channels for guild %s: %v", id, err)
		}

		cfg.ServerConfigs[id] = serverConfig
	}

	return nil
}

// loadPresetMessages 从数据库加载预设消息
func (a *LegacyDatabaseAdapter) loadPresetMessages(db *sql.DB, guildID string, config *model.ServerConfig) error {
	rows, err := db.Query(`
		SELECT id, name, value, description, type
		FROM preset_messages
		WHERE guild_id = ?
	`, guildID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var presetMessages []model.PresetMessage
	for rows.Next() {
		var (
			id          string
			name        string
			value       string
			description sql.NullString
			messageType sql.NullString
		)

		if err := rows.Scan(&id, &name, &value, &description, &messageType); err != nil {
			continue
		}

		presetMessage := model.PresetMessage{
			ID:    id,
			Name:  name,
			Value: value,
			Type:  "text",
		}

		if description.Valid {
			presetMessage.Description = description.String
		}

		if messageType.Valid {
			presetMessage.Type = messageType.String
		}

		presetMessages = append(presetMessages, presetMessage)
	}

	config.PresetMessages = presetMessages
	return nil
}

// loadTopChannels 从数据库加载回顶频道配置
func (a *LegacyDatabaseAdapter) loadTopChannels(db *sql.DB, guildID string, config *model.ServerConfig) error {
	rows, err := db.Query(`
		SELECT id, channel_id, message_limit, excluded_message_ids
		FROM top_channels
		WHERE guild_id = ?
	`, guildID)
	if err != nil {
		return err
	}
	defer rows.Close()

	topChannels := make(map[string]*model.TopChannelConfig)
	for rows.Next() {
		var (
			id                 string
			channelID          string
			messageLimit       int
			excludedMessageIDs sql.NullString
		)

		if err := rows.Scan(&id, &channelID, &messageLimit, &excludedMessageIDs); err != nil {
			continue
		}

		topChannel := &model.TopChannelConfig{
			ChannelID:    channelID,
			MessageLimit: messageLimit,
		}

		if excludedMessageIDs.Valid {
			// 这里可以添加JSON解析逻辑来解析排除的消息ID
			// 当前简化实现
		}

		topChannels[id] = topChannel
	}

	config.TopChannels = topChannels
	return nil
}

// GetDBSize 获取数据库文件大小（兼容旧函数）
func (a *LegacyDatabaseAdapter) GetDBSize(filepath string) (int64, error) {
	fi, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// GetGuildDatabase 获取服务器数据库连接（新增便捷方法）
func (a *LegacyDatabaseAdapter) GetGuildDatabase(guildID string) (*sql.DB, error) {
	return a.service.pool.GetGuildDB(guildID)
}

// GetUserDatabase 获取用户数据库连接（新增便捷方法）
func (a *LegacyDatabaseAdapter) GetUserDatabase() (*sql.DB, error) {
	return a.service.pool.GetUserDB()
}

// GetKickUserDatabase 获取踢人用户数据库连接（新增便捷方法）
func (a *LegacyDatabaseAdapter) GetKickUserDatabase() (*sql.DB, error) {
	return a.service.pool.GetKickUserDB()
}

// GetTimedTasksDatabase 获取定时任务数据库连接（新增便捷方法）
func (a *LegacyDatabaseAdapter) GetTimedTasksDatabase() (*sql.DB, error) {
	return a.service.pool.GetTimedTasksDB()
}

// GetNewPostDatabase 获取新帖子数据库连接（新增便捷方法）
func (a *LegacyDatabaseAdapter) GetNewPostDatabase(guildID string) (*sql.DB, error) {
	return a.service.pool.GetNewPostDB(guildID)
}