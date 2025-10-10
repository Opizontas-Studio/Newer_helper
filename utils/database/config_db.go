package database

import (
	"database/sql"
	"newer_helper/model"
	"strings"
)

func CreateGuildTables(db *sql.DB) error {
	createGuildsTableSQL := `CREATE TABLE IF NOT EXISTS guild_configs (
		"guild_id" TEXT NOT NULL PRIMARY KEY,
		"name" TEXT,
		"admin_role_ids" TEXT,
		"user_role_ids" TEXT,
		"enable" TEXT DEFAULT 'false'
	);`
	_, err := db.Exec(createGuildsTableSQL)
	if err != nil {
		return err
	}

	createPresetsTableSQL := `CREATE TABLE IF NOT EXISTS preset_messages (
		"id" TEXT NOT NULL PRIMARY KEY,
		"guild_id" TEXT NOT NULL,
		"name" TEXT,
		"value" TEXT,
		"description" TEXT,
		"type" TEXT,
		FOREIGN KEY(guild_id) REFERENCES guild_configs(guild_id)
	);`
	_, err = db.Exec(createPresetsTableSQL)
	if err != nil {
		return err
	}

	createTopChannelsTableSQL := `CREATE TABLE IF NOT EXISTS top_channels (
		"channel_id" TEXT NOT NULL PRIMARY KEY,
		"guild_id" TEXT NOT NULL,
		"message_limit" INTEGER,
		"excluded_message_ids" TEXT,
		FOREIGN KEY(guild_id) REFERENCES guild_configs(guild_id)
	);`
	_, err = db.Exec(createTopChannelsTableSQL)
	if err != nil {
		return err
	}

	createLeaderboardAdsTableSQL := `CREATE TABLE IF NOT EXISTS leaderboard_ads (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"guild_id" TEXT NOT NULL,
		"content" TEXT NOT NULL,
		"image_url" TEXT,
		"enabled" BOOLEAN NOT NULL DEFAULT TRUE,
		FOREIGN KEY(guild_id) REFERENCES guild_configs(guild_id)
	);`
	_, err = db.Exec(createLeaderboardAdsTableSQL)
	if err != nil {
		return err
	}

	createPunishmentStatsChannelsTableSQL := `CREATE TABLE IF NOT EXISTS punishment_stats_channels (
		"channel_id" TEXT NOT NULL PRIMARY KEY,
		"guild_id" TEXT NOT NULL,
		"message_id" TEXT,
		"target_guild_id" TEXT NOT NULL,
		FOREIGN KEY(guild_id) REFERENCES guild_configs(guild_id)
	);`
	_, err = db.Exec(createPunishmentStatsChannelsTableSQL)
	if err != nil {
		return err
	}

	createAutoTriggersTableSQL := `CREATE TABLE IF NOT EXISTS auto_triggers (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"guild_id" TEXT NOT NULL,
		"keywords" TEXT NOT NULL,
		"preset_id" TEXT NOT NULL,
		"channel_id" TEXT NOT NULL,
		FOREIGN KEY(guild_id) REFERENCES guild_configs(guild_id),
		UNIQUE (guild_id, preset_id, channel_id)
	);`
	_, err = db.Exec(createAutoTriggersTableSQL)
	if err != nil {
		return err
	}
	return nil
}

