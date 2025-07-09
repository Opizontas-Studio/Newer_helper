package handlers

import (
	"discord-bot/utils"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

func SystemInfoHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Get CPU info
	cpuCount, _ := cpu.Counts(true)
	cpuPercent, _ := cpu.Percent(0, false)

	// Get memory info
	vm, _ := mem.VirtualMemory()

	// Get host info
	hostInfo, _ := host.Info()

	// Get database size
	dbFiles, err := utils.ListDBFiles()
	var totalDbSize int64
	if err == nil {
		for _, file := range dbFiles {
			size, err := utils.GetDBSize(file)
			if err == nil {
				totalDbSize += size
			}
		}
	}
	dbSize := totalDbSize / 1024 / 1024 // in MB

	// Get discordgo session stats
	guilds := len(s.State.Guilds)
	users, err := utils.GetTotalUserCount()
	if err != nil {
		users = 0
	}
	var threads int
	type dbMapping struct {
		Database string `json:"database"`
	}
	mappingFile, err := os.ReadFile("data/databaseMapping.json")
	if err == nil {
		var dbMap map[string]dbMapping
		if json.Unmarshal(mappingFile, &dbMap) == nil {
			for _, mapping := range dbMap {
				db, err := utils.InitDB(mapping.Database)
				if err == nil {
					count, err := utils.GetTotalPostCount(db)
					if err == nil {
						threads += count
					}
					db.Close()
				}
			}
		}
	}

	embed := &discordgo.MessageEmbed{
		Title: "系统信息",
		Color: 0x5865F2, // Discord Blurple
		Fields: []*discordgo.MessageEmbedField{
			{Name: "💻 OS 版本", Value: fmt.Sprintf("%s %s", hostInfo.Platform, hostInfo.PlatformVersion), Inline: true},
			{Name: "🔧 内核版本", Value: hostInfo.KernelVersion, Inline: true},
			{Name: "🐹 Go 版本", Value: runtime.Version(), Inline: true},
			{Name: "🔼 CPU 数量", Value: fmt.Sprintf("%d", cpuCount), Inline: true},
			{Name: "🔥 CPU 使用率", Value: fmt.Sprintf("%.1f%%", cpuPercent[0]), Inline: true},
			{Name: "🧠 系统内存", Value: fmt.Sprintf("%.1f%% (%d MB / %d MB)", vm.UsedPercent, vm.Used/1024/1024, vm.Total/1024/1024), Inline: true},
			{Name: "🗃️ 数据库大小", Value: fmt.Sprintf("%d MB", dbSize), Inline: true},
			{Name: "⏱️ WebSocket 延迟", Value: s.HeartbeatLatency().String(), Inline: true},
			{Name: "🚀 Goroutines", Value: fmt.Sprintf("%d", runtime.NumGoroutine()), Inline: true},
			{Name: "🌍 缓存服务器数", Value: fmt.Sprintf("%d", guilds), Inline: true},
			{Name: "👥 缓存用户数", Value: fmt.Sprintf("%d", users), Inline: true},
			{Name: "🗨️ 缓存子区数", Value: fmt.Sprintf("%d", threads), Inline: true},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("系统监控・今天%s", time.Now().Format("15:04")),
		},
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}
