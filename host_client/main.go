package main

import (
	"context"
	"embed"
	"gochat/db"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var MigrationFiles embed.FS

func main() {
	cfg, err := LoadOrInitHostConfig()
	if err != nil {
		log.Fatal("Startup failed:", err)
	}

	// Init DB
	db.ChatDB, err = db.InitDB(cfg.DBFile, MigrationFiles, "migrations")
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.CloseDB(db.ChatDB)

	// Graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start WebSocket client
	go func() {
		err := SocketClient(ctx, cfg.UUID, cfg.AuthorID)
		if err != nil {
			log.Println("SocketClient error:", err)
		}
	}()

	// Listen for OS signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs

	log.Println("Interrupt received, shutting down...")
	cancel()

	// Try to send notice of offline to relay
	payload := map[string]string{
		"author_id": cfg.AuthorID,
	}

	resp, err := PostJSON(relayBaseURL.String()+"/api/host_offline/"+cfg.UUID, payload, nil)
	if err != nil {
		log.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	time.Sleep(1 * time.Second)
}
