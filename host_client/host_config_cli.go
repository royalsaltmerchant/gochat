package main

import (
	"fmt"
)

func LoadOrInitHostConfigCLI() (*HostConfig, error) {
	// Try loading existing config first
	if cfg, err := loadExistingConfig(); err == nil {
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

	// Register with relay
	fmt.Println("Registering with relay server...")
	uuid, authorID, err := registerHostWithRelay(name)
	if err != nil {
		return nil, fmt.Errorf("failed to register host with relay: %w", err)
	}

	cfg := &HostConfig{
		UUID:     uuid,
		AuthorID: authorID,
		Name:     name,
		DBFile:   dbPath,
	}

	// Save config
	if err := saveConfig(cfg); err != nil {
		fmt.Printf("Warning: failed to save config: %v\n", err)
	}

	return cfg, nil
}
