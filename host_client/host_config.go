package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type HostConfig struct {
	UUID     string `json:"uuid"`
	AuthorID string `json:"author_id"`
	Name     string `json:"name"`
	DBFile   string `json:"db_file"`
}

// func generateSecret(n int) string {
// 	b := make([]byte, n)
// 	_, _ = rand.Read(b)
// 	return hex.EncodeToString(b)
// }

func getConfigPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	execDir := filepath.Dir(execPath)
	log.Println(execDir)
	return filepath.Join(execDir, "host_config.json"), nil
}

func LoadOrInitHostConfig() (*HostConfig, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	if data, err := os.ReadFile(configPath); err == nil {
		var cfg HostConfig
		if json.Unmarshal(data, &cfg) == nil && cfg.UUID != "" {
			return &cfg, nil
		}
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter a name for this host: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	uuid, authorID, err := registerHostWithRelay(name)
	if err != nil {
		return nil, fmt.Errorf("failed to register host: %w", err)
	}

	cfg := &HostConfig{
		UUID:     uuid,
		AuthorID: authorID,
		Name:     name,
		DBFile:   "chat.db",
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath, data, 0644)

	return cfg, nil
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
