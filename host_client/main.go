package main

import (
	"context"
	"gochat/db"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Load .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Init DB
	dbName := os.Getenv("CHAT_DB_FILE")
	db.ChatDB, err = db.InitDB(dbName)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.CloseDB(db.ChatDB)

	// Create a context that cancels on SIGINT or SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start socket client in a goroutine
	go func() {
		err := SocketClient(ctx, "d31433ba-8374-44a0-998d-12a54b34173d", "3adb5f62-31ae-46eb-abc6-a303d5f16188")
		if err != nil {
			log.Println("SocketClient error:", err)
		}
	}()

	// Listen for OS signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	// Wait for a signal
	<-sigs
	log.Println("Interrupt received, shutting down...")
	cancel()

	// Optional: give goroutines time to finish
	time.Sleep(1 * time.Second)
}
