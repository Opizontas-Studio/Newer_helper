package personalnav

import (
	"fmt"
	"newer_helper/model"
	"newer_helper/utils"
	"sort"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// buildSlotSelectionResponse 构建当导航槽位已满时，让用户选择一个进行覆盖的交互响应。
func buildSlotSelectionResponse(navigations []model.PersonalNavigation, userID string) *discordgo.InteractionResponseData {
	if len(navigations) == 0 {
		return nil
	}

	sort.Slice(navigations, func(i, j int) bool {
		return navigations[i].NavID < navigations[j].NavID
	})

	options := make([]discordgo.SelectMenuOption, 0, len(navigations))
	for _, nav := range navigations {
		label := fmt.Sprintf("导航 %d (ID: %d)", nav.NavID, nav.ID)
		if nav.ChannelName != "" {
			label = fmt.Sprintf("导航 %d (ID: %d) · %s", nav.NavID, nav.ID, nav.ChannelName)
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
		Description: "您最多只能创建 3 个导航。请选择一个现有导航进行覆盖。",
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

// buildNavigationSelectionResponse 构建一个通用的导航选择菜单，用于刷新或删除操作。
func buildNavigationSelectionResponse(navigations []model.PersonalNavigation, title, customID, userID string) *discordgo.InteractionResponseData {
	if len(navigations) == 0 {
		return nil
	}

	sort.Slice(navigations, func(i, j int) bool {
		return navigations[i].NavID < navigations[j].NavID
	})

	options := make([]discordgo.SelectMenuOption, 0, len(navigations))
	for _, nav := range navigations {
		label := fmt.Sprintf("导航 %d (ID: %d)", nav.NavID, nav.ID)
		if nav.ChannelName != "" {
			label = fmt.Sprintf("导航 %d (ID: %d) · %s", nav.NavID, nav.ID, nav.ChannelName)
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
		Description: "请从下方选择一个您创建的导航。",
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

// buildAreaSelectionResponse 构建分区选择菜单，让用户选择要包含在导航中的分区。
func buildAreaSelectionResponse(cfg *model.Config, guildID, userID string, navID int) (*discordgo.InteractionResponseData, error) {
	choices, err := BuildChannelChoices(cfg, guildID, userID)
	if err != nil {
		return nil, err
	}
	if len(choices) == 0 {
		return nil, nil
	}

	options := make([]discordgo.SelectMenuOption, 0, len(choices))
	for _, choice := range choices {
		label := fmt.Sprintf("%s · %d篇", choice.ChannelName, choice.PostCount)
		options = append(options, discordgo.SelectMenuOption{
			Label:       utils.TruncateString(label, 100),
			Value:       choice.TableName,
			Description: utils.TruncateString(fmt.Sprintf("频道: %s", choice.ChannelName), 100),
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("① 选择分区 · 导航槽 %d", navID),
		Description: "请选择一个或多个需要生成导航的分区。\n仅显示您在其中发布过作品的分区。",
		Color:       embedColorPrimary,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "提示：再次选择会覆盖旧导航。",
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
						Placeholder: "选择一个或多个分区 (按住 Ctrl/Cmd 多选)",
						MinValues:   &[]int{1}[0],
						MaxValues:   maxValues,
						Options:     options,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "下一步",
						Style:    discordgo.PrimaryButton,
						CustomID: fmt.Sprintf("%s%d:%s", componentSubmitAreaPrefix, navID, userID),
					},
				},
			},
		},
	}, nil
}

// buildUpdateModeSelectionResponse 构建更新模式选择界面（修改消息 vs 删除更新）。
func buildUpdateModeSelectionResponse(navID int, userID string) *discordgo.InteractionResponseData {
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("② 选择更新方式 · 导航槽 %d", navID),
		Description: "请选择导航的更新方式：\n\n**修改消息 (推荐)**\n刷新时直接编辑现有的导航消息，体验更流畅。\n\n**删除更新**\n刷新时删除旧消息并发送新消息，适用于消息权限复杂的频道。",
		Color:       embedColorPrimary,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "提示：创建后无法修改更新方式",
		},
	}

	return &discordgo.InteractionResponseData{
		Flags:  discordgo.MessageFlagsEphemeral,
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "修改消息 (推荐)",
						Style:    discordgo.PrimaryButton,
						CustomID: fmt.Sprintf("%s%d:%s:%s", componentSubmitUpdateModePrefix, navID, updateModeEdit, userID),
					},
					discordgo.Button{
						Label:    "删除更新",
						Style:    discordgo.SecondaryButton,
						CustomID: fmt.Sprintf("%s%d:%s:%s", componentSubmitUpdateModePrefix, navID, updateModeDelete, userID),
					},
				},
			},
		},
	}
}
