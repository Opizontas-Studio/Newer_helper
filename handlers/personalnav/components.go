package personalnav

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"

	"github.com/bwmarrin/discordgo"
)

var (
	// selectionCache stores temporary selections from select menus, keyed by message ID.
	selectionCache = make(map[string][]string)
	cacheLock      sync.Mutex
)

// HandlePersonalNavComponent 是个人导航功能中所有组件交互的总入口和路由器。
// 它解析 customID，并根据其前缀将交互分发到相应的处理函数。
func HandlePersonalNavComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *model.Config, customID string) bool {
	parts := strings.Split(customID, ":")
	baseID := parts[0]
	// 为了处理像 "personal_nav:submit_area:1:user_id" 这样的多段 customID，
	// 我们将除最后一段（通常是用户ID）之外的所有部分连接起来作为基础ID。
	if len(parts) > 1 && strings.HasPrefix(customID, "personal_nav") {
		baseID = strings.Join(parts[:len(parts)-1], ":")
	}

	switch {
	// 用户选择了要创建/覆盖的导航槽位
	case baseID == componentSelectSlot:
		handleSlotSelectionComponent(s, i, cfg, customID)
	// 用户正在与分区选择菜单交互
	case strings.HasPrefix(baseID, componentSelectAreaPrefix):
		handleAreaSelectionMenuComponent(s, i)
	// 用户点击了“确认”按钮，提交分区选择
	case strings.HasPrefix(baseID, componentSubmitAreaPrefix):
		handleAreaSubmitComponent(s, i, customID)
	// 用户点击了“确认”按钮，提交更新模式选择
	case strings.HasPrefix(baseID, componentSubmitUpdateModePrefix):
		handleUpdateModeSubmitComponent(s, i, cfg, customID)
	// 用户从菜单中选择了一个要刷新的导航
	case baseID == componentRefreshSelect:
		handleRefreshSelectionComponent(s, i, cfg, customID)
	// 用户从菜单中选择了一个要删除的导航
	case baseID == componentDeleteSelect:
		handleDeleteSelectionComponent(s, i, customID)
	default:
		// 如果 customID 与个人导航无关，则返回 false，由其他处理器处理
		return false
	}
	return true
}

// handleSlotSelectionComponent 处理用户从槽位选择菜单中做出选择后的逻辑。
// 它会用分区选择界面更新原始消息。
func handleSlotSelectionComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *model.Config, customID string) {
	userID, err := extractUserIDFromCustomID(customID)
	if err != nil {
		utils.SendEphemeralResponse(s, i, "无效的组件ID。")
		return
	}

	values := i.MessageComponentData().Values
	if len(values) == 0 {
		utils.SendEphemeralResponse(s, i, "请选择一个导航槽位。")
		return
	}
	slot, err := strconv.Atoi(values[0])
	if err != nil || slot <= 0 {
		utils.SendEphemeralResponse(s, i, "无效的槽位选择。")
		return
	}

	// 构建分区选择菜单
	data, err := buildAreaSelectionResponse(cfg, i.GuildID, userID, slot)
	if err != nil {
		utils.SendEphemeralResponse(s, i, err.Error())
		return
	}
	if data == nil {
		utils.SendEphemeralResponse(s, i, "暂未在任何子区找到属于您的作品。")
		return
	}

	// 用分区选择菜单更新原始交互消息
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: data,
	})
	if err != nil {
		log.Printf("Failed to update slot selection with area options: %v", err)
	}
}

// handleAreaSelectionMenuComponent 处理用户在分区选择菜单中选择/取消选择一个或多个分区时的交互。
// 它将当前用户的选择暂存到缓存中，并延迟更新消息，等待用户点击“确认”按钮。
func handleAreaSelectionMenuComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 将当前选择存储在缓存中，以消息ID为键。
	cacheLock.Lock()
	selectionCache[i.Message.ID] = i.MessageComponentData().Values
	cacheLock.Unlock()

	// 确认交互，但暂时不发送任何可见的响应。
	// 用户将通过点击提交按钮来最终确认他们的选择。
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		log.Printf("Failed to acknowledge area selection: %v", err)
	}
}

