package services

import (
	"discord-bot/commands"
	"discord-bot/model"
	"log"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// commandService 实现 CommandService 接口
type commandService struct {
	discord    DiscordService
	config     *model.Config
	registered []*discordgo.ApplicationCommand
	handlers   map[string]func(*discordgo.Session, *discordgo.InteractionCreate)
	mu         sync.RWMutex
}

// NewCommandService 创建新的命令服务
func NewCommandService(discord DiscordService, config *model.Config) (CommandService, error) {
	return &commandService{
		discord:    discord,
		config:     config,
		registered: make([]*discordgo.ApplicationCommand, 0),
		handlers:   make(map[string]func(*discordgo.Session, *discordgo.InteractionCreate)),
	}, nil
}

// RegisterCommands 注册命令到Discord
func (c *commandService) RegisterCommands(guildID string, commands []*discordgo.ApplicationCommand) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	session := c.discord.GetSession()
	log.Printf("Registering %d commands for guild %s...", len(commands), guildID)
	
	registeredCmds, err := session.ApplicationCommandBulkOverwrite(session.State.User.ID, guildID, commands)
	if err != nil {
		return err
	}
	
	c.registered = append(c.registered, registeredCmds...)
	log.Printf("Successfully registered %d commands for guild %s", len(registeredCmds), guildID)
	return nil
}

// RefreshCommands 刷新特定服务器的命令
func (c *commandService) RefreshCommands(guildID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	serverCfg, ok := c.config.ServerConfigs[guildID]
	if !ok {
		log.Printf("Could not find server config for guild: %s", guildID)
		return nil
	}
	
	log.Printf("Refreshing commands for guild %s", serverCfg.GuildID)
	
	cmds := commands.GenerateCommands(&serverCfg)
	session := c.discord.GetSession()
	
	log.Printf("Registering %d new commands for guild %s...", len(cmds), serverCfg.GuildID)
	registeredCmds, err := session.ApplicationCommandBulkOverwrite(session.State.User.ID, serverCfg.GuildID, cmds)
	if err != nil {
		log.Printf("Cannot update commands for guild '%s': %v", serverCfg.GuildID, err)
		return err
	}
	
	c.registered = append(c.registered, registeredCmds...)
	log.Printf("Successfully refreshed commands for guild %s", serverCfg.GuildID)
	return nil
}

// GetRegisteredCommands 获取已注册的命令
func (c *commandService) GetRegisteredCommands() []*discordgo.ApplicationCommand {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// 返回命令的副本以避免并发修改
	result := make([]*discordgo.ApplicationCommand, len(c.registered))
	copy(result, c.registered)
	return result
}

// SetCommandHandlers 设置命令处理器
func (c *commandService) SetCommandHandlers(handlers map[string]func(*discordgo.Session, *discordgo.InteractionCreate)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.handlers = handlers
	
	// 注册处理器到Discord会话
	session := c.discord.GetSession()
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		c.handleInteraction(s, i)
	})
}

// handleInteraction 处理Discord交互
func (c *commandService) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if handler, exists := c.handlers[i.ApplicationCommandData().Name]; exists {
		handler(s, i)
	}
}

// RefreshAllCommands 刷新所有服务器的命令
func (c *commandService) RefreshAllCommands() error {
	log.Println("Refreshing commands for all guilds...")
	
	for _, serverCfg := range c.config.ServerConfigs {
		go func(guildID string) {
			if err := c.RefreshCommands(guildID); err != nil {
				log.Printf("Failed to refresh commands for guild %s: %v", guildID, err)
			}
		}(serverCfg.GuildID)
	}
	
	return nil
}