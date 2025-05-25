// //go:build !windows

// package main

// import (
// 	"encoding/json"
// 	"os"

// 	"github.com/andlabs/ui"
// )

// func LoadOrInitHostConfigGUI(onDone func(*HostConfig)) {
// 	go func() {
// 		configPath, err := getAppSupportPathFor(configFileName)
// 		if err != nil {
// 			showErrorAndQuit("Failed to resolve config path: " + err.Error())
// 			return
// 		}
// 		dbPath, err := getAppSupportPathFor(dbName)
// 		if err != nil {
// 			showErrorAndQuit("Failed to resolve DB path: " + err.Error())
// 			return
// 		}

// 		if data, err := os.ReadFile(configPath); err == nil {
// 			var cfg HostConfig
// 			if json.Unmarshal(data, &cfg) == nil && cfg.UUID != "" {
// 				ui.QueueMain(func() { onDone(&cfg) })
// 				return
// 			}
// 		}

// 		// No valid config: show prompt
// 		ui.QueueMain(func() {
// 			input := ui.NewEntry()
// 			window := ui.NewWindow("Enter Host Name", 300, 100, false)
// 			box := ui.NewVerticalBox()
// 			box.Append(ui.NewLabel("Name for this host:"), false)
// 			box.Append(input, false)

// 			submit := ui.NewButton("Submit")
// 			box.Append(submit, false)

// 			submit.OnClicked(func(*ui.Button) {
// 				name := input.Text()
// 				if name == "" {
// 					ui.MsgBoxError(window, "Error", "Host name cannot be empty")
// 					return
// 				}

// 				go func() {
// 					uuid, authorID, err := registerHostWithRelay(name)
// 					if err != nil {
// 						ui.QueueMain(func() {
// 							ui.MsgBoxError(window, "Error", "Failed to register host: "+err.Error())
// 						})
// 						return
// 					}

// 					cfg := &HostConfig{
// 						UUID:     uuid,
// 						AuthorID: authorID,
// 						Name:     name,
// 						DBFile:   dbPath,
// 					}
// 					data, _ := json.MarshalIndent(cfg, "", "  ")
// 					_ = os.WriteFile(configPath, data, 0644)

// 					ui.QueueMain(func() {
// 						window.Hide()
// 						onDone(cfg)
// 					})
// 				}()
// 			})

// 			window.SetChild(box)
// 			window.OnClosing(func(*ui.Window) bool {
// 				ui.Quit()
// 				return true
// 			})
// 			window.Show()
// 			bringToFront()
// 		})
// 	}()
// }