// handleAreaSubmitComponent 处理用户点击“确认分区”按钮后的逻辑。
// 它会从缓存中读取用户选择的分区，并显示更新模式选择界面。
func handleAreaSubmitComponent(s *discordgo.Session, i *discordgo.InteractionCreate, customID string) {
	userID, err := extractUserIDFromCustomID(customID)
	if err != nil {
		utils.SendEphemeralResponse(s, i, "无效的组件ID。")
		return
	}

	// 从缓存中检索选择的分区。
	cacheLock.Lock()
	tableNames := selectionCache[i.Message.ID]
	// 注意：此处不删除缓存，因为在下一步选择更新模式后仍然需要这些分区信息。
	cacheLock.Unlock()

	if len(tableNames) == 0 {
		utils.SendEphemeralResponse(s, i, "您必须至少选择一个分区。")
		return
	}

	parts := strings.Split(customID, ":")
	if len(parts) < 4 {
		utils.SendEphemeralResponse(s, i, "无效的导航槽位ID。")
		return
	}
	slot, err := strconv.Atoi(parts[2])
	if err != nil || slot <= 0 {
		utils.SendEphemeralResponse(s, i, "无效的导航槽位。")
		return
	}

	// 显示更新模式选择界面
	data := buildUpdateModeSelectionResponse(slot, userID)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: data,
	})
	if err != nil {
		log.Printf("Failed to show update mode selection: %v", err)
	}
}

// handleRefreshSelectionComponent 处理用户从刷新选择菜单中选择一个导航后的逻辑。
func handleRefreshSelectionComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *model.Config, customID string) {
	userID, err := extractUserIDFromCustomID(customID)
	if err != nil {
		utils.SendEphemeralResponse(s, i, "无效的组件ID。")
		return
	}

	values := i.MessageComponentData().Values
	if len(values) == 0 {
		utils.SendEphemeralResponse(s, i, "请选择一个导航。")
		return
	}
	navID, err := strconv.Atoi(values[0])
	if err != nil {
		utils.SendEphemeralResponse(s, i, "无效的导航选择。")
		return
	}

	// 延迟响应，因为刷新可能需要一些时间
	if err := deferComponentAck(s, i); err != nil {
		return
	}

	// 从数据库获取导航记录
	nav, err := database.GetPersonalNavigation(userID, i.GuildID, navID)
	if err != nil {
		log.Printf("Failed to load navigation for refresh: %v", err)
		updateComponentMessage(s, i, "❌ 读取导航数据时出错。")
		return
	}
	if nav == nil {
		updateComponentMessage(s, i, "❌ 找不到该导航。")
		return
	}

	// 执行刷新操作
	if err := refreshNavigation(s, cfg, i, *nav); err != nil {
		log.Printf("personal-nav: failed to refresh navigation %d for user %s: %v", navID, userID, err)
		updateComponentMessage(s, i, fmt.Sprintf("❌ 导航刷新失败：%s", err.Error()))
		return
	}

	updateComponentMessage(s, i, "✅ 导航已刷新。")
}

// handleDeleteSelectionComponent 处理用户从删除选择菜单中选择一个导航后的逻辑。
func handleDeleteSelectionComponent(s *discordgo.Session, i *discordgo.InteractionCreate, customID string) {
	userID, err := extractUserIDFromCustomID(customID)
	if err != nil {
		utils.SendEphemeralResponse(s, i, "无效的组件ID。")
		return
	}

	values := i.MessageComponentData().Values
	if len(values) == 0 {
		utils.SendEphemeralResponse(s, i, "请选择一个导航。")
		return
	}
	navID, err := strconv.Atoi(values[0])
	if err != nil {
		utils.SendEphemeralResponse(s, i, "无效的导航选择。")
		return
	}

	// 延迟响应，因为删除可能需要一些时间
	if err := deferComponentAck(s, i); err != nil {
		return
	}

	// 从数据库获取导航记录
	nav, err := database.GetPersonalNavigation(userID, i.GuildID, navID)
	if err != nil {
		log.Printf("Failed to load navigation for delete: %v", err)
		updateComponentMessage(s, i, "❌ 读取导航数据时出错。")
		return
	}
	if nav == nil {
		updateComponentMessage(s, i, "❌ 找不到该导航。")
		return
	}

	// 执行删除操作
	if err := deleteNavigation(s, *nav); err != nil {
		updateComponentMessage(s, i, fmt.Sprintf("❌ 导航删除失败：%s", err.Error()))
		return
	}

	updateComponentMessage(s, i, "✅ 导航已删除。")
}