func LoadConfigFromDB(db *sql.DB, cfg *model.Config) error {
	rows, err := db.Query("SELECT guild_id, name, admin_role_ids, user_role_ids, enable FROM guild_configs")
	if err != nil {
		return err
	}
	defer rows.Close()

	cfg.ServerConfigs = make(map[string]model.ServerConfig)
	for rows.Next() {
		var sc model.ServerConfig
		var adminRoles, userRoles, enableStr string
		if err := rows.Scan(&sc.GuildID, &sc.Name, &adminRoles, &userRoles, &enableStr); err != nil {
			return err
		}
		if sc.GuildID == "0" {
			continue
		}
		sc.AdminRoleIDs = strings.Split(adminRoles, ",")
		sc.UserRoleIDs = strings.Split(userRoles, ",")
		sc.Enable = enableStr == "true"
		sc.PresetMessages = []model.PresetMessage{} // Will be loaded separately
		cfg.ServerConfigs[sc.GuildID] = sc
	}

	presetRows, err := db.Query("SELECT id, guild_id, name, value, description, type FROM preset_messages")
	if err != nil {
		return err
	}
	defer presetRows.Close()

	for presetRows.Next() {
		var p model.PresetMessage
		var guildID string
		var description sql.NullString
		if err := presetRows.Scan(&p.ID, &guildID, &p.Name, &p.Value, &description, &p.Type); err != nil {
			return err
		}
		if description.Valid {
			p.Description = description.String
		}
		if sc, ok := cfg.ServerConfigs[guildID]; ok {
			sc.PresetMessages = append(sc.PresetMessages, p)
			cfg.ServerConfigs[guildID] = sc
		}
	}

	topChannelRows, err := db.Query("SELECT channel_id, guild_id, message_limit, excluded_message_ids FROM top_channels")
	if err != nil {
		return err
	}
	defer topChannelRows.Close()

	for topChannelRows.Next() {
		var tc model.TopChannelConfig
		var guildID string
		var excludedIDs string
		if err := topChannelRows.Scan(&tc.ChannelID, &guildID, &tc.MessageLimit, &excludedIDs); err != nil {
			return err
		}
		if excludedIDs != "" {
			tc.ExcludedMessageIDs = strings.Split(excludedIDs, ",")
		} else {
			tc.ExcludedMessageIDs = []string{}
		}
		if sc, ok := cfg.ServerConfigs[guildID]; ok {
			if sc.TopChannels == nil {
				sc.TopChannels = make(map[string]*model.TopChannelConfig)
			}
			sc.TopChannels[tc.ChannelID] = &tc
			cfg.ServerConfigs[guildID] = sc
		}
	}

	punishmentStatsRows, err := db.Query("SELECT channel_id, guild_id, message_id, target_guild_id FROM punishment_stats_channels")
	if err != nil {
		return err
	}
	defer punishmentStatsRows.Close()

	cfg.PunishmentStatsChannels = make(map[string]model.PunishmentStatsChannel)
	autoTriggerRows, err := db.Query("SELECT id, guild_id, keywords, preset_id, channel_id FROM auto_triggers")
	if err != nil {
		return err
	}
	defer autoTriggerRows.Close()

	for autoTriggerRows.Next() {
		var at model.AutoTriggerConfig
		var guildID, keywords string
		if err := autoTriggerRows.Scan(&at.ID, &guildID, &keywords, &at.PresetID, &at.ChannelID); err != nil {
			return err
		}
		at.Keywords = strings.Split(keywords, ",")
		if sc, ok := cfg.ServerConfigs[guildID]; ok {
			sc.AutoTriggers = append(sc.AutoTriggers, at)
			cfg.ServerConfigs[guildID] = sc
		}
	}
	for punishmentStatsRows.Next() {
		var psc model.PunishmentStatsChannel
		var messageID sql.NullString
		if err := punishmentStatsRows.Scan(&psc.ChannelID, &psc.GuildID, &messageID, &psc.TargetGuildID); err != nil {
			return err
		}
		if messageID.Valid {
			psc.MessageID = messageID.String
		}
		cfg.PunishmentStatsChannels[psc.ChannelID] = psc
	}

	return nil
}

