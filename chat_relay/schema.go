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
			signing_public_key TEXT NOT NULL DEFAULT '',
			online INTEGER DEFAULT 0
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.HostDB.Exec(stmt); err != nil {
			return fmt.Errorf("schema exec failed: %w", err)
		}
	}
	if _, err := db.HostDB.Exec(`DROP TABLE IF EXISTS chat_identities`); err != nil {
		return fmt.Errorf("failed to drop legacy chat_identities table: %w", err)
	}
	if err := migrateHostsTableWithoutAuthorID(); err != nil {
		return err
	}

	if err := ensureRelayColumnExists("hosts", "online", `ALTER TABLE hosts ADD COLUMN online INTEGER DEFAULT 0`); err != nil {
		return err
	}
	if err := ensureRelayColumnExists("hosts", "signing_public_key", `ALTER TABLE hosts ADD COLUMN signing_public_key TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}

	return nil
}

func relayTableColumns(tableName string) (map[string]struct{}, error) {
	rows, err := db.HostDB.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return nil, fmt.Errorf("table_info query failed for %s: %w", tableName, err)
	}
	defer rows.Close()

	columns := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("table_info scan failed for %s: %w", tableName, err)
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("table_info row error for %s: %w", tableName, err)
	}
	return columns, nil
}

func migrateHostsTableWithoutAuthorID() error {
	columns, err := relayTableColumns("hosts")
	if err != nil {
		return err
	}
	if _, hasAuthorID := columns["author_id"]; !hasAuthorID {
		return nil
	}

	selectSigning := "'' AS signing_public_key"
	if _, ok := columns["signing_public_key"]; ok {
		selectSigning = "COALESCE(signing_public_key, '') AS signing_public_key"
	}
	selectOnline := "0 AS online"
	if _, ok := columns["online"]; ok {
		selectOnline = "COALESCE(online, 0) AS online"
	}

	if _, err := db.HostDB.Exec(`DROP TABLE IF EXISTS hosts_new`); err != nil {
		return fmt.Errorf("failed dropping stale hosts_new table: %w", err)
	}
	if _, err := db.HostDB.Exec(`
		CREATE TABLE IF NOT EXISTS hosts_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			signing_public_key TEXT NOT NULL DEFAULT '',
			online INTEGER DEFAULT 0
		)
	`); err != nil {
		return fmt.Errorf("failed creating hosts_new table: %w", err)
	}

	copyQuery := fmt.Sprintf(`
		INSERT INTO hosts_new (id, uuid, name, signing_public_key, online)
		SELECT id, uuid, name, %s, %s FROM hosts
	`, selectSigning, selectOnline)
	if _, err := db.HostDB.Exec(copyQuery); err != nil {
		return fmt.Errorf("failed copying hosts data into hosts_new: %w", err)
	}
	if _, err := db.HostDB.Exec(`DROP TABLE hosts`); err != nil {
		return fmt.Errorf("failed dropping legacy hosts table: %w", err)
	}
	if _, err := db.HostDB.Exec(`ALTER TABLE hosts_new RENAME TO hosts`); err != nil {
		return fmt.Errorf("failed renaming hosts_new table: %w", err)
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
