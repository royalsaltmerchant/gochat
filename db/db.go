package db

import (
	"database/sql"
	"fmt"
)

var DB *sql.DB

func InitDB(database string) error {
	var err error
	DB, err = sql.Open("sqlite3", database)
	if err != nil {
		return err
	}

	return nil
}

func CloseDB() {
	if DB != nil {
		DB.Close()
		fmt.Println("Database connection closed")
	}
}
