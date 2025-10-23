package personalnav

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"newer_helper/bot"
	"newer_helper/model"
	"newer_helper/utils"
	"newer_helper/utils/database"

	"github.com/bwmarrin/discordgo"
)

const (
	componentSelectSlot           = "personal_nav:slot"
	componentSelectAreaPrefix     = "personal_nav:area:"
	componentSubmitAreaPrefix     = "personal_nav:submit_area:"
	componentRefreshSelect        = "personal_nav:refresh"
	componentDeleteSelect         = "personal_nav:delete"
	maxNavigationSlots            = 3
	maxLatestPostsToDisplay       = 10
	maxMyWorksToDisplay           = 999 // 实际移除限制，由自动分页处理
	embedColorPrimary         int = 0x5865F2
	embedColorHighlight       int = 0xFEE75C
	embedColorSecondary       int = 0x57F287
)

type channelChoice struct {
	TableName   string
	ChannelID   string
	ChannelName string
	PostCount   int
}

// HandlePersonalNavCommand is the entry point for /personal-nav.
func HandlePersonalNavCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot) {
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

	if userIDOpt, ok := optionMap["userid"]; ok {
		specifiedUserID := strings.TrimSpace(userIDOpt.StringValue())
		if specifiedUserID != "" {
			isDev := utils.CheckPermission(nil, requesterID, nil, nil, b.GetConfig().DeveloperUserIDs, nil) == utils.DeveloperPermission
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

	switch action {
	case "create":
		handleCreateAction(s, i, b, targetUserID)
	case "refresh":
		handleRefreshAction(s, i, b, input, targetUserID)
	case "delete":
		handleDeleteAction(s, i, b, input, targetUserID)
	default:
		utils.SendEphemeralResponse(s, i, "未知的操作类型。")
	}
}

func handleCreateAction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, userID string) {
	guildID := i.GuildID
	log.Printf("personal-nav: handleCreateAction guild=%s user=%s", guildID, userID)

	log.Printf("personal-nav: loading existing navigation entries...")
	navigations, err := database.GetPersonalNavigations(userID, guildID)
	if err != nil {
		log.Printf("Failed to load personal navigations for %s/%s: %v", guildID, userID, err)
		utils.SendEphemeralResponse(s, i, "读取导航数据时出错。")
		return
	}
	log.Printf("personal-nav: existing navigations count=%d", len(navigations))

	slot := findFirstAvailableSlot(navigations)
	if slot == 0 {
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

	data, err := buildAreaSelectionResponse(b, guildID, userID, slot)
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

func handleRefreshAction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, messageID, userID string) {
	guildID := i.GuildID

	// Direct refresh via message id.
	if messageID != "" {
		nav, err := database.GetPersonalNavigationByMessageID(guildID, messageID)
		if err != nil {
			log.Printf("Failed to locate navigation by message id %s: %v", messageID, err)
			utils.SendEphemeralResponse(s, i, "查询导航记录时出错。")
			return
		}
		if nav == nil || nav.UserID != userID {
			utils.SendEphemeralResponse(s, i, "找不到与该消息关联的导航，或您没有权限。")
			return
		}
		if err := refreshNavigation(s, b, i, *nav); err != nil {
			utils.SendEphemeralResponse(s, i, err.Error())
			return
		}
		utils.SendEphemeralResponse(s, i, "导航已刷新。")
		return
	}

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

func handleDeleteAction(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, messageID, userID string) {
	guildID := i.GuildID

	if messageID != "" {
		nav, err := database.GetPersonalNavigationByMessageID(guildID, messageID)
		if err != nil {
			log.Printf("Failed to locate navigation by message id %s: %v", messageID, err)
			utils.SendEphemeralResponse(s, i, "查询导航记录时出错。")
			return
		}
		if nav == nil || nav.UserID != userID {
			utils.SendEphemeralResponse(s, i, "找不到与该消息关联的导航，或您没有权限。")
			return
		}
		if err := deleteNavigation(s, *nav); err != nil {
			utils.SendEphemeralResponse(s, i, err.Error())
			return
		}
		utils.SendEphemeralResponse(s, i, "导航已删除。")
		return
	}

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

func buildSlotSelectionResponse(navigations []model.PersonalNavigation, userID string) *discordgo.InteractionResponseData {
	if len(navigations) == 0 {
		return nil
	}

	sort.Slice(navigations, func(i, j int) bool {
		return navigations[i].NavID < navigations[j].NavID
	})

	options := make([]discordgo.SelectMenuOption, 0, len(navigations))
	for _, nav := range navigations {
		label := fmt.Sprintf("导航 %d", nav.NavID)
		if nav.ChannelName != "" {
			label = fmt.Sprintf("导航 %d · %s", nav.NavID, nav.ChannelName)
		}
		desc := fmt.Sprintf("位于 <#%s>", nav.MessageChannelID)
		if nav.MessageChannelID == "" {
			desc = "频道未知"
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:       utils.TruncateString(label, 100),
			Value:       strconv.Itoa(nav.NavID),
			Description: utils.TruncateString(desc, 100),
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "导航上限已满",
		Description: "请选择一个导航槽位进行覆盖。",
		Color:       embedColorHighlight,
	}

	return &discordgo.InteractionResponseData{
		Flags:  discordgo.MessageFlagsEphemeral,
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    fmt.Sprintf("%s:%s", componentSelectSlot, userID),
						Placeholder: "选择需要覆盖的导航",
						MinValues:   &[]int{1}[0],
						MaxValues:   1,
						Options:     options,
					},
				},
			},
		},
	}
}

func buildNavigationSelectionResponse(navigations []model.PersonalNavigation, title, customID, userID string) *discordgo.InteractionResponseData {
	if len(navigations) == 0 {
		return nil
	}

	sort.Slice(navigations, func(i, j int) bool {
		return navigations[i].NavID < navigations[j].NavID
	})

	options := make([]discordgo.SelectMenuOption, 0, len(navigations))
	for _, nav := range navigations {
		label := fmt.Sprintf("导航 %d", nav.NavID)
		if nav.ChannelName != "" {
			label = fmt.Sprintf("导航 %d · %s", nav.NavID, nav.ChannelName)
		}
		desc := fmt.Sprintf("<#%s>", nav.MessageChannelID)
		if nav.MessageChannelID == "" {
			desc = "创建位置未知"
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:       utils.TruncateString(label, 100),
			Value:       strconv.Itoa(nav.NavID),
			Description: utils.TruncateString(desc, 100),
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: "请选择一个导航槽位。",
		Color:       embedColorPrimary,
	}

	return &discordgo.InteractionResponseData{
		Flags:  discordgo.MessageFlagsEphemeral,
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    fmt.Sprintf("%s:%s", customID, userID),
						Placeholder: "选择一个导航",
						MinValues:   &[]int{1}[0],
						MaxValues:   1,
						Options:     options,
					},
				},
			},
		},
	}
}

