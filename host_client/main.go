package main

import (
	"context"
	"embed"
	"gochat/db"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/andlabs/ui"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var MigrationFiles embed.FS

func main() {
	ui.Main(setupUI)
}

func runMainLogic(cfg *HostConfig) {
	var err error

	db.ChatDB, err = db.InitDB(cfg.DBFile, MigrationFiles, "migrations")
	if err != nil {
		log.Println("Error opening database:", err)
		return
	}
	defer db.CloseDB(db.ChatDB)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := SocketClient(ctx, cfg.UUID, cfg.AuthorID)
		if err != nil {
			log.Println("SocketClient error:", err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs

	log.Println("Interrupt received, shutting down...")
	cancel()

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
		multiline := ui.NewMultilineEntry()
		multiline.SetReadOnly(true)

		win := ui.NewWindow("Parch Host", 600, 400, true)
		box := ui.NewVerticalBox()
		box.Append(ui.NewLabel("Parch Host Logs:"), false)
		box.Append(multiline, true)

		win.SetChild(box)
		win.OnClosing(func(*ui.Window) bool {
			ui.Quit()
			return true
		})
		win.Show()
		bringToFront()

		log.SetOutput(logWriter{multiline})
		log.Println("Startup complete. Connecting...")

		go runMainLogic(cfg)
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
