package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"newer_helper/model"
	"os"
	"sync"
	"time"
)

var (
	panelData     *model.PersistentPanelData
	panelDataFile = "./data/persistent_panels.json"
	panelMutex    sync.RWMutex
)

// LoadPanelData 加载面板数据
func LoadPanelData() error {
	panelMutex.Lock()
	defer panelMutex.Unlock()

	if panelData != nil {
		return nil
	}

	panelData = &model.PersistentPanelData{
		Panels: make(map[string]map[string]*model.PersistentPanelInfo),
	}

	// 确保data目录存在
	if err := os.MkdirAll("./data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// 如果文件不存在，创建空文件
	if _, err := os.Stat(panelDataFile); os.IsNotExist(err) {
		return savePanelDataInternal()
	}

	data, err := os.ReadFile(panelDataFile)
	if err != nil {
		return fmt.Errorf("failed to read panel data file: %v", err)
	}

	if err := json.Unmarshal(data, panelData); err != nil {
		return fmt.Errorf("failed to unmarshal panel data: %v", err)
	}

	// 确保map不为nil
	if panelData.Panels == nil {
		panelData.Panels = make(map[string]map[string]*model.PersistentPanelInfo)
	}

	log.Printf("Loaded persistent panel data for %d guilds", len(panelData.Panels))
	return nil
}

// savePanelDataInternal 内部保存函数，不加锁
func savePanelDataInternal() error {
	if panelData == nil {
		return fmt.Errorf("panel data not initialized")
	}

	data, err := json.MarshalIndent(panelData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal panel data: %v", err)
	}

	if err := os.WriteFile(panelDataFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write panel data file: %v", err)
	}

	return nil
}

// SavePanelData 保存面板数据到文件
func SavePanelData() error {
	panelMutex.Lock()
	defer panelMutex.Unlock()
	return savePanelDataInternal()
}

// SavePersistentPanel 保存持久化面板信息
func SavePersistentPanel(guildID, channelID, messageID, title, description, scope string) error {
	if err := LoadPanelData(); err != nil {
		return err
	}

	panelMutex.Lock()
	defer panelMutex.Unlock()

	if panelData.Panels[guildID] == nil {
		panelData.Panels[guildID] = make(map[string]*model.PersistentPanelInfo)
	}

	panelData.Panels[guildID][channelID] = &model.PersistentPanelInfo{
		MessageID:   messageID,
		Title:       title,
		Description: description,
		Scope:       scope,
		LastUpdated: time.Now().Unix(),
	}

	return savePanelDataInternal()
}

// GetPersistentPanel 获取持久化面板信息
func GetPersistentPanel(guildID, channelID string) (*model.PersistentPanelInfo, bool) {
	if err := LoadPanelData(); err != nil {
		log.Printf("Failed to load panel data: %v", err)
		return nil, false
	}

	panelMutex.RLock()
	defer panelMutex.RUnlock()

	if guild, exists := panelData.Panels[guildID]; exists {
		if panel, exists := guild[channelID]; exists {
			return panel, true
		}
	}

	return nil, false
}

// UpdatePanelMessageID 更新面板消息ID
func UpdatePanelMessageID(guildID, channelID, newMessageID string) error {
	if err := LoadPanelData(); err != nil {
		return err
	}

	panelMutex.Lock()
	defer panelMutex.Unlock()

	if guild, exists := panelData.Panels[guildID]; exists {
		if panel, exists := guild[channelID]; exists {
			panel.MessageID = newMessageID
			panel.LastUpdated = time.Now().Unix()
			return savePanelDataInternal()
		}
	}

	return fmt.Errorf("panel not found for guild %s, channel %s", guildID, channelID)
}

// DeletePersistentPanel 删除持久化面板信息
func DeletePersistentPanel(guildID, channelID string) error {
	if err := LoadPanelData(); err != nil {
		return err
	}

	panelMutex.Lock()
	defer panelMutex.Unlock()

	if guild, exists := panelData.Panels[guildID]; exists {
		delete(guild, channelID)
		// 如果guild下没有频道了，也删除guild
		if len(guild) == 0 {
			delete(panelData.Panels, guildID)
		}
		return savePanelDataInternal()
	}

	return nil // 不存在就当作已删除
}

// GetAllGuildPanels 获取指定公会的所有持久化面板
func GetAllGuildPanels(guildID string) map[string]*model.PersistentPanelInfo {
	if err := LoadPanelData(); err != nil {
		log.Printf("Failed to load panel data: %v", err)
		return nil
	}

	panelMutex.RLock()
	defer panelMutex.RUnlock()

	if guild, exists := panelData.Panels[guildID]; exists {
		// 返回副本以避免并发问题
		result := make(map[string]*model.PersistentPanelInfo)
		for channelID, panel := range guild {
			result[channelID] = panel
		}
		return result
	}

	return nil
}
