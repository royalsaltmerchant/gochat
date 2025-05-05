package db

import (
	"database/sql"
	"fmt"
)

var DB *sql.DB

func InitDB(database string) error {
	var err error
	DB, err = sql.Open("sqlite3", database+"?_foreign_keys=1")
	if err != nil {
		return err
	}

	var enabled int
	err = DB.QueryRow("PRAGMA foreign_keys").Scan(&enabled)
	if err != nil {
		return fmt.Errorf("error checking foreign keys: %v", err)
	}
	if enabled != 1 {
		return fmt.Errorf("foreign keys are not enabled")
	}

	_, err = DB.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return fmt.Errorf("error enabling foreign keys: %v", err)
	}

	return nil
}

func CloseDB() {
	if DB != nil {
		DB.Close()
		fmt.Println("Database connection closed")
	}
}
