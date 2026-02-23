package main

import (
	"context"
	"gochat/db"
	"log"
)

func runMainLogic(ctx context.Context, cfg *HostConfig) {
	var err error
	currentHostUUID = cfg.UUID
	runtimeHostConfig = cfg

	if err := prepareHostDatabase(cfg.DBFile); err != nil {
		log.Println("Error preparing host database:", err)
		return
	}

	db.ChatDB, err = db.InitSQLite(cfg.DBFile)
	if err != nil {
		log.Println("Error opening database:", err)
		return
	}
	defer db.CloseDB(db.ChatDB)
	if err := ensureHostClientSchema(); err != nil {
		log.Println("Error ensuring host schema:", err)
		return
	}

	go func() {
		err := SocketClient(ctx, cfg.UUID)
		if err != nil {
			log.Println("SocketClient error:", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")
}
