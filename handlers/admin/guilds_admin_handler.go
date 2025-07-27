package admin

import (
	"database/sql"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func HandleGuildsAdminCommand(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB, cfg *model.Config) {
	permissionLevel := utils.CheckPermission(i.Member.Roles, i.Member.User.ID, nil, nil, cfg.DeveloperUserIDs, cfg.SuperAdminRoleIDs)
	if permissionLevel < utils.DeveloperPermission {
		utils.SendEphemeralResponse(s, i, "You do not have permission to use this command.")
		return
	}

	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
	for _, opt := range i.ApplicationCommandData().Options {
		optionMap[opt.Name] = opt
	}

	action, ok := optionMap["action"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: action option is missing.")
		return
	}

	actionStr := action.StringValue()

	switch actionStr {
	case "activate", "deactivate":
		handleToggleGuild(s, i, db, optionMap, actionStr == "activate")
	case "add_guild":
		handleAddGuild(s, i, db, optionMap)
	case "add_admin", "remove_admin":
		handleRole(s, i, db, optionMap, "admin", actionStr == "add_admin")
	case "add_user", "remove_user":
		handleRole(s, i, db, optionMap, "user", actionStr == "add_user")
	case "list_config":
		handleListGuildConfig(s, i, db, optionMap)
	default:
		utils.SendEphemeralResponse(s, i, "Unknown action.")
	}
}

func handleToggleGuild(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB, options map[string]*discordgo.ApplicationCommandInteractionDataOption, enable bool) {
	guildOpt, ok := options["guild"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: guild option is missing.")
		return
	}
	guildID := guildOpt.StringValue()

	config, err := database.GetGuildConfig(db, guildID)
	if err != nil {
		log.Printf("Error getting guild config for guild %s: %v", guildID, err)
		utils.SendEphemeralResponse(s, i, "An error occurred while fetching the configuration.")
		return
	}
	if config == nil {
		utils.SendEphemeralResponse(s, i, fmt.Sprintf("Guild with ID `%s` not found.", guildID))
		return
	}

	config.Enable = enable
	err = database.UpdateGuildConfig(db, *config)
	if err != nil {
		log.Printf("Error updating guild config for guild %s: %v", guildID, err)
		utils.SendEphemeralResponse(s, i, "Failed to update configuration.")
		return
	}

	status := "disabled"
	if enable {
		status = "enabled"
	}
	utils.SendEphemeralResponse(s, i, fmt.Sprintf("Successfully %s guild `%s` (%s).", status, config.Name, guildID))
}

func handleAddGuild(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB, options map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	guildOpt, ok := options["guild"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: guild option is missing.")
		return
	}
	guildID := guildOpt.StringValue()

	config, err := database.GetGuildConfig(db, guildID)
	if err != nil {
		log.Printf("Error checking guild config for guild %s: %v", guildID, err)
		utils.SendEphemeralResponse(s, i, "An error occurred while checking the configuration.")
		return
	}
	if config != nil {
		utils.SendEphemeralResponse(s, i, fmt.Sprintf("Guild with ID `%s` already exists.", guildID))
		return
	}

	// Fetch guild name from Discord API
	guild, err := s.Guild(guildID)
	if err != nil {
		log.Printf("Error fetching guild info for %s: %v", guildID, err)
		utils.SendEphemeralResponse(s, i, "Could not fetch guild information from Discord. Please ensure the ID is correct and the bot is in the server.")
		return
	}

	newConfig := model.ServerConfig{
		GuildID:      guildID,
		Name:         guild.Name,
		Enable:       false,
		AdminRoleIDs: []string{},
		UserRoleIDs:  []string{},
	}

	err = database.AddGuildConfig(db, newConfig)
	if err != nil {
		log.Printf("Error adding guild config for guild %s: %v", guildID, err)
		utils.SendEphemeralResponse(s, i, "Failed to add new guild configuration.")
		return
	}

	utils.SendEphemeralResponse(s, i, fmt.Sprintf("Successfully added guild `%s` (%s). It is disabled by default.", newConfig.Name, newConfig.GuildID))
}

func handleRole(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB, options map[string]*discordgo.ApplicationCommandInteractionDataOption, roleType string, add bool) {
	guildOpt, ok := options["guild"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: guild option is missing.")
		return
	}
	guildID := guildOpt.StringValue()

	roleOpt, ok := options["role"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: role option is missing.")
		return
	}
	roleID := roleOpt.RoleValue(s, i.GuildID).ID

	config, err := database.GetGuildConfig(db, guildID)
	if err != nil {
		log.Printf("Error getting guild config for guild %s: %v", guildID, err)
		utils.SendEphemeralResponse(s, i, "An error occurred while fetching the configuration.")
		return
	}
	if config == nil {
		utils.SendEphemeralResponse(s, i, fmt.Sprintf("Guild with ID `%s` not found.", guildID))
		return
	}

	var roleList []string
	if roleType == "admin" {
		roleList = config.AdminRoleIDs
	} else {
		roleList = config.UserRoleIDs
	}

	found := false
	for _, rID := range roleList {
		if rID == roleID {
			found = true
			break
		}
	}

	if add {
		if found {
			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Role <@&%s> is already in the %s list for guild `%s`.", roleID, roleType, config.Name))
			return
		}
		roleList = append(roleList, roleID)
	} else {
		if !found {
			utils.SendEphemeralResponse(s, i, fmt.Sprintf("Role <@&%s> not found in the %s list for guild `%s`.", roleID, roleType, config.Name))
			return
		}
		var newRoleList []string
		for _, rID := range roleList {
			if rID != roleID {
				newRoleList = append(newRoleList, rID)
			}
		}
		roleList = newRoleList
	}

	if roleType == "admin" {
		config.AdminRoleIDs = roleList
	} else {
		config.UserRoleIDs = roleList
	}

	err = database.UpdateGuildConfig(db, *config)
	if err != nil {
		log.Printf("Error updating guild config for guild %s: %v", guildID, err)
		utils.SendEphemeralResponse(s, i, "Failed to update configuration.")
		return
	}

	actionStr := "removed"
	if add {
		actionStr = "added"
	}
	utils.SendEphemeralResponse(s, i, fmt.Sprintf("Successfully %s role <@&%s> as %s for guild `%s`.", actionStr, roleID, roleType, config.Name))
}

func handleListGuildConfig(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB, options map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	guildOpt, ok := options["guild"]
	if !ok {
		utils.SendEphemeralResponse(s, i, "Error: guild option is missing.")
		return
	}
	guildID := guildOpt.StringValue()

	config, err := database.GetGuildConfig(db, guildID)
	if err != nil {
		log.Printf("Error getting guild config for guild %s: %v", guildID, err)
		utils.SendEphemeralResponse(s, i, "An error occurred while fetching the configuration.")
		return
	}
	if config == nil {
		utils.SendEphemeralResponse(s, i, fmt.Sprintf("Guild with ID `%s` not found.", guildID))
		return
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**Configuration for %s (`%s`)**\n", config.Name, config.GuildID))
	builder.WriteString(fmt.Sprintf("Enabled: `%v`\n", config.Enable))

	builder.WriteString("Admin Roles:\n")
	if len(config.AdminRoleIDs) > 0 && config.AdminRoleIDs[0] != "" {
		for _, id := range config.AdminRoleIDs {
			builder.WriteString(fmt.Sprintf("- <@&%s>\n", id))
		}
	} else {
		builder.WriteString("- None\n")
	}

	builder.WriteString("User Roles:\n")
	if len(config.UserRoleIDs) > 0 && config.UserRoleIDs[0] != "" {
		for _, id := range config.UserRoleIDs {
			builder.WriteString(fmt.Sprintf("- <@&%s>\n", id))
		}
	} else {
		builder.WriteString("- None\n")
	}

	utils.SendEphemeralResponse(s, i, builder.String())
}
