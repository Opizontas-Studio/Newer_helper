package model

// NewCardPushConfig defines the configuration for the new card push feature.
type NewCardPushConfig struct {
	PushChannelIDs      []string            `json:"push_channel_ids"`
	WhitelistedMessages map[string][]string `json:"whitelisted_messages,omitempty"`
}
