package main

import (
	"context"
	"gochat/db"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	cfg, err := LoadOrInitHostConfig()
	if err != nil {
		log.Fatal("Startup failed:", err)
	}

	// Init DB
	db.ChatDB, err = db.InitDB(cfg.DBFile)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.CloseDB(db.ChatDB)

	// Graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start WebSocket client
	go func() {
		err := SocketClient(ctx, cfg.UUID, cfg.AuthorID, cfg.JWTSecret)
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
	time.Sleep(1 * time.Second)
}
