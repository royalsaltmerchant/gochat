package main

import (
	"fmt"
	"gochat/db"
)

func ensureHostClientSchema() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS spaces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			author_id INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			space_uuid TEXT NOT NULL,
			allow_voice INTEGER DEFAULT 0,
			FOREIGN KEY (space_uuid) REFERENCES spaces(uuid) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_uuid TEXT NOT NULL,
			content TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			timestamp TEXT,
			FOREIGN KEY (channel_uuid) REFERENCES channels(uuid) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS space_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			space_uuid TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			joined INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (space_uuid) REFERENCES spaces(uuid) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_channels_space_uuid ON channels(space_uuid)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_channel_time ON messages(channel_uuid, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_space_users_space_user ON space_users(space_uuid, user_id)`,
	}

	for _, stmt := range statements {
		if _, err := db.ChatDB.Exec(stmt); err != nil {
			return fmt.Errorf("schema exec failed: %w", err)
		}
	}

	if err := ensureColumnExists("channels", "allow_voice", `ALTER TABLE channels ADD COLUMN allow_voice INTEGER DEFAULT 0`); err != nil {
		return err
	}

	if !isOfficialHostInstance() {
		return nil
	}

	seedStatements := []string{
		fmt.Sprintf(`INSERT OR IGNORE INTO spaces (uuid, name, author_id) VALUES (%q, %q, 0)`, officialSpaceUUID, officialSpaceName),
	}
	for _, channelName := range officialSpaceChannels {
		channelUUID := fmt.Sprintf("%s-%s", officialSpaceUUID, channelName)
		seedStatements = append(seedStatements, fmt.Sprintf(
			`INSERT OR IGNORE INTO channels (uuid, name, space_uuid) VALUES (%q, %q, %q)`,
			channelUUID,
			channelName,
			officialSpaceUUID,
		))
	}

	for _, stmt := range seedStatements {
		if _, err := db.ChatDB.Exec(stmt); err != nil {
			return fmt.Errorf("seed exec failed: %w", err)
		}
	}

	return nil
}

func ensureColumnExists(tableName, columnName, alterStmt string) error {
	rows, err := db.ChatDB.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return fmt.Errorf("table_info query failed for %s: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("table_info scan failed for %s: %w", tableName, err)
		}
		if name == columnName {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("table_info row error for %s: %w", tableName, err)
	}

	if _, err := db.ChatDB.Exec(alterStmt); err != nil {
		return fmt.Errorf("alter table failed for %s.%s: %w", tableName, columnName, err)
	}
	return nil
}
