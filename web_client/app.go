package main

import (
	"context"
	"fmt"
)

type App struct {
	ctx         context.Context
	HostManager *HostManager
}

func NewApp() *App {
	return &App{
		HostManager: NewHostManager("hosts.json"),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// Forward HostManager methods directly

func (a *App) GetHosts() ([]Host, error) {
	return a.HostManager.GetHosts()
}

func (a *App) VerifyHostKey(hostUUID string) (string, error) {
	return a.HostManager.VerifyHostKey(hostUUID)
}
