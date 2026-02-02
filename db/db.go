// db/db.go
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	sqlite3 "modernc.org/sqlite"
)

func init() {
	// Register modernc.org/sqlite under the "sqlite3" name for compatibility
	// with golang-migrate and existing code
	sql.Register("sqlite3", &sqlite3.Driver{})
}

var ChatDB *sql.DB
var HostDB *sql.DB

func InitDB(databaseName string, migrations embed.FS, migrationPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", databaseName)
	if err != nil {
		return nil, err
	}

	// Enable foreign keys - modernc.org/sqlite requires explicit PRAGMA execution
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("error enabling foreign keys: %v", err)
	}

	var enabled int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&enabled)
	if err != nil {
		return nil, fmt.Errorf("error checking foreign keys: %v", err)
	}
	if enabled != 1 {
		return nil, fmt.Errorf("foreign keys are not enabled")
	}

	err = runMigrations(db, migrations, migrationPath)
	if err != nil {
		return nil, fmt.Errorf("migration error: %v", err)
	}

	return db, nil
}

func CloseDB(databaseInstance *sql.DB) {
	if databaseInstance != nil {
		databaseInstance.Close()
		fmt.Println("Database connection closed")
	}
}

func runMigrations(db *sql.DB, migrations embed.FS, migrationPath string) error {
	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("sqlite driver error: %w", err)
	}

	d, err := iofs.New(migrations, migrationPath)
	if err != nil {
		return fmt.Errorf("iofs source error: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("migrate instance error: %w", err)
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up error: %w", err)
	}

	log.Println("Migrations applied successfully")
	return nil
}
