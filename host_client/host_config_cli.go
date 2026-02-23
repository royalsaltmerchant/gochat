package main

import (
	"fmt"
	"log"
)

func LoadOrInitHostConfigCLI() (*HostConfig, error) {
	// Try loading existing config first
	if cfg, err := loadExistingConfig(); err == nil {
		if err := ensureHostRegisteredWithRelay(cfg); err != nil {
			log.Printf("Warning: failed to sync host registration with relay: %v", err)
		}
		_ = saveConfig(cfg)
		return cfg, nil
	}

	dbPath, err := getAppSupportPathFor(dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve DB path: %w", err)
	}

	// No valid config: prompt for host name
	fmt.Println("No existing host configuration found.")
	name := promptInput("Enter a name for this host: ")
	if name == "" {
		return nil, fmt.Errorf("host name cannot be empty")
	}
	tmpCfg := &HostConfig{Name: name}
	if err := ensureHostSigningKeys(tmpCfg); err != nil {
		return nil, fmt.Errorf("failed to initialize host signing keys: %w", err)
	}

	// Register with relay
	fmt.Println("Registering with relay server...")
	uuid, err := registerHostWithRelay("", name, tmpCfg.SigningPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to register host with relay: %w", err)
	}

	cfg := &HostConfig{
		UUID:              uuid,
		Name:              name,
		DBFile:            dbPath,
		SigningPublicKey:  tmpCfg.SigningPublicKey,
		SigningPrivateKey: tmpCfg.SigningPrivateKey,
	}

	// Save config
	if err := saveConfig(cfg); err != nil {
		fmt.Printf("Warning: failed to save config: %v\n", err)
	}

	return cfg, nil
}
