package scanner

import (
	"discord-bot/model"
	"discord-bot/utils"
	punishments_db "discord-bot/utils/database/punishments"
	"encoding/json"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

// StartPunishmentTimer starts a background goroutine to check for and remove expired punishment roles.
func StartPunishmentTimer(s *discordgo.Session) {
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes for better responsiveness
	go func() {
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
		processedRoles := processPunishmentRecord(s, db, punishment, currentTime)

		// If all roles have been processed, mark punishment as completed
		if len(processedRoles) == 0 && punishment.RolesRemoveAt != "{}" && punishment.RolesRemoveAt != "" {
			err := punishments_db.UpdatePunishmentStatus(db, punishment.PunishmentID, "completed")
			if err != nil {
				log.Printf("Failed to update punishment status for ID %d: %v", punishment.PunishmentID, err)
			}
		}
	}
}

// processPunishmentRecord processes a single punishment record and removes expired roles
// Returns the remaining roles that haven't expired yet
func processPunishmentRecord(s *discordgo.Session, db *sqlx.DB, punishment model.PunishmentRecord, currentTime time.Time) map[string]time.Time {
	if punishment.RolesRemoveAt == "{}" || punishment.RolesRemoveAt == "" {
		return nil
	}

	// Parse roles remove times
	var rolesRemoveAt map[string]time.Time
	err := json.Unmarshal([]byte(punishment.RolesRemoveAt), &rolesRemoveAt)
	if err != nil {
		log.Printf("Failed to parse roles_remove_at for punishment ID %d: %v", punishment.PunishmentID, err)
		return nil
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
			return remainingRoles
		}

		err = punishments_db.RemoveExpiredRoleFromPunishment(db, punishment.PunishmentID, "", string(remainingRolesJSON))
		if err != nil {
			log.Printf("Failed to update roles_remove_at for punishment ID %d: %v", punishment.PunishmentID, err)
		}
	}

	return remainingRoles
}