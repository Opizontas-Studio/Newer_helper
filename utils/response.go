package utils

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

// SendPublicResponse sends a public response to an interaction.
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

// SendEphemeralResponse sends an ephemeral message.
func SendEphemeralResponse(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Error sending ephemeral response: %v", err)
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

// DeferResponse defers an interaction response, optionally making it ephemeral.
func DeferResponse(s *discordgo.Session, i *discordgo.InteractionCreate, ephemeral bool) error {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}
	if ephemeral {
		response.Data = &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		}
	}
	return s.InteractionRespond(i.Interaction, response)
}
