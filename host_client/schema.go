package main

import (
	"fmt"
	"gochat/db"
)

func ensureHostClientSchema() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS chat_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_key TEXT NOT NULL UNIQUE,
			enc_public_key TEXT NOT NULL DEFAULT '',
			username TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
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
				message_id TEXT NOT NULL DEFAULT '',
				sender_auth_public_key TEXT NOT NULL DEFAULT '',
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
		`CREATE INDEX IF NOT EXISTS idx_chat_users_public_key ON chat_users(public_key)`,
	}

	for _, stmt := range statements {
		if _, err := db.ChatDB.Exec(stmt); err != nil {
			return fmt.Errorf("schema exec failed: %w", err)
		}
	}

	if err := ensureColumnExists("channels", "allow_voice", `ALTER TABLE channels ADD COLUMN allow_voice INTEGER DEFAULT 0`); err != nil {
		return err
	}
	if err := ensureColumnExists("chat_users", "created_at", `ALTER TABLE chat_users ADD COLUMN created_at TEXT`); err != nil {
		return err
	}
	if err := ensureColumnExists("chat_users", "updated_at", `ALTER TABLE chat_users ADD COLUMN updated_at TEXT`); err != nil {
		return err
	}
	if err := ensureColumnExists("chat_users", "enc_public_key", `ALTER TABLE chat_users ADD COLUMN enc_public_key TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := ensureColumnExists("messages", "message_id", `ALTER TABLE messages ADD COLUMN message_id TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := ensureColumnExists("messages", "sender_auth_public_key", `ALTER TABLE messages ADD COLUMN sender_auth_public_key TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if _, err := db.ChatDB.Exec(`UPDATE chat_users SET created_at = COALESCE(created_at, CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("failed to backfill chat_users.created_at: %w", err)
	}
	if _, err := db.ChatDB.Exec(`UPDATE chat_users SET updated_at = COALESCE(updated_at, CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("failed to backfill chat_users.updated_at: %w", err)
	}
	if _, err := db.ChatDB.Exec(`
		DELETE FROM space_users
		 WHERE id NOT IN (
			SELECT MIN(id)
			  FROM space_users
			 GROUP BY space_uuid, user_id
		 )
	`); err != nil {
		return fmt.Errorf("failed to dedupe space_users before unique index: %w", err)
	}
	if _, err := db.ChatDB.Exec(`DROP INDEX IF EXISTS idx_space_users_space_user`); err != nil {
		return fmt.Errorf("failed to drop legacy non-unique space_users index: %w", err)
	}
	if _, err := db.ChatDB.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_space_users_unique_space_user
			ON space_users(space_uuid, user_id)
	`); err != nil {
		return fmt.Errorf("failed to create unique space_users index: %w", err)
	}
	if _, err := db.ChatDB.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_channel_sender_msgid
			ON messages(channel_uuid, sender_auth_public_key, message_id)
			WHERE message_id <> '' AND sender_auth_public_key <> ''
	`); err != nil {
		return fmt.Errorf("failed to create message replay-protection index: %w", err)
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
