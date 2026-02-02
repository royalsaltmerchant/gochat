package main

import (
	"context"
	"embed"
	"gochat/db"
	"log"
	"time"
)

//go:embed migrations/*.sql
var MigrationFiles embed.FS

func runMainLogic(ctx context.Context, cfg *HostConfig) {
	var err error
	db.ChatDB, err = db.InitDB(cfg.DBFile, MigrationFiles, "migrations")
	if err != nil {
		log.Println("Error opening database:", err)
		return
	}
	defer db.CloseDB(db.ChatDB)

	go func() {
		err := SocketClient(ctx, cfg.UUID, cfg.AuthorID)
		if err != nil {
			log.Println("SocketClient error:", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")

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