func AddPreset(db *sql.DB, guildID string, preset model.PresetMessage) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("INSERT INTO preset_messages (id, guild_id, name, value, description, type) VALUES (?, ?, ?, ?, ?, ?)",
		preset.ID, guildID, preset.Name, preset.Value, preset.Description, preset.Type)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func SaveTopChannelConfig(db *sql.DB, guildID string, config model.TopChannelConfig) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	excludedIDs := strings.Join(config.ExcludedMessageIDs, ",")

	_, err = tx.Exec(`
		INSERT INTO top_channels (channel_id, guild_id, message_limit, excluded_message_ids)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(channel_id) DO UPDATE SET
			message_limit = excluded.message_limit,
			excluded_message_ids = excluded.excluded_message_ids;
	`, config.ChannelID, guildID, config.MessageLimit, excludedIDs)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func UpdatePreset(db *sql.DB, guildID string, preset model.PresetMessage) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE preset_messages SET name = ?, value = ?, description = ?, type = ? WHERE id = ? AND guild_id = ?",
		preset.Name, preset.Value, preset.Description, preset.Type, preset.ID, guildID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func AddLeaderboardAd(db *sql.DB, guildID string, content string, imageURL string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("INSERT INTO leaderboard_ads (guild_id, content, image_url, enabled) VALUES (?, ?, ?, ?)",
		guildID, content, imageURL, true)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func DeleteLeaderboardAd(db *sql.DB, adID int, guildID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM leaderboard_ads WHERE id = ? AND guild_id = ?", adID, guildID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func ListLeaderboardAds(db *sql.DB, guildID string) ([]model.LeaderboardAd, error) {
	rows, err := db.Query("SELECT id, guild_id, content, image_url, enabled FROM leaderboard_ads WHERE guild_id = ? ORDER BY id ASC", guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ads []model.LeaderboardAd
	for rows.Next() {
		var ad model.LeaderboardAd
		var imageURL sql.NullString
		if err := rows.Scan(&ad.ID, &ad.GuildID, &ad.Content, &imageURL, &ad.Enabled); err != nil {
			return nil, err
		}
		if imageURL.Valid {
			ad.ImageURL = imageURL.String
		}
		ads = append(ads, ad)
	}
	return ads, nil
}

func ToggleLeaderboardAd(db *sql.DB, adID int, guildID string, enabled bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE leaderboard_ads SET enabled = ? WHERE id = ? AND guild_id = ?", enabled, adID, guildID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func GetRandomEnabledLeaderboardAd(db *sql.DB, guildID string) (*model.LeaderboardAd, error) {
	row := db.QueryRow("SELECT id, guild_id, content, image_url, enabled FROM leaderboard_ads WHERE guild_id = ? AND enabled = TRUE ORDER BY RANDOM() LIMIT 1", guildID)

	var ad model.LeaderboardAd
	var imageURL sql.NullString
	err := row.Scan(&ad.ID, &ad.GuildID, &ad.Content, &imageURL, &ad.Enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No ad found is not an error
		}
		return nil, err
	}
	if imageURL.Valid {
		ad.ImageURL = imageURL.String
	}
	return &ad, nil
}

func DeletePreset(db *sql.DB, guildID string, presetID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM preset_messages WHERE id = ? AND guild_id = ?", presetID, guildID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func AddPunishmentStatsChannel(db *sql.DB, guildID, channelID, targetGuildID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("INSERT INTO punishment_stats_channels (guild_id, channel_id, target_guild_id) VALUES (?, ?, ?)",
		guildID, channelID, targetGuildID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func DeletePunishmentStatsChannel(db *sql.DB, channelID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM punishment_stats_channels WHERE channel_id = ?", channelID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func UpdatePunishmentStatsTargetGuild(db *sql.DB, channelID, targetGuildID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE punishment_stats_channels SET target_guild_id = ? WHERE channel_id = ?", targetGuildID, channelID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func UpdatePunishmentStatsChannel(db *sql.DB, channelID, messageID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE punishment_stats_channels SET message_id = ? WHERE channel_id = ?", messageID, channelID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
func AddAutoTrigger(db *sql.DB, guildID, keyword, presetID, channelID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	var keywords string
	err = tx.QueryRow("SELECT keywords FROM auto_triggers WHERE guild_id = ? AND preset_id = ? AND channel_id = ?", guildID, presetID, channelID).Scan(&keywords)
	if err != nil && err != sql.ErrNoRows {
		tx.Rollback()
		return err
	}

	if err == sql.ErrNoRows {
		_, err = tx.Exec("INSERT INTO auto_triggers (guild_id, keywords, preset_id, channel_id) VALUES (?, ?, ?, ?)",
			guildID, keyword, presetID, channelID)
	} else {
		newKeywords := keywords + "," + keyword
		_, err = tx.Exec("UPDATE auto_triggers SET keywords = ? WHERE guild_id = ? AND preset_id = ? AND channel_id = ?",
			newKeywords, guildID, presetID, channelID)
	}

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func DeleteAutoTrigger(db *sql.DB, guildID, keyword, channelID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	rows, err := tx.Query("SELECT id, keywords FROM auto_triggers WHERE guild_id = ? AND channel_id = ?", guildID, channelID)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var keywords string
		if err := rows.Scan(&id, &keywords); err != nil {
			continue
		}

		keywordList := strings.Split(keywords, ",")
		newKeywordList := []string{}
		for _, k := range keywordList {
			if k != keyword {
				newKeywordList = append(newKeywordList, k)
			}
		}

		if len(newKeywordList) == 0 {
			_, err = tx.Exec("DELETE FROM auto_triggers WHERE id = ?", id)
		} else {
			newKeywords := strings.Join(newKeywordList, ",")
			_, err = tx.Exec("UPDATE auto_triggers SET keywords = ? WHERE id = ?", newKeywords, id)
		}

		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
func GetGuildConfig(db *sql.DB, guildID string) (*model.ServerConfig, error) {
	row := db.QueryRow("SELECT name, admin_role_ids, user_role_ids, enable FROM guild_configs WHERE guild_id = ?", guildID)

	var sc model.ServerConfig
	var adminRoles, userRoles, enableStr string
	err := row.Scan(&sc.Name, &adminRoles, &userRoles, &enableStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, err
	}

	sc.GuildID = guildID
	sc.AdminRoleIDs = strings.Split(adminRoles, ",")
	sc.UserRoleIDs = strings.Split(userRoles, ",")
	sc.Enable = enableStr == "true"

	return &sc, nil
}

func AddGuildConfig(db *sql.DB, config model.ServerConfig) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	adminRoles := strings.Join(config.AdminRoleIDs, ",")
	userRoles := strings.Join(config.UserRoleIDs, ",")
	enableStr := "false"
	if config.Enable {
		enableStr = "true"
	}

	_, err = tx.Exec("INSERT INTO guild_configs (guild_id, name, admin_role_ids, user_role_ids, enable) VALUES (?, ?, ?, ?, ?)",
		config.GuildID, config.Name, adminRoles, userRoles, enableStr)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func UpdateGuildConfig(db *sql.DB, config model.ServerConfig) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	adminRoles := strings.Join(config.AdminRoleIDs, ",")
	userRoles := strings.Join(config.UserRoleIDs, ",")
	enableStr := "false"
	if config.Enable {
		enableStr = "true"
	}

	_, err = tx.Exec("UPDATE guild_configs SET name = ?, admin_role_ids = ?, user_role_ids = ?, enable = ? WHERE guild_id = ?",
		config.Name, adminRoles, userRoles, enableStr, config.GuildID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func GetAllGuilds(db *sql.DB) ([]model.ServerConfig, error) {
	rows, err := db.Query("SELECT guild_id, name, enable FROM guild_configs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var guilds []model.ServerConfig
	for rows.Next() {
		var sc model.ServerConfig
		var enableStr string
		if err := rows.Scan(&sc.GuildID, &sc.Name, &enableStr); err != nil {
			return nil, err
		}
		sc.Enable = enableStr == "true"
		guilds = append(guilds, sc)
	}
	return guilds, nil
}
func OverwriteAutoTrigger(db *sql.DB, guildID, keyword, presetID, channelID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE auto_triggers SET keywords = ? WHERE guild_id = ? AND preset_id = ? AND channel_id = ?",
		keyword, guildID, presetID, channelID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
