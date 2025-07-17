package database

import (
	"database/sql"
	"discord-bot/model"
	"strings"
)

func CreateGuildTables(db *sql.DB) error {
	createGuildsTableSQL := `CREATE TABLE IF NOT EXISTS guild_configs (
		"guild_id" TEXT NOT NULL PRIMARY KEY,
		"name" TEXT,
		"admin_role_ids" TEXT,
		"user_role_ids" TEXT
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

	return nil
}

func LoadConfigFromDB(db *sql.DB, cfg *model.Config) error {
	rows, err := db.Query("SELECT guild_id, name, admin_role_ids, user_role_ids FROM guild_configs")
	if err != nil {
		return err
	}
	defer rows.Close()

	cfg.ServerConfigs = make(map[string]model.ServerConfig)
	for rows.Next() {
		var sc model.ServerConfig
		var adminRoles, userRoles string
		if err := rows.Scan(&sc.GuildID, &sc.Name, &adminRoles, &userRoles); err != nil {
			return err
		}
		sc.AdminRoleIDs = strings.Split(adminRoles, ",")
		sc.UserRoleIDs = strings.Split(userRoles, ",")
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
