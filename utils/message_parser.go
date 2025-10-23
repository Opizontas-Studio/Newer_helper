package utils

import (
	"fmt"
	"regexp"

	"github.com/bwmarrin/discordgo"
)

// ParseMessageLinks parses Discord message links from a string and returns the message objects.
func ParseMessageLinks(s *discordgo.Session, messageLinks string) ([]*discordgo.Message, error) {
	re := regexp.MustCompile(`https://discord.com/channels/(\d+)/(\d+)/(\d+)`)
	matches := re.FindAllStringSubmatch(messageLinks, -1)

	var messages []*discordgo.Message
	for _, match := range matches {
		if len(match) == 4 {
			channelID := match[2]
			messageID := match[3]
			msg, err := s.ChannelMessage(channelID, messageID)
			if err != nil {
				return nil, fmt.Errorf("error fetching message %s: %w", match[0], err)
			}
			messages = append(messages, msg)
		}
	}
	return messages, nil
}

// ParseMessageLink parses a single Discord message link and returns the guild ID, channel ID, and message ID.
func ParseMessageLink(link string) (guildID, channelID, messageID string, err error) {
	re := regexp.MustCompile(`https://discord.com/channels/(\d+)/(\d+)/(\d+)`)
	matches := re.FindStringSubmatch(link)

	if len(matches) != 4 {
		return "", "", "", fmt.Errorf("invalid message link format")
	}

	return matches[1], matches[2], matches[3], nil
}

// ReplaceMacro replaces a macro placeholder with a value in a string.
func ReplaceMacro(text, macro, value string) string {
	return regexp.MustCompile(regexp.QuoteMeta(macro)).ReplaceAllString(text, value)
}

// TruncateString shortens a string to a maximum length, handling runes correctly.
func TruncateString(input string, maxLen int) string {
	if len([]rune(input)) <= maxLen {
		return input
	}
	runes := []rune(input)
	return string(runes[:maxLen])
}
