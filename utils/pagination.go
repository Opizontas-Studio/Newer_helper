package utils

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// CreatePaginationComponents creates a set of pagination buttons.
func CreatePaginationComponents(currentPage, totalPages int, customIDPrefix string, args ...string) []discordgo.MessageComponent {
	if totalPages <= 1 {
		return nil
	}

	buttonArgs := ""
	for _, arg := range args {
		buttonArgs += ":" + arg
	}

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "上一页",
					Style:    discordgo.PrimaryButton,
					Disabled: currentPage == 1,
					CustomID: fmt.Sprintf("%s:%d%s", customIDPrefix, currentPage-1, buttonArgs),
				},
				discordgo.Button{
					Label:    "下一页",
					Style:    discordgo.PrimaryButton,
					Disabled: currentPage == totalPages,
					CustomID: fmt.Sprintf("%s:%d%s", customIDPrefix, currentPage+1, buttonArgs),
				},
			},
		},
	}
}