// handleUpdateModeSubmitComponent 处理用户点击“确认更新模式”按钮后的逻辑。
// 这是创建/更新导航流程的最后一步，它将调用核心逻辑来生成或更新导航消息。
func handleUpdateModeSubmitComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *model.Config, customID string) {
	parts := strings.Split(customID, ":")
	if len(parts) < 5 {
		utils.SendEphemeralResponse(s, i, "无效的组件ID。")
		return
	}

	// 解析 customID: componentSubmitUpdateModePrefix + navID + ":" + updateMode + ":" + userID
	// e.g., "personal_nav:submit_update_mode:1:edit:12345"
	slot, err := strconv.Atoi(parts[2])
	if err != nil || slot <= 0 {
		utils.SendEphemeralResponse(s, i, "无效的导航槽位。")
		return
	}

	updateMode := parts[3]
	if updateMode != updateModeEdit && updateMode != updateModeDelete {
		utils.SendEphemeralResponse(s, i, "无效的更新模式。")
		return
	}

	userID := parts[4]

	// 从缓存中获取之前选择的分区
	cacheLock.Lock()
	tableNames := selectionCache[i.Message.ID]
	delete(selectionCache, i.Message.ID) // 使用完毕后清理缓存
	cacheLock.Unlock()

	if len(tableNames) == 0 {
		utils.SendEphemeralResponse(s, i, "分区选择已过期，请重新操作。")
		return
	}

	// 延迟响应，因为创建/更新导航可能需要一些时间
	if err := deferComponentAck(s, i); err != nil {
		return
	}

	// 调用创建/替换导航的逻辑
	if err := createOrReplaceNavigation(s, i, cfg, slot, tableNames, userID, updateMode); err != nil {
		log.Printf("personal-nav: failed to create navigation slot %d for user %s in guild %s: %v", slot, userID, i.GuildID, err)
		updateComponentMessage(s, i, fmt.Sprintf("❌ 导航更新失败：%s", err.Error()))
		return
	}

	modeDesc := "修改消息"
	if updateMode == updateModeDelete {
		modeDesc = "删除更新"
	}
	updateComponentMessage(s, i, fmt.Sprintf("✅ 导航已更新（更新方式：%s），已发送三条导航消息。", modeDesc))
}

// createOrReplaceNavigation 是一个包装器，调用核心的 updateNavigation 逻辑来创建或替换导航。
func createOrReplaceNavigation(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *model.Config, navID int, tableNames []string, userID, updateMode string) error {
	// i.ChannelID 作为备用频道ID，以防在数据库中找不到现有的导航消息频道。
	return updateNavigation(s, cfg, i.GuildID, i.ChannelID, navID, tableNames, userID, updateMode)
}

// refreshNavigation 是一个包装器，调用核心的 updateNavigation 逻辑来刷新现有导航。
func refreshNavigation(s *discordgo.Session, cfg *model.Config, i *discordgo.InteractionCreate, nav model.PersonalNavigation) error {
	tableNames := strings.Split(nav.TableName, ",")
	// 从数据库记录中读取更新模式，如果为空（旧数据）则默认使用 delete 模式。
	updateMode := nav.UpdateMode
	if updateMode == "" {
		updateMode = updateModeDelete
		log.Printf("personal-nav: nav %d has no UpdateMode, defaulting to delete", nav.NavID)
	}
	// 使用存储在导航记录中的消息频道ID。
	fallbackChannelID := nav.MessageChannelID
	if fallbackChannelID == "" {
		// 作为备用，使用当前交互发生的频道。
		fallbackChannelID = i.ChannelID
	}
	return updateNavigation(s, cfg, nav.GuildID, fallbackChannelID, nav.NavID, tableNames, nav.UserID, updateMode)
}

// extractUserIDFromCustomID 从 customID 的末尾提取用户ID。
func extractUserIDFromCustomID(customID string) (string, error) {
	parts := strings.Split(customID, ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid customID format")
	}
	return parts[len(parts)-1], nil
}

// deferComponentAck 向 Discord 发送一个“延迟更新”的响应。
// 这会移除消息上的“加载中”状态，并允许在稍后通过 InteractionResponseEdit 更新消息。
func deferComponentAck(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		log.Printf("personal-nav: failed to defer interaction: %v", err)
	}
	return err
}

// updateComponentMessage 编辑一个交互的原始响应消息，通常用于在操作完成后显示最终状态（成功或失败）。
// 它会清除所有的 embeds 和 components，只显示一条简单的文本消息。
func updateComponentMessage(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	emptyEmbeds := []*discordgo.MessageEmbed{}
	emptyComponents := []discordgo.MessageComponent{}
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    &content,
		Embeds:     &emptyEmbeds,
		Components: &emptyComponents,
	})
	if err != nil {
		log.Printf("personal-nav: failed to update component message: %v", err)
	}
}
