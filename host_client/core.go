package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gochat/db"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type HostConfig struct {
	UUID     string `json:"uuid"`
	AuthorID string `json:"author_id"`
	Name     string `json:"name"`
	DBFile   string `json:"db_file"`
}

func runMainLogic(ctx context.Context, cfg *HostConfig) {
	fmt.Println("🗝️  Host Key (UUID):", cfg.UUID)

	configPath, err := getAppSupportPathFor(configFileName)
	if err != nil {
		log.Println("Error resolving config path:", err)
	} else {
		fmt.Println("📄 Config File:", configPath)
	}

	if cfg.DBFile != "" {
		fmt.Println("🗂️  Database File:", cfg.DBFile)
	}

	var errDB error
	db.ChatDB, errDB = db.InitDB(cfg.DBFile, MigrationFiles, "migrations")
	if errDB != nil {
		log.Println("Error opening database:", errDB)
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
}

func getAppSupportPathFor(filename string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("unable to get user config directory: %w", err)
	}

	appDir := filepath.Join(configDir, "ParchHost")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", fmt.Errorf("unable to create config directory: %w", err)
	}

	configPath := filepath.Join(appDir, filename)
	log.Println("Using config path:", configPath)
	return configPath, nil
}

func registerHostWithRelay(name string) (uuid string, authorID string, err error) {
	url := relayBaseURL.String() + "/api/register_host"
	payload := map[string]string{"name": name}
	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("relay returned status %d", resp.StatusCode)
	}

	var result struct {
		UUID     string `json:"uuid"`
		AuthorID string `json:"author_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	return result.UUID, result.AuthorID, nil
}
