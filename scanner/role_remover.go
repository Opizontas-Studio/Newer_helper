package scanner

import (
	"discord-bot/utils/database"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
)

// StartRoleRemover starts a background goroutine to check for and remove expired roles.
func StartRoleRemover(s *discordgo.Session, db *sqlx.DB) {
	ticker := time.NewTicker(3 * time.Hour)
	go func() {
		for range ticker.C {
			tasks, err := database.GetDueTasks(db)
			if err != nil {
				log.Printf("Error getting due tasks: %v", err)
				continue
			}

			for _, task := range tasks {
				err := s.GuildMemberRoleRemove(task.GuildID, task.UserID, task.RoleID)
				if err != nil {
					log.Printf("Failed to remove role %s from user %s: %v", task.RoleID, task.UserID, err)
					// Optionally, handle specific errors (e.g., user not found, role not found)
				} else {
					log.Printf("Successfully removed role %s from user %s", task.RoleID, task.UserID)
					err := database.DeleteTask(db, task.ID)
					if err != nil {
						log.Printf("Failed to delete task %d: %v", task.ID, err)
					}
				}
			}
		}
	}()
}