func buildAreaSelectionResponse(b *bot.Bot, guildID, userID string, navID int) (*discordgo.InteractionResponseData, error) {
	choices, err := buildChannelChoices(b, guildID, userID)
	if err != nil {
		return nil, err
	}
	if len(choices) == 0 {
		log.Printf("personal-nav: user %s has no channel choices in guild %s", userID, guildID)
		return nil, nil
	}

	log.Printf("personal-nav: user %s has %d channel choices in guild %s", userID, len(choices), guildID)

	options := make([]discordgo.SelectMenuOption, 0, len(choices))
	for _, choice := range choices {
		label := fmt.Sprintf("%s · %d", choice.ChannelName, choice.PostCount)
		options = append(options, discordgo.SelectMenuOption{
			Label:       utils.TruncateString(label, 100),
			Value:       choice.TableName,
			Description: utils.TruncateString(fmt.Sprintf("频道: %s", choice.ChannelName), 100),
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("选择渲染区域 · 导航槽 %d", navID),
		Description: "仅显示您在其中发布过作品的分区，请选择一个或多个需要生成导航的分区。",
		Color:       embedColorPrimary,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "提示：最多同时保存 3 个导航，再次选择会覆盖旧导航。",
		},
	}

	maxValues := len(options)
	if maxValues > 25 {
		maxValues = 25
	}

	return &discordgo.InteractionResponseData{
		Flags:  discordgo.MessageFlagsEphemeral,
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    fmt.Sprintf("%s%d:%s", componentSelectAreaPrefix, navID, userID),
						Placeholder: "选择一个或多个分区",
						MinValues:   &[]int{1}[0],
						MaxValues:   maxValues,
						Options:     options,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "提交选择",
						Style:    discordgo.PrimaryButton,
						CustomID: fmt.Sprintf("%s%d:%s", componentSubmitAreaPrefix, navID, userID),
					},
				},
			},
		},
	}, nil
}

func buildChannelChoices(b *bot.Bot, guildID, userID string) ([]channelChoice, error) {
	cfg := b.GetConfig()
	guildTask, ok := cfg.TaskConfig[guildID]
	if !ok {
		return nil, fmt.Errorf("未配置该服务器的分区信息。")
	}

	dbPath := fmt.Sprintf("data/%s.db", guildID)
	db, err := database.InitDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败。")
	}
	defer db.Close()

	log.Printf("personal-nav: scanning channel choices for user %s in guild %s", userID, guildID)

	var choices []channelChoice
	for name, channelTask := range guildTask.Data {
		if len(channelTask.ChannelID) < 4 {
			continue
		}
		tableName := fmt.Sprintf("%s_%s", name, channelTask.ChannelID[len(channelTask.ChannelID)-4:])
		count, err := database.CountPostsByAuthorInTable(db, tableName, userID)
		if err != nil {
			log.Printf("Failed to count posts for user %s table %s: %v", userID, tableName, err)
			continue
		}
		if count == 0 {
			log.Printf("personal-nav: skip table %s for user %s (no posts)", tableName, userID)
			continue
		}
		choices = append(choices, channelChoice{
			TableName:   tableName,
			ChannelID:   channelTask.ChannelID,
			ChannelName: name,
			PostCount:   count,
		})
	}

	log.Printf("personal-nav: total channel choices gathered=%d", len(choices))

	sort.Slice(choices, func(i, j int) bool {
		return choices[i].ChannelName < choices[j].ChannelName
	})

	return choices, nil
}

func validateUsageContext(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	userID := i.Member.User.ID
	channel, err := s.State.Channel(i.ChannelID)
	if err != nil {
		channel, err = s.Channel(i.ChannelID)
		if err != nil {
			log.Printf("Failed to fetch channel %s for personal nav command: %v", i.ChannelID, err)
			return false
		}
	}

	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err == nil && perms&discordgo.PermissionManageChannels != 0 {
		return true
	}

	switch channel.Type {
	case discordgo.ChannelTypeGuildPublicThread,
		discordgo.ChannelTypeGuildPrivateThread,
		discordgo.ChannelTypeGuildNewsThread:
		if channel.OwnerID == userID {
			return true
		}
		if channel.ParentID != "" {
			perms, err = s.UserChannelPermissions(userID, channel.ParentID)
			if err == nil && perms&discordgo.PermissionManageChannels != 0 {
				return true
			}
		}
	default:
		// Already checked direct manage permission.
	}
	return false
}
