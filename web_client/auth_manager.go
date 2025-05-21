package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AuthTokenManager struct {
	File string // e.g. "auth_token.json"
}

type AuthToken struct {
	Token string `json:"token"`
}

func NewAuthTokenManager() (*AuthTokenManager, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(configDir, "ParchClient", "auth_token.json")

	// Ensure directory exists
	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return nil, err
	}

	return &AuthTokenManager{File: path}, nil
}

func (a *AuthTokenManager) SaveToken(token string) error {
	data, err := json.MarshalIndent(AuthToken{Token: token}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.File, data, 0600)
}

func (a *AuthTokenManager) LoadToken() (string, error) {
	data, err := os.ReadFile(a.File)
	if err != nil {
		return "", nil // no file yet
	}

	var authToken AuthToken
	if err := json.Unmarshal(data, &authToken); err != nil {
		return "", err
	}
	return authToken.Token, nil
}

func (a *AuthTokenManager) RemoveToken() error {
	if _, err := os.Stat(a.File); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(a.File)
}
