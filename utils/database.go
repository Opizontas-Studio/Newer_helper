package utils

import (
	"database/sql"
	"discord-bot/model"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(filepath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func CreateTable(db *sql.DB, tableName string) error {
	createTableSQL := `CREATE TABLE IF NOT EXISTS "` + tableName + `" (
		"id" TEXT NOT NULL PRIMARY KEY,
		"title" TEXT,
		"author" TEXT,
		"author_id" TEXT,
		"content" TEXT,
		"tags" TEXT,
		"message_count" INTEGER,
		"timestamp" INTEGER,
		"cover_image_url" TEXT
	);`

	_, err := db.Exec(createTableSQL)
	return err
}

func InsertPost(db *sql.DB, post model.Post, tableName string) error {
	if err := CreateTable(db, tableName); err != nil {
		return err
	}

	insertSQL := `INSERT OR REPLACE INTO "` + tableName + `"(id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(insertSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(post.ID, post.Title, post.Author, post.AuthorID, post.Content, post.Tags, post.MessageCount, post.Timestamp, post.CoverImageURL)
	return err
}

func GetAllPosts(db *sql.DB, tableName string) ([]model.Post, error) {
	rows, err := db.Query(`SELECT id, title, author, author_id, content, tags, message_count, timestamp, cover_image_url FROM "` + tableName + `"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var post model.Post
		if err := rows.Scan(&post.ID, &post.Title, &post.Author, &post.AuthorID, &post.Content, &post.Tags, &post.MessageCount, &post.Timestamp, &post.CoverImageURL); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func InitGuildDB(filepath string) (*sql.DB, error) {
	if err := os.MkdirAll("./data", os.ModePerm); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}

	createGuildsTableSQL := `CREATE TABLE IF NOT EXISTS guild_configs (
		"guild_id" TEXT NOT NULL PRIMARY KEY,
		"name" TEXT,
		"admin_role_ids" TEXT,
		"user_role_ids" TEXT
	);`
	_, err = db.Exec(createGuildsTableSQL)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	return db, nil
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
		if err := presetRows.Scan(&p.ID, &guildID, &p.Name, &p.Value, &p.Description, &p.Type); err != nil {
			return err
		}
		if sc, ok := cfg.ServerConfigs[guildID]; ok {
			sc.PresetMessages = append(sc.PresetMessages, p)
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
