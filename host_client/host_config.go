package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type HostConfig struct {
	UUID              string `json:"uuid"`
	Name              string `json:"name"`
	DBFile            string `json:"db_file"`
	SigningPublicKey  string `json:"signing_public_key,omitempty"`
	SigningPrivateKey string `json:"signing_private_key,omitempty"`
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
	if err := ensureHostSigningKeys(&cfg); err != nil {
		return nil, err
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

func generateHostSigningKeyPair() (publicKey string, privateKey string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.RawStdEncoding.EncodeToString(pub), base64.RawStdEncoding.EncodeToString(priv), nil
}

func ensureHostSigningKeys(cfg *HostConfig) error {
	if cfg == nil {
		return fmt.Errorf("missing host config")
	}
	if strings.TrimSpace(cfg.SigningPublicKey) != "" && strings.TrimSpace(cfg.SigningPrivateKey) != "" {
		return nil
	}
	publicKey, privateKey, err := generateHostSigningKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate host signing keypair: %w", err)
	}
	cfg.SigningPublicKey = publicKey
	cfg.SigningPrivateKey = privateKey
	return nil
}

func registerHostWithRelay(hostUUID string, name string, signingPublicKey string) (uuid string, err error) {
	url := relayBaseURL.String() + "/api/register_host"
	payload := map[string]string{
		"uuid":               strings.TrimSpace(hostUUID),
		"name":               name,
		"signing_public_key": signingPublicKey,
	}
	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("relay returned status %d", resp.StatusCode)
	}

	var result struct {
		UUID string `json:"uuid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.UUID, nil
}

func ensureHostRegisteredWithRelay(cfg *HostConfig) error {
	if cfg == nil {
		return fmt.Errorf("missing host config")
	}
	hostUUID := strings.TrimSpace(cfg.UUID)
	if hostUUID == "" {
		return fmt.Errorf("missing host UUID")
	}
	registeredUUID, err := registerHostWithRelay(hostUUID, cfg.Name, cfg.SigningPublicKey)
	if err != nil {
		return err
	}
	if registeredUUID != hostUUID {
		return fmt.Errorf("relay returned mismatched host UUID")
	}
	return nil
}
