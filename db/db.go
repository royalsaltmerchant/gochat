package db

import (
	"database/sql"
	"fmt"
)

var ChatDB *sql.DB
var HostDB *sql.DB

func InitDB(databaseName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", databaseName+"?_foreign_keys=1")
	if err != nil {
		return nil, err
	}

	var enabled int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&enabled)
	if err != nil {
		return nil, fmt.Errorf("error checking foreign keys: %v", err)
	}
	if enabled != 1 {
		return nil, fmt.Errorf("foreign keys are not enabled")
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return nil, fmt.Errorf("error enabling foreign keys: %v", err)
	}

	return db, nil
}

func CloseDB(databaseInstance *sql.DB) {
	if databaseInstance != nil {
		databaseInstance.Close()
		fmt.Println("Database connection closed")
	}
}
