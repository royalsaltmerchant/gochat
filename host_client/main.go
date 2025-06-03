package main

import (
	"context"
	"embed"
	"gochat/db"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/andlabs/ui"
	_ "github.com/mattn/go-sqlite3"

	// Windows build specific import for andlabs
	_ "github.com/andlabs/ui/winmanifest"
)

//go:embed migrations/*.sql
var MigrationFiles embed.FS

func main() {
	ui.Main(setupUI)
}

func runMainLogic(ctx context.Context, cfg *HostConfig) {
	var err error
	db.ChatDB, err = db.InitDB(cfg.DBFile, MigrationFiles, "migrations")
	if err != nil {
		log.Println("Error opening database:", err)
		return
	}
	defer db.CloseDB(db.ChatDB)

	go func() {
		err := SocketClient(ctx, cfg.UUID, cfg.AuthorID)
		if err != nil {
			log.Println("SocketClient error:", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")

	payload := map[string]string{
		"author_id": cfg.AuthorID,
	}
	resp, err := PostJSON(relayBaseURL.String()+"/api/host_offline/"+cfg.UUID, payload, nil)
	if err != nil {
		log.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	time.Sleep(1 * time.Second)
}

func setupUI() {
	LoadOrInitHostConfigGUI(func(cfg *HostConfig) {
		ctx, cancel := context.WithCancel(context.Background())

		win := ui.NewWindow("Parch Host", 600, 400, true)

		hostLabel := ui.NewLabel("Host Key:")

		hostKey := ui.NewLabel(cfg.UUID)

		copyBtn := ui.NewButton("Copy")
		copyBtn.OnClicked(func(*ui.Button) {
			Copy(cfg.UUID)
		})

		hostKeyRow := ui.NewHorizontalBox()
		hostKeyRow.Append(hostKey, true)
		hostKeyRow.Append(copyBtn, false)

		hostKeyBox := ui.NewVerticalBox()
		hostKeyBox.Append(hostLabel, false)
		hostKeyBox.Append(hostKeyRow, false)
		hostKeyBox.Append(ui.NewHorizontalSeparator(), false)

		logHeader := ui.NewLabel("Parch Host Logs:")

		multiline := ui.NewMultilineEntry()
		multiline.SetReadOnly(true)

		logBox := ui.NewVerticalBox()
		logBox.Append(logHeader, false)
		logBox.Append(multiline, true)

		container := ui.NewVerticalBox()

		// Add vertical spacing by inserting empty labels
		container.Append(ui.NewLabel(" "), false)
		container.Append(hostKeyBox, false)
		container.Append(ui.NewLabel(" "), false)
		container.Append(logBox, true)

		win.SetChild(container)

		win.OnClosing(func(*ui.Window) bool {
			log.Println("Closing window, sending shutdown signal...")
			cancel() // cancel context
			go func() {
				time.Sleep(1500 * time.Millisecond) // give time for shutdown
				ui.Quit()
			}()
			return true // allow window to close
		})

		win.Show()
		bringToFront()

		log.SetOutput(logWriter{multiline})
		log.Println("Startup complete. Connecting...")

		go runMainLogic(ctx, cfg)
	})
}

func bringToFront() {
	appPath := os.Args[0]
	appName := strings.TrimSuffix(filepath.Base(appPath), filepath.Ext(appPath))
	_ = exec.Command("osascript", "-e", `tell application "System Events" to set frontmost of process "`+appName+`" to true`).Run()
}

type logWriter struct {
	box *ui.MultilineEntry
}

func (w logWriter) Write(p []byte) (n int, err error) {
	ui.QueueMain(func() {
		w.box.Append(string(p))
	})
	return len(p), nil
}

func showErrorAndQuit(msg string) {
	ui.QueueMain(func() {
		ui.MsgBoxError(nil, "Startup Error", msg)
		ui.Quit()
	})
}
