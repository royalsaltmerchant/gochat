package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AuthTokenManager struct {
	File string // e.g. "auth_tokens.json"
}

type AuthTokenConfig map[string]string // map[hostUUID]token

func NewAuthTokenManager() (*AuthTokenManager, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(configDir, "gochat", "auth_tokens.json")

	// Ensure directory exists
	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return nil, err
	}

	return &AuthTokenManager{File: path}, nil
}

func (a *AuthTokenManager) SaveAuthToken(hostUUID, token string) error {
	config := AuthTokenConfig{}

	// Load existing file if it exists
	if data, err := os.ReadFile(a.File); err == nil {
		_ = json.Unmarshal(data, &config)
	}

	// Overwrite or add
	config[hostUUID] = token

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(a.File, data, 0600)
}
func (a *AuthTokenManager) LoadAuthToken(hostUUID string) (string, error) {
	data, err := os.ReadFile(a.File)
	if err != nil {
		return "", nil // no file yet
	}

	var config AuthTokenConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return "", err
	}

	return config[hostUUID], nil
}

func (a *AuthTokenManager) RemoveAuthToken(hostUUID string) error {
	data, err := os.ReadFile(a.File)
	if err != nil {
		// If the file doesn't exist, there's nothing to remove
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var config AuthTokenConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	// Remove the token for the given hostUUID
	delete(config, hostUUID)

	// If the map is now empty, delete the file
	if len(config) == 0 {
		return os.Remove(a.File)
	}

	// Otherwise, write updated map back to file
	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(a.File, newData, 0600)
}
