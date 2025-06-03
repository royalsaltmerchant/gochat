//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func LoadOrInitHostConfig(onDone func(*HostConfig)) {
	// 🔥 No more go func()
	configPath, err := getAppSupportPathFor(configFileName)
	if err != nil {
		log.Fatal("Failed to resolve config path:", err)
	}
	dbPath, err := getAppSupportPathFor(dbName)
	if err != nil {
		log.Fatal("Failed to resolve DB path:", err)
	}

	var cfg HostConfig

	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &cfg); err == nil && cfg.UUID != "" {
			onDone(&cfg)
			return
		}
	}

	// fallback: prompt for name
	var name string
	fmt.Print("📝 Enter a name for this host: ")
	_, err = fmt.Scanln(&name)
	if err != nil || name == "" {
		log.Fatal("Invalid or empty host name")
	}

	uuid, authorID, err := registerHostWithRelay(name)
	if err != nil {
		log.Fatal("Failed to register host:", err)
	}

	uuid, authorID, err = registerHostWithRelay(name)
	if err != nil {
		log.Fatal("Failed to register host:", err)
	}

	cfg = HostConfig{
		UUID:     uuid,
		AuthorID: authorID,
		Name:     name,
		DBFile:   dbPath,
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(configPath, data, 0644)

	onDone(&cfg)
}
