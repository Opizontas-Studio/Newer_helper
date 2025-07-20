package utils

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

// SendErrorResponse sends an ephemeral error message.
func SendErrorResponse(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Error sending error response: %v", err)
	}
}
func SendPublicResponse(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})
	if err != nil {
		log.Printf("Error sending public response: %v", err)
	}
}

// SendSimpleResponse sends a simple ephemeral message.
func SendSimpleResponse(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Error sending simple response: %v", err)
	}
}

// SendFollowUp sends a follow-up message to an interaction.
func SendFollowUp(s *discordgo.Session, i *discordgo.Interaction, message string) {
	_, err := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Content: &message,
	})
	if err != nil {
		log.Printf("Error sending follow-up message: %v", err)
	}
}

// SendFollowUpError sends a follow-up error message to an interaction.
func SendFollowUpError(s *discordgo.Session, i *discordgo.Interaction, message string) {
	errorMsg := "❌ " + message
	_, err := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Content: &errorMsg,
	})
	if err != nil {
		log.Printf("Error sending follow-up error message: %v", err)
	}
}

// EditErrorResponse edits an interaction to show an error message.
func EditErrorResponse(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	errorMsg := "❌ " + message
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &errorMsg,
	})
	if err != nil {
		log.Printf("Error sending edit error response: %v", err)
	}
}
