package personalnav

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"newer_helper/bot"
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

// HandlePersonalNavComponent routes select-menu interactions for personal navigation workflows.
func HandlePersonalNavComponent(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, customID string) bool {
	parts := strings.Split(customID, ":")
	baseID := parts[0]
	if len(parts) > 1 && strings.HasPrefix(customID, "personal_nav") {
		baseID = strings.Join(parts[:len(parts)-1], ":")
	}

	switch {
	case baseID == componentSelectSlot:
		handleSlotSelectionComponent(s, i, b, customID)
	case strings.HasPrefix(baseID, componentSelectAreaPrefix):
		handleAreaSelectionMenuComponent(s, i, b, customID)
	case strings.HasPrefix(baseID, componentSubmitAreaPrefix):
		handleAreaSubmitComponent(s, i, b, customID)
	case baseID == componentRefreshSelect:
		handleRefreshSelectionComponent(s, i, b, customID)
	case baseID == componentDeleteSelect:
		handleDeleteSelectionComponent(s, i, b, customID)
	default:
		return false
	}
	return true
}

func handleSlotSelectionComponent(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, customID string) {
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

	data, err := buildAreaSelectionResponse(b, i.GuildID, userID, slot)
	if err != nil {
		utils.SendEphemeralResponse(s, i, err.Error())
		return
	}
	if data == nil {
		utils.SendEphemeralResponse(s, i, "暂未在任何子区找到属于您的作品。")
		return
	}

	// Update the original message with the area selection menu.
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: data,
	})
	if err != nil {
		log.Printf("Failed to update slot selection with area options: %v", err)
	}
}

func handleAreaSelectionMenuComponent(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, customID string) {
	// Store the current selection in the cache, keyed by the message ID.
	cacheLock.Lock()
	selectionCache[i.Message.ID] = i.MessageComponentData().Values
	cacheLock.Unlock()

	// Acknowledge the interaction, but don't respond yet.
	// The user will click the submit button to confirm their selection.
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		log.Printf("Failed to acknowledge area selection: %v", err)
	}
}

func handleAreaSubmitComponent(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, customID string) {
	userID, err := extractUserIDFromCustomID(customID)
	if err != nil {
		utils.SendEphemeralResponse(s, i, "无效的组件ID。")
		return
	}

	// Retrieve the select menu values from the cache.
	cacheLock.Lock()
	tableNames := selectionCache[i.Message.ID]
	delete(selectionCache, i.Message.ID) // Clean up after retrieval.
	cacheLock.Unlock()

	if len(tableNames) == 0 {
		utils.SendEphemeralResponse(s, i, "您必须选择一个分区。")
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

	if err := deferComponentAck(s, i); err != nil {
		return
	}

	if err := createOrReplaceNavigation(s, i, b, slot, tableNames, userID); err != nil {
		log.Printf("personal-nav: failed to create navigation slot %d for user %s in guild %s: %v", slot, userID, i.GuildID, err)
		updateComponentMessage(s, i, fmt.Sprintf("❌ 导航更新失败：%s", err.Error()))
		return
	}

	updateComponentMessage(s, i, "✅ 导航已更新，已发送三条导航消息。")
}

func handleRefreshSelectionComponent(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, customID string) {
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

	if err := deferComponentAck(s, i); err != nil {
		return
	}

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

	if err := refreshNavigation(s, b, i, *nav); err != nil {
		log.Printf("personal-nav: failed to refresh navigation %d for user %s: %v", navID, userID, err)
		updateComponentMessage(s, i, fmt.Sprintf("❌ 导航刷新失败：%s", err.Error()))
		return
	}

	updateComponentMessage(s, i, "✅ 导航已刷新。")
}

func handleDeleteSelectionComponent(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, customID string) {
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

	if err := deferComponentAck(s, i); err != nil {
		return
	}

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

	if err := deleteNavigation(s, *nav); err != nil {
		updateComponentMessage(s, i, fmt.Sprintf("❌ 导航删除失败：%s", err.Error()))
		return
	}

	updateComponentMessage(s, i, "✅ 导航已删除。")
}

func createOrReplaceNavigation(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, navID int, tableNames []string, userID string) error {
	return updateNavigation(s, b, i, navID, tableNames, userID)
}

func refreshNavigation(s *discordgo.Session, b *bot.Bot, i *discordgo.InteractionCreate, nav model.PersonalNavigation) error {
	tableNames := strings.Split(nav.TableName, ",")
	return updateNavigation(s, b, i, nav.NavID, tableNames, nav.UserID)
}

func deleteNavigation(s *discordgo.Session, nav model.PersonalNavigation) error {
	log.Printf("personal-nav: delete navigation start guild=%s user=%s nav=%d", nav.GuildID, nav.UserID, nav.NavID)
	channelID := nav.MessageChannelID
	if channelID == "" {
		channelID = nav.ChannelID
	}

	// 拆分并删除所有"我的作品"消息
	var myWorksIDs []string
	if nav.MessageIDMyWorks != "" {
		myWorksIDs = strings.Split(nav.MessageIDMyWorks, ",")
	}

	var allIDs []string
	for _, id := range myWorksIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			allIDs = append(allIDs, id)
		}
	}
	allIDs = append(allIDs, nav.MessageIDTopWorks, nav.MessageIDLatestWorks)

	for _, id := range allIDs {
		if id == "" {
			continue
		}
		if err := s.ChannelMessageDelete(channelID, id); err != nil {
			// Tolerate not found errors.
			if restErr, ok := err.(*discordgo.RESTError); ok && restErr.Response != nil && restErr.Response.StatusCode == 404 {
				continue
			}
			log.Printf("Failed to delete navigation message %s: %v", id, err)
		} else {
			log.Printf("personal-nav: deleted message %s in channel %s", id, channelID)
		}
	}

	if err := database.DeletePersonalNavigation(nav.UserID, nav.GuildID, nav.NavID); err != nil {
		return fmt.Errorf("删除导航记录失败。")
	}
	log.Printf("personal-nav: delete navigation finished for nav=%d", nav.NavID)
	return nil
}

func extractUserIDFromCustomID(customID string) (string, error) {
	parts := strings.Split(customID, ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid customID format")
	}
	return parts[len(parts)-1], nil
}

func deferComponentAck(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		log.Printf("personal-nav: failed to defer interaction: %v", err)
	}
	return err
}

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
