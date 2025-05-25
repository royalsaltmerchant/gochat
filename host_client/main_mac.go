// //go:build !windows

// package main

// import (
// 	"context"
// 	"embed"
// 	"log"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"strings"
// 	"time"

// 	"github.com/andlabs/ui"
// 	_ "github.com/mattn/go-sqlite3"
// )

// //go:embed migrations/*.sql
// var MigrationFiles embed.FS

// func main() {
// 	ui.Main(setupUI)
// }

// func setupUI() {
// 	LoadOrInitHostConfigGUI(func(cfg *HostConfig) {
// 		ctx, cancel := context.WithCancel(context.Background())

// 		win := ui.NewWindow("Parch Host", 600, 400, true)

// 		hostLabel := ui.NewLabel("Host Key:")

// 		hostKey := ui.NewLabel(cfg.UUID)

// 		copyBtn := ui.NewButton("Copy")
// 		copyBtn.OnClicked(func(*ui.Button) {
// 			copyToClipboard(cfg.UUID)
// 		})

// 		hostKeyRow := ui.NewHorizontalBox()
// 		hostKeyRow.Append(hostKey, true)
// 		hostKeyRow.Append(copyBtn, false)

// 		hostKeyBox := ui.NewVerticalBox()
// 		hostKeyBox.Append(hostLabel, false)
// 		hostKeyBox.Append(hostKeyRow, false)
// 		hostKeyBox.Append(ui.NewHorizontalSeparator(), false)

// 		logHeader := ui.NewLabel("Parch Host Logs:")

// 		multiline := ui.NewMultilineEntry()
// 		multiline.SetReadOnly(true)

// 		logBox := ui.NewVerticalBox()
// 		logBox.Append(logHeader, false)
// 		logBox.Append(multiline, true)

// 		container := ui.NewVerticalBox()

// 		// Add vertical spacing by inserting empty labels
// 		container.Append(ui.NewLabel(" "), false)
// 		container.Append(hostKeyBox, false)
// 		container.Append(ui.NewLabel(" "), false)
// 		container.Append(logBox, true)

// 		win.SetChild(container)

// 		win.OnClosing(func(*ui.Window) bool {
// 			log.Println("Closing window, sending shutdown signal...")
// 			cancel() // cancel context
// 			go func() {
// 				time.Sleep(1500 * time.Millisecond) // give time for shutdown
// 				ui.Quit()
// 			}()
// 			return true // allow window to close
// 		})

// 		win.Show()
// 		bringToFront()

// 		log.SetOutput(logWriter{multiline})
// 		log.Println("Startup complete. Connecting...")

// 		go runMainLogic(ctx, cfg)
// 	})
// }

// func bringToFront() {
// 	appPath := os.Args[0]
// 	appName := strings.TrimSuffix(filepath.Base(appPath), filepath.Ext(appPath))
// 	_ = exec.Command("osascript", "-e", `tell application "System Events" to set frontmost of process "`+appName+`" to true`).Run()
// }

// type logWriter struct {
// 	box *ui.MultilineEntry
// }

// func (w logWriter) Write(p []byte) (n int, err error) {
// 	ui.QueueMain(func() {
// 		w.box.Append(string(p))
// 	})
// 	return len(p), nil
// }

// func showErrorAndQuit(msg string) {
// 	ui.QueueMain(func() {
// 		ui.MsgBoxError(nil, "Startup Error", msg)
// 		ui.Quit()
// 	})
// }

// func copyToClipboard(text string) {
// 	cmd := exec.Command("pbcopy")
// 	in, err := cmd.StdinPipe()
// 	if err != nil {
// 		log.Println("Clipboard error:", err)
// 		return
// 	}
// 	if err := cmd.Start(); err != nil {
// 		log.Println("Clipboard error:", err)
// 		return
// 	}
// 	_, _ = in.Write([]byte(text))
// 	_ = in.Close()
// 	_ = cmd.Wait()
// }
