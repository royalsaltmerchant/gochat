package main

import (
	"context"
	"fmt"
)

type App struct {
	ctx          context.Context
	HostManager  *HostManager
	TokenManager *AuthTokenManager
}

func NewApp() *App {
	hostMgr, err := NewHostManager()
	if err != nil {
		panic("Failed to create host manager: " + err.Error())
	}

	tokenMgr, err := NewAuthTokenManager()
	if err != nil {
		panic("Failed to create token manager: " + err.Error())
	}

	return &App{
		HostManager:  hostMgr,
		TokenManager: tokenMgr,
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

func (a *App) SaveAuthToken(hostUUID, token string) error {
	return a.TokenManager.SaveAuthToken(hostUUID, token)
}

func (a *App) LoadAuthToken(hostUUID string) (string, error) {
	return a.TokenManager.LoadAuthToken(hostUUID)
}
func (a *App) RemoveAuthToken(hostUUID string) error {
	return a.TokenManager.RemoveAuthToken(hostUUID)
}
