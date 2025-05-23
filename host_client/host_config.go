package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/andlabs/ui"
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

func LoadOrInitHostConfigGUI(onDone func(*HostConfig)) {
	go func() {
		configPath, err := getAppSupportPathFor(configFileName)
		if err != nil {
			showErrorAndQuit("Failed to resolve config path: " + err.Error())
			return
		}
		dbPath, err := getAppSupportPathFor(dbName)
		if err != nil {
			showErrorAndQuit("Failed to resolve DB path: " + err.Error())
			return
		}

		if data, err := os.ReadFile(configPath); err == nil {
			var cfg HostConfig
			if json.Unmarshal(data, &cfg) == nil && cfg.UUID != "" {
				ui.QueueMain(func() { onDone(&cfg) })
				return
			}
		}

		// No valid config: show prompt
		ui.QueueMain(func() {
			input := ui.NewEntry()
			window := ui.NewWindow("Enter Host Name", 300, 100, false)
			box := ui.NewVerticalBox()
			box.Append(ui.NewLabel("Name for this host:"), false)
			box.Append(input, false)

			submit := ui.NewButton("Submit")
			box.Append(submit, false)

			submit.OnClicked(func(*ui.Button) {
				name := input.Text()
				if name == "" {
					ui.MsgBoxError(window, "Error", "Host name cannot be empty")
					return
				}

				go func() {
					uuid, authorID, err := registerHostWithRelay(name)
					if err != nil {
						ui.QueueMain(func() {
							ui.MsgBoxError(window, "Error", "Failed to register host: "+err.Error())
						})
						return
					}

					cfg := &HostConfig{
						UUID:     uuid,
						AuthorID: authorID,
						Name:     name,
						DBFile:   dbPath,
					}
					data, _ := json.MarshalIndent(cfg, "", "  ")
					_ = os.WriteFile(configPath, data, 0644)

					ui.QueueMain(func() {
						window.Hide()
						onDone(cfg)
					})
				}()
			})

			window.SetChild(box)
			window.OnClosing(func(*ui.Window) bool {
				ui.Quit()
				return true
			})
			window.Show()
			bringToFront()
		})
	}()
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
