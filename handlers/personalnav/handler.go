package personalnav

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"

	"github.com/bwmarrin/discordgo"
)

const (
	componentSelectSlot                 = "personal_nav:slot"
	componentSelectAreaPrefix           = "personal_nav:area:"
	componentSubmitAreaPrefix           = "personal_nav:submit_area:"
	componentSubmitUpdateModePrefix     = "personal_nav:submit_update_mode:"
	componentRefreshSelect              = "personal_nav:refresh"
	componentDeleteSelect               = "personal_nav:delete"
	maxNavigationSlots                  = 3
	maxLatestPostsToDisplay             = 10
	embedColorPrimary               int = 0x5865F2
	embedColorHighlight             int = 0xFEE75C
	embedColorSecondary             int = 0x57F287
	updateModeEdit                      = "edit"   // 修改消息模式
	updateModeDelete                    = "delete" // 删除更新模式
)

type ChannelChoice struct {
	TableName   string
	ChannelID   string
	ChannelName string
	PostCount   int
}

// HandlePersonalNavCommand 是处理 `/personal-nav` 斜杠命令的入口函数。
// 它负责解析命令、验证权限，并根据子命令（action）将请求分发到相应的处理函数。
func HandlePersonalNavCommand(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *model.Config) {
	// 验证用户是否在允许的上下文（例如，自己创建的子区或有管理权限的频道）中使用此命令。
	if !validateUsageContext(s, i) {
		utils.SendEphemeralResponse(s, i, "该命令只能在您管理的频道或您创建的子区内使用。")
		return
	}

	guildID := i.GuildID
	requesterID := i.Member.User.ID
	targetUserID := requesterID

	optionMap := map[string]*discordgo.ApplicationCommandInteractionDataOption{}
	for _, opt := range i.ApplicationCommandData().Options {
		optionMap[opt.Name] = opt
	}

	// 开发者可以通过 `userid` 参数为其他用户操作导航。
	if userIDOpt, ok := optionMap["userid"]; ok {
		specifiedUserID := strings.TrimSpace(userIDOpt.StringValue())
		if specifiedUserID != "" {
			// 检查执行者是否是开发者
			isDev := utils.CheckPermission(nil, requesterID, nil, nil, cfg.DeveloperUserIDs, nil) == utils.DeveloperPermission
			if isDev {
				targetUserID = specifiedUserID
				log.Printf("personal-nav: developer %s is acting on behalf of user %s", requesterID, targetUserID)
			} else {
				utils.SendEphemeralResponse(s, i, "您没有权限使用 `userid` 参数。")
				return
			}
		}
	}

	actionOpt, ok := optionMap["action"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "缺少操作类型。")
		return
	}

	input := ""
	if opt, ok := optionMap["input"]; ok {
		input = strings.TrimSpace(opt.StringValue())
	}

	action := actionOpt.StringValue()
	log.Printf("personal-nav: received action=%s guild=%s user=%s channel=%s", action, guildID, targetUserID, i.ChannelID)

	// 根据 action 参数分发到不同的处理函数
	switch action {
	case "create":
		handleCreateAction(s, i, cfg, targetUserID)
	case "refresh":
		handleRefreshAction(s, i, cfg, input, targetUserID)
	case "delete":
		handleDeleteAction(s, i, input, targetUserID)
	default:
		utils.SendEphemeralResponse(s, i, "未知的操作类型。")
	}
}

// handleCreateAction 处理创建导航的逻辑。
// 它会检查用户的可用导航槽位，并引导用户进入分区选择流程。
func handleCreateAction(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *model.Config, userID string) {
	guildID := i.GuildID
	log.Printf("personal-nav: handleCreateAction guild=%s user=%s", guildID, userID)

	// 加载用户已有的导航
	navigations, err := database.GetPersonalNavigations(userID, guildID)
	if err != nil {
		log.Printf("Failed to load personal navigations for %s/%s: %v", guildID, userID, err)
		utils.SendEphemeralResponse(s, i, "读取导航数据时出错。")
		return
	}

	// 查找第一个可用的导航槽位
	slot := findFirstAvailableSlot(navigations)
	if slot == 0 {
		// 如果没有可用槽位，则显示一个选择菜单，让用户选择要覆盖的槽位。
		data := buildSlotSelectionResponse(navigations, userID)
		if data == nil {
			utils.SendEphemeralResponse(s, i, "无法加载现有导航。")
			return
		}

		log.Printf("personal-nav: all slots occupied, prompting override selection.")
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: data,
		})
		if err != nil {
			log.Printf("Failed to send slot selection response: %v", err)
		}
		return
	}

	// 如果有可用槽位，直接进入分区选择界面。
	data, err := buildAreaSelectionResponse(cfg, guildID, userID, slot)
	if err != nil {
		utils.SendEphemeralResponse(s, i, err.Error())
		return
	}
	if data == nil {
		utils.SendEphemeralResponse(s, i, "暂未在任何子区找到属于您的作品。")
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
	if err != nil {
		log.Printf("Failed to send area selection menu: %v", err)
	}
}

