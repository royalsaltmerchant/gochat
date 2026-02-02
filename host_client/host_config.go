package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type HostConfig struct {
	UUID     string `json:"uuid"`
	AuthorID string `json:"author_id"`
	Name     string `json:"name"`
	DBFile   string `json:"db_file"`
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

func loadExistingConfig() (*HostConfig, error) {
	configPath, err := getAppSupportPathFor(configFileName)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg HostConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.UUID == "" {
		return nil, fmt.Errorf("invalid config: missing UUID")
	}

	// Update DB path to current location
	dbPath, err := getAppSupportPathFor(dbName)
	if err != nil {
		return nil, err
	}
	cfg.DBFile = dbPath

	return &cfg, nil
}

func saveConfig(cfg *HostConfig) error {
	configPath, err := getAppSupportPathFor(configFileName)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
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
