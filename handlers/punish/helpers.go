package punish

import (
	"discord-bot/internal/repository"
	"discord-bot/model"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// applyTimeoutIfRequired handles the logic for applying timeouts to users.
// It checks the user's recent punishment history and applies a timeout if the frequency exceeds the configured threshold.
// It also creates a timed task to add a role if configured.
func applyTimeoutIfRequired(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	punishmentRepo repository.PunishmentRepository,
	timedTaskRepo repository.TimedTaskRepository,
	config model.KickConfigEntry,
	targetUser *discordgo.User,
) (bool, string, error) {
	// If frequency is not set, or time is not set, timeout logic is disabled.
	if config.Timeout.Frequency <= 0 || config.Timeout.Time == "" {
		return false, "", nil
	}

	// Check history within the configured time window.
	history, err := punishmentRepo.GetByUserID(targetUser.ID)
	if err != nil {
		return false, "", fmt.Errorf("failed to get punishment history: %w", err)
	}

	// If history count is less than frequency, do not apply timeout.
	if len(history) < config.Timeout.Frequency {
		return false, "", nil
	}

	// Parse the timeout duration from config.
	duration, err := time.ParseDuration(config.Timeout.TimeoutTime)
	if err != nil {
		return false, "", fmt.Errorf("invalid timeout_time duration format: %w", err)
	}
	until := time.Now().Add(duration)

	// Apply the timeout to the guild member.
	err = s.GuildMemberTimeout(i.GuildID, targetUser.ID, &until)
	if err != nil {
		return false, "", fmt.Errorf("failed to apply timeout: %w", err)
	}

	timeoutDurationStr := formatDuration(duration)

	// If configured and available, create a timed task to add a role after a certain duration.
	if timedTaskRepo != nil && len(config.Timeout.AddRole) > 0 && config.Timeout.AddRoleTimeoutTime != "" {
		addRoleDuration, err := time.ParseDuration(config.Timeout.AddRoleTimeoutTime)
		if err != nil {
			log.Printf("Invalid add_role_timeout_time format: %v", err)
		} else {
			task := &model.TimedTask{
				GuildID:  i.GuildID,
				UserID:   targetUser.ID,
				RoleID:   config.Timeout.AddRole[0],
				RemoveAt: time.Now().Add(addRoleDuration),
			}
			if err := timedTaskRepo.Create(task); err != nil {
				log.Printf("Failed to create timed task for adding role: %v", err)
			}
		}
	}

	return true, timeoutDurationStr, nil
}

// formatDuration formats a time.Duration into a human-readable string (e.g., "1天2小时3分钟").
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d天", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d小时", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d分钟", minutes))
	}
	if len(parts) == 0 {
		return "0分钟"
	}
	return strings.Join(parts, "")
}

// isUserWhitelisted checks if a member has a role present in the whitelist.
func isUserWhitelisted(member *discordgo.Member, config model.KickConfigEntry) bool {
	for _, whitelistedRole := range config.WhitelistRoleID {
		for _, userRole := range member.Roles {
			if userRole == whitelistedRole {
				return true
			}
		}
	}
	return false
}

// removePunishmentRoles removes a list of specified roles from a guild member.
func removePunishmentRoles(s *discordgo.Session, guildID, userID string, rolesToRemove []string) {
	for _, roleID := range rolesToRemove {
		err := s.GuildMemberRoleRemove(guildID, userID, roleID)
		if err != nil {
			log.Printf("Failed to remove role %s from user %s: %v", roleID, userID, err)
		}
	}
}
