package main

import (
	"database/sql"
	"fmt"
	"gochat/db"
	"log"
	"os"
	"path/filepath"
	"time"
)

func prepareHostDatabase(dbPath string) error {
	if dbPath == "" {
		return fmt.Errorf("empty host DB path")
	}

	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat host DB: %w", err)
	}

	existingDB, err := db.InitSQLite(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open existing host DB: %w", err)
	}

	incompatible, reason, err := hasIncompatibleHostSchema(existingDB)
	if err != nil {
		_ = existingDB.Close()
		return err
	}

	if err := existingDB.Close(); err != nil {
		return fmt.Errorf("failed to close existing host DB: %w", err)
	}

	if !incompatible {
		return nil
	}

	ts := time.Now().UTC().Format("20060102_150405")
	archivePath := fmt.Sprintf("%s.legacy.%s", dbPath, ts)
	if err := os.Rename(dbPath, archivePath); err != nil {
		return fmt.Errorf("failed to archive incompatible host DB: %w", err)
	}
	_ = renameIfExists(dbPath+"-wal", archivePath+"-wal")
	_ = renameIfExists(dbPath+"-shm", archivePath+"-shm")

	log.Printf("Archived incompatible host DB: %s -> %s (%s)", dbPath, archivePath, reason)
	return nil
}

func hasIncompatibleHostSchema(conn *sql.DB) (bool, string, error) {
	requiredColumns := map[string][]string{
		"chat_users":  {"id", "public_key", "enc_public_key", "username"},
		"spaces":      {"id", "uuid", "name", "author_id"},
		"channels":    {"id", "uuid", "name", "space_uuid", "allow_voice"},
		"messages":    {"id", "channel_uuid", "content", "user_id", "timestamp"},
		"space_users": {"id", "space_uuid", "user_id", "joined"},
	}

	for tableName, cols := range requiredColumns {
		exists, colSet, err := tableColumns(conn, tableName)
		if err != nil {
			return false, "", fmt.Errorf("failed checking table %s: %w", tableName, err)
		}
		if !exists {
			continue
		}

		for _, col := range cols {
			if !colSet[col] {
				reason := fmt.Sprintf("table %s missing required column %s", tableName, col)
				return true, reason, nil
			}
		}
	}

	chatUsersExists, _, err := tableColumns(conn, "chat_users")
	if err != nil {
		return false, "", fmt.Errorf("failed checking table chat_users: %w", err)
	}
	if !chatUsersExists {
		legacyTables := []string{"spaces", "space_users", "messages"}
		for _, tableName := range legacyTables {
			count, err := tableRowCount(conn, tableName)
			if err != nil {
				return false, "", fmt.Errorf("failed counting rows in %s: %w", tableName, err)
			}
			if count > 0 {
				reason := "legacy identityless chat data detected; requires clean host DB"
				return true, reason, nil
			}
		}
	}

	return false, "", nil
}

func tableRowCount(conn *sql.DB, tableName string) (int, error) {
	var count int
	err := conn.QueryRow(`SELECT COUNT(1) FROM ` + tableName).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func tableColumns(conn *sql.DB, tableName string) (bool, map[string]bool, error) {
	rows, err := conn.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false, nil, err
	}
	defer rows.Close()

	cols := make(map[string]bool)
	count := 0

	for rows.Next() {
		count++
		var cid int
		var name string
		var ctype string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultValue, &pk); err != nil {
			return false, nil, err
		}
		cols[name] = true
	}

	if err := rows.Err(); err != nil {
		return false, nil, err
	}

	return count > 0, cols, nil
}

func renameIfExists(fromPath, toPath string) error {
	if _, err := os.Stat(fromPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if filepath.Clean(fromPath) == filepath.Clean(toPath) {
		return nil
	}
	return os.Rename(fromPath, toPath)
}
