package main

import (
	"fmt"
	"gochat/db"
)

func ensureChatRelaySchema() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS hosts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			author_id TEXT NOT NULL UNIQUE,
			online INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS chat_identities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_key TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_identities_public_key ON chat_identities(public_key)`,
	}

	for _, stmt := range statements {
		if _, err := db.HostDB.Exec(stmt); err != nil {
			return fmt.Errorf("schema exec failed: %w", err)
		}
	}

	if err := ensureRelayColumnExists("hosts", "online", `ALTER TABLE hosts ADD COLUMN online INTEGER DEFAULT 0`); err != nil {
		return err
	}
	if err := ensureRelayColumnExists("chat_identities", "created_at", `ALTER TABLE chat_identities ADD COLUMN created_at TEXT`); err != nil {
		return err
	}
	if err := ensureRelayColumnExists("chat_identities", "updated_at", `ALTER TABLE chat_identities ADD COLUMN updated_at TEXT`); err != nil {
		return err
	}

	if _, err := db.HostDB.Exec(`UPDATE chat_identities SET created_at = COALESCE(created_at, CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("failed to backfill chat_identities.created_at: %w", err)
	}
	if _, err := db.HostDB.Exec(`UPDATE chat_identities SET updated_at = COALESCE(updated_at, CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("failed to backfill chat_identities.updated_at: %w", err)
	}

	return nil
}

func ensureRelayColumnExists(tableName, columnName, alterStmt string) error {
	rows, err := db.HostDB.Query("PRAGMA table_info(" + tableName + ")")
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

	if _, err := db.HostDB.Exec(alterStmt); err != nil {
		return fmt.Errorf("alter table failed for %s.%s: %w", tableName, columnName, err)
	}
	return nil
}