// handleRefreshAction 处理刷新导航的逻辑。
// 如果提供了 input（导航ID或消息ID），则直接刷新；否则，显示一个选择菜单让用户选择。
func handleRefreshAction(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *model.Config, input, userID string) {
	guildID := i.GuildID

	if input != "" {
		// 通过输入（ID或消息ID）查找导航
		nav, err := findNavigationFromInput(guildID, userID, input)
		if err != nil {
			utils.SendEphemeralResponse(s, i, err.Error())
			return
		}

		// 调用核心刷新逻辑
		if err := refreshNavigation(s, cfg, i, *nav); err != nil {
			utils.SendEphemeralResponse(s, i, err.Error())
			return
		}
		utils.SendEphemeralResponse(s, i, fmt.Sprintf("导航 ID %d 已刷新。", nav.ID))
		return
	}

	// 如果没有提供 input，则显示选择菜单
	navigations, err := database.GetPersonalNavigations(userID, guildID)
	if err != nil {
		log.Printf("Failed to query navigations for refresh: %v", err)
		utils.SendEphemeralResponse(s, i, "读取导航数据时出错。")
		return
	}
	if len(navigations) == 0 {
		utils.SendEphemeralResponse(s, i, "您还没有创建任何导航。")
		return
	}

	log.Printf("personal-nav: prompting user %s to select navigation for refresh (count=%d)", userID, len(navigations))
	data := buildNavigationSelectionResponse(navigations, "选择需要刷新的导航", componentRefreshSelect, userID)
	if data == nil {
		utils.SendEphemeralResponse(s, i, "无法加载导航列表。")
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
	if err != nil {
		log.Printf("Failed to send refresh selection menu: %v", err)
	}
}

// handleDeleteAction 处理删除导航的逻辑。
// 如果提供了 input（导航ID或消息ID），则直接删除；否则，显示一个选择菜单让用户选择。
func handleDeleteAction(s *discordgo.Session, i *discordgo.InteractionCreate, input, userID string) {
	guildID := i.GuildID

	if input != "" {
		// 通过输入（ID或消息ID）查找导航
		nav, err := findNavigationFromInput(guildID, userID, input)
		if err != nil {
			utils.SendEphemeralResponse(s, i, err.Error())
			return
		}

		// 调用核心删除逻辑
		if err := deleteNavigation(s, *nav); err != nil {
			utils.SendEphemeralResponse(s, i, err.Error())
			return
		}
		utils.SendEphemeralResponse(s, i, fmt.Sprintf("导航 ID %d 已删除。", nav.ID))
		return
	}

	// 如果没有提供 input，则显示选择菜单
	navigations, err := database.GetPersonalNavigations(userID, guildID)
	if err != nil {
		log.Printf("Failed to query navigations for deletion: %v", err)
		utils.SendEphemeralResponse(s, i, "读取导航数据时出错。")
		return
	}
	if len(navigations) == 0 {
		utils.SendEphemeralResponse(s, i, "当前没有可以删除的导航。")
		return
	}

	log.Printf("personal-nav: prompting user %s to select navigation for delete (count=%d)", userID, len(navigations))
	data := buildNavigationSelectionResponse(navigations, "选择需要删除的导航", componentDeleteSelect, userID)
	if data == nil {
		utils.SendEphemeralResponse(s, i, "无法加载导航列表。")
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
	if err != nil {
		log.Printf("Failed to send delete selection menu: %v", err)
	}
}

// findNavigationFromInput 根据用户输入查找导航记录。
// 输入可以是唯一的导航数据库ID，也可以是导航消息的ID。
// 此函数还会检查用户是否有权操作该导航。
func findNavigationFromInput(guildID, userID, input string) (*model.PersonalNavigation, error) {
	// 首先尝试将输入解析为唯一的数据库ID (int64)
	if navID, err := strconv.ParseInt(input, 10, 64); err == nil {
		nav, err := database.GetPersonalNavigationByID(navID)
		if err != nil {
			log.Printf("Failed to locate navigation by id %d: %v", navID, err)
			return nil, fmt.Errorf("查询导航记录时出错。")
		}
		if nav == nil {
			return nil, fmt.Errorf("找不到ID为 %d 的导航。", navID)
		}
		// 验证所有权
		if nav.UserID != userID {
			return nil, fmt.Errorf("您没有权限操作此导航。")
		}
		return nav, nil
	}

	// 如果不是有效的数字，则尝试将其作为消息ID进行查找
	nav, err := database.GetPersonalNavigationByMessageID(guildID, input)
	if err != nil {
		log.Printf("Failed to locate navigation by message id %s: %v", input, err)
		return nil, fmt.Errorf("查询导航记录时出错。")
	}
	if nav == nil {
		return nil, fmt.Errorf("找不到与该消息ID关联的导航。")
	}
	// 验证所有权
	if nav.UserID != userID {
		return nil, fmt.Errorf("您没有权限操作此导航。")
	}
	return nav, nil
}

// findFirstAvailableSlot 查找用户第一个可用的导航槽位 (1 到 maxNavigationSlots)。
// 如果所有槽位都被占用，则返回 0。
func findFirstAvailableSlot(existing []model.PersonalNavigation) int {
	occupied := make(map[int]bool, len(existing))
	for _, nav := range existing {
		occupied[nav.NavID] = true
	}
	for slot := 1; slot <= maxNavigationSlots; slot++ {
		if !occupied[slot] {
			return slot
		}
	}
	return 0
}

// validateUsageContext 验证命令是否在允许的上下文中使用。
// 允许的上下文包括：
// 1. 用户拥有“管理频道”权限的任何频道。
// 2. 用户是所有者（创建者）的任何子区。
// 3. 在用户拥有“管理频道”权限的父频道下创建的任何子区。
func validateUsageContext(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	userID := i.Member.User.ID
	channel, err := s.State.Channel(i.ChannelID)
	if err != nil {
		// 如果状态中没有，则从API获取
		channel, err = s.Channel(i.ChannelID)
		if err != nil {
			log.Printf("Failed to fetch channel %s for personal nav command: %v", i.ChannelID, err)
			return false
		}
	}

	// 检查在当前频道或父频道是否有管理权限
	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err == nil && perms&discordgo.PermissionManageChannels != 0 {
		return true
	}

	// 检查是否是子区的所有者
	switch channel.Type {
	case discordgo.ChannelTypeGuildPublicThread,
		discordgo.ChannelTypeGuildPrivateThread,
		discordgo.ChannelTypeGuildNewsThread:
		if channel.OwnerID == userID {
			return true
		}
		// 检查在父频道是否有管理权限
		if channel.ParentID != "" {
			perms, err = s.UserChannelPermissions(userID, channel.ParentID)
			if err == nil && perms&discordgo.PermissionManageChannels != 0 {
				return true
			}
		}
	default:
		// 对于普通频道，已在前面检查过直接的管理权限。
	}
	return false
}

// BuildChannelChoices collects all channels where the user has posted at least one work.
func BuildChannelChoices(cfg *model.Config, guildID, userID string) ([]ChannelChoice, error) {
	guildTask, ok := cfg.TaskConfig[guildID]
	if !ok {
		return nil, fmt.Errorf("未配置该服务器的分区信息。")
	}

	dbPath := fmt.Sprintf("data/%s.db", guildID)
	db, err := database.InitDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	defer db.Close()

	var choices []ChannelChoice
	for name, channelTask := range guildTask.Data {
		if len(channelTask.ChannelID) < 4 {
			continue
		}
		tableName := fmt.Sprintf("%s_%s", name, channelTask.ChannelID[len(channelTask.ChannelID)-4:])

		if err := database.EnsurePostTableSchema(db, tableName); err != nil {
			log.Printf("personal-nav: failed to ensure schema for table %s when building choices: %v", tableName, err)
			continue
		}

		count, err := database.CountPostsByAuthorInTable(db, tableName, userID)
		if err != nil {
			log.Printf("personal-nav: failed to count posts for user %s in table %s: %v", userID, tableName, err)
			continue
		}

		if count > 0 {
			choices = append(choices, ChannelChoice{
				TableName:   tableName,
				ChannelID:   channelTask.ChannelID,
				ChannelName: name,
				PostCount:   count,
			})
		}
	}

	if len(choices) == 0 {
		// This is not an error, just means the user has no posts. The UI should handle this.
		return []ChannelChoice{}, nil
	}

	return choices, nil
}
