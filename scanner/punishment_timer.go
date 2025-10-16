package scanner

import (
	"encoding/json"
	"fmt"
	"log"
	"newer_helper/model"
	"newer_helper/utils"
	punishments_db "newer_helper/utils/database/punishments"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

// StartPunishmentTimer starts a background goroutine to check for and remove expired punishment roles.
func StartPunishmentTimer(s *discordgo.Session) {
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes for better responsiveness
	go func() {
		// Run once right away so we don't wait for the first ticker tick after restart
		processPunishmentTimers(s)
		for range ticker.C {
			processPunishmentTimers(s)
		}
	}()
}

// processPunishmentTimers processes all active punishment records with temporary roles
func processPunishmentTimers(s *discordgo.Session) {
	// Load punishment config to get database path
	punishConfig, err := utils.LoadPunishConfig("config/config_file/punish_config.json")
	if err != nil {
		log.Printf("Error loading punish config for timer: %v", err)
		return
	}

	// Connect to database
	db, err := punishments_db.Init(punishConfig.DatabasePath)
	if err != nil {
		log.Printf("Error connecting to punishment DB for timer: %v", err)
		return
	}
	defer db.Close()

	// Get all active punishments
	punishments, err := punishments_db.GetActivePunishments(db)
	if err != nil {
		log.Printf("Error getting active punishments: %v", err)
		return
	}

	currentTime := time.Now()

	for _, punishment := range punishments {
		processedRoles, err := processPunishmentRecord(s, db, punishment, currentTime, punishConfig.PunishConfig)
		if err != nil {
			log.Printf("Failed to process punishment ID %d: %v", punishment.PunishmentID, err)
			continue
		}

		// If all roles have been processed, mark punishment as completed
		if processedRoles != nil && len(processedRoles) == 0 && punishment.RolesRemoveAt != "{}" && punishment.RolesRemoveAt != "" {
			err := punishments_db.UpdatePunishmentStatus(db, punishment.PunishmentID, "completed")
			if err != nil {
				log.Printf("Failed to update punishment status for ID %d: %v", punishment.PunishmentID, err)
			}
		}
	}
}

// processPunishmentRecord processes a single punishment record and removes expired roles.
// Returns the remaining roles that haven't expired yet.
func processPunishmentRecord(
	s *discordgo.Session,
	db *sqlx.DB,
	punishment model.PunishmentRecord,
	currentTime time.Time,
	config map[string]map[string]model.ActionConfig,
) (map[string]time.Time, error) {
	var rolesRemoveAt map[string]time.Time
	var err error

	if punishment.RolesRemoveAt == "{}" || punishment.RolesRemoveAt == "" {
		rolesRemoveAt, err = rebuildRolesRemoveAt(punishment, config)
		if err != nil {
			return nil, fmt.Errorf("failed to rebuild missing roles_remove_at: %w", err)
		}

		if len(rolesRemoveAt) == 0 {
			return nil, nil
		}

		if err := persistRolesRemoveAt(db, punishment.PunishmentID, rolesRemoveAt); err != nil {
			return nil, fmt.Errorf("failed to persist rebuilt roles_remove_at: %w", err)
		}

		log.Printf("Reconstructed roles_remove_at for punishment ID %d", punishment.PunishmentID)
	} else {
		err = json.Unmarshal([]byte(punishment.RolesRemoveAt), &rolesRemoveAt)
		if err != nil {
			log.Printf("Failed to parse roles_remove_at for punishment ID %d: %v", punishment.PunishmentID, err)

			rolesRemoveAt, rebuildErr := rebuildRolesRemoveAt(punishment, config)
			if rebuildErr != nil {
				return nil, fmt.Errorf("failed to rebuild invalid roles_remove_at: %w", rebuildErr)
			}

			if len(rolesRemoveAt) == 0 {
				return nil, nil
			}

			if err := persistRolesRemoveAt(db, punishment.PunishmentID, rolesRemoveAt); err != nil {
				return nil, fmt.Errorf("failed to persist rebuilt roles_remove_at: %w", err)
			}

			log.Printf("Reconstructed corrupt roles_remove_at for punishment ID %d", punishment.PunishmentID)
		}
	}

	remainingRoles := make(map[string]time.Time)
	rolesRemoved := false

	for roleID, removeTime := range rolesRemoveAt {
		if currentTime.After(removeTime) {
			// Role has expired, remove it
			err := s.GuildMemberRoleRemove(punishment.GuildID, punishment.UserID, roleID)
			if err != nil {
				log.Printf("Failed to remove role %s from user %s: %v", roleID, punishment.UserID, err)
				// Keep the role in the list to retry later
				remainingRoles[roleID] = removeTime
			} else {
				log.Printf("Successfully removed expired role %s from user %s (punishment ID: %d)",
					roleID, punishment.UserID, punishment.PunishmentID)
				rolesRemoved = true
			}
		} else {
			// Role hasn't expired yet, keep it
			remainingRoles[roleID] = removeTime
		}
	}

	// Update the database if any roles were removed
	if rolesRemoved {
		remainingRolesJSON, err := json.Marshal(remainingRoles)
		if err != nil {
			log.Printf("Failed to serialize remaining roles for punishment ID %d: %v", punishment.PunishmentID, err)
			return remainingRoles, err
		}

		err = punishments_db.RemoveExpiredRoleFromPunishment(db, punishment.PunishmentID, "", string(remainingRolesJSON))
		if err != nil {
			log.Printf("Failed to update roles_remove_at for punishment ID %d: %v", punishment.PunishmentID, err)
			return remainingRoles, err
		}
	}

	return remainingRoles, nil
}

// rebuildRolesRemoveAt attempts to reconstruct missing role removal times based on the punishment config.
func rebuildRolesRemoveAt(punishment model.PunishmentRecord, config map[string]map[string]model.ActionConfig) (map[string]time.Time, error) {
	if punishment.TempRolesJSON == "" || punishment.TempRolesJSON == "[]" {
		return nil, nil
	}

	var tempRoles []string
	if err := json.Unmarshal([]byte(punishment.TempRolesJSON), &tempRoles); err != nil {
		return nil, fmt.Errorf("failed to parse temp_roles_json: %w", err)
	}

	roleSet := make(map[string]struct{})
	for _, role := range tempRoles {
		if role == "0" || role == "" {
			continue
		}
		roleSet[role] = struct{}{}
	}

	if len(roleSet) == 0 {
		return nil, nil
	}

	guildConfig, ok := config[punishment.GuildID]
	if !ok {
		return nil, nil
	}

	actionConfig, ok := guildConfig[punishment.ActionType]
	if !ok {
		return nil, nil
	}

	timeoutDays, ok := matchTimeoutDays(roleSet, actionConfig)
	if !ok || timeoutDays <= 0 {
		return nil, nil
	}

	removeAt := time.Unix(punishment.Timestamp, 0).Add(time.Duration(timeoutDays) * 24 * time.Hour)

	rebuilt := make(map[string]time.Time, len(roleSet))
	for role := range roleSet {
		rebuilt[role] = removeAt
	}

	return rebuilt, nil
}

func matchTimeoutDays(roleSet map[string]struct{}, actionConfig model.ActionConfig) (int, bool) {
	var fallbackDays int
	fallbackSet := false
	fallbackAmbiguous := false

	for _, level := range actionConfig.Data {
		levelRoles := make(map[string]struct{})
		for _, role := range level.AddRole {
			if role == "0" || role == "" {
				continue
			}
			levelRoles[role] = struct{}{}
		}

		if days, ok := parsePositiveInt(level.AddRoleTimeoutTime); ok {
			if !fallbackSet {
				fallbackDays = days
				fallbackSet = true
			} else if fallbackDays != days {
				fallbackAmbiguous = true
			}
		}

		if len(levelRoles) == 0 || len(levelRoles) != len(roleSet) {
			continue
		}

		match := true
		for role := range levelRoles {
			if _, ok := roleSet[role]; !ok {
				match = false
				break
			}
		}

		if !match {
			continue
		}

		if days, ok := parsePositiveInt(level.AddRoleTimeoutTime); ok && days > 0 {
			return days, true
		}
	}

	if fallbackSet && !fallbackAmbiguous && fallbackDays > 0 {
		return fallbackDays, true
	}

	return 0, false
}

func parsePositiveInt(value string) (int, bool) {
	days, err := strconv.Atoi(value)
	if err != nil || days <= 0 {
		return 0, false
	}
	return days, true
}

func persistRolesRemoveAt(db *sqlx.DB, punishmentID int64, roles map[string]time.Time) error {
	jsonData, err := json.Marshal(roles)
	if err != nil {
		return fmt.Errorf("failed to serialize rebuilt roles_remove_at: %w", err)
	}

	if err := punishments_db.RemoveExpiredRoleFromPunishment(db, punishmentID, "", string(jsonData)); err != nil {
		return fmt.Errorf("failed to update roles_remove_at: %w", err)
	}

	return nil
}
