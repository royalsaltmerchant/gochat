//go:build windows

package main

import (
	"context"
	"embed"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var MigrationFiles embed.FS

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C or termination signals
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		sig := <-sigChan
		log.Println("Received signal:", sig)
		cancel()
	}()

	LoadOrInitHostConfig(func(cfg *HostConfig) {
		runMainLogic(ctx, cfg)
	})
}
