package utils

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

// SendPrivateMessage sends a direct message to a user.
func SendPrivateMessage(s *discordgo.Session, userID, message string) {
	channel, err := s.UserChannelCreate(userID)
	if err != nil {
		log.Printf("Error creating private channel with user %s: %v", userID, err)
		return
	}
	_, err = s.ChannelMessageSend(channel.ID, message)
	if err != nil {
		log.Printf("Error sending private message to user %s: %v", userID, err)
	}
}

// SendPrivateEmbedMessage sends a direct message with an embed to a user.
func SendPrivateEmbedMessage(s *discordgo.Session, userID string, embed *discordgo.MessageEmbed) {
	channel, err := s.UserChannelCreate(userID)
	if err != nil {
		log.Printf("Error creating private channel with user %s: %v", userID, err)
		return
	}
	_, err = s.ChannelMessageSendEmbed(channel.ID, embed)
	if err != nil {
		log.Printf("Error sending private embed message to user %s: %v", userID, err)
	}
}
