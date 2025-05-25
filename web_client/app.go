package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx          context.Context
	HostManager  *HostManager
	TokenManager *AuthTokenManager
	relayBaseURL string
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
		relayBaseURL: "https://parch.julianranieri.com",
		HostManager:  hostMgr,
		TokenManager: tokenMgr,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) Alert(message string) {
	runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   "Parch",
		Message: message,
	})
}

func (a *App) Confirm(message string) (bool, error) {
	result, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Confirm",
		Message:       message,
		Buttons:       []string{"Yes", "No"},
		DefaultButton: "Yes",
	})
	if err != nil {
		return false, err
	}
	return result == "Yes", nil
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) OpenInBrowser(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// Forward HostManager methods directly

func (a *App) GetHosts() ([]Host, error) {
	return a.HostManager.GetHosts()
}

func (a *App) VerifyHostKey(hostUUID string) (interface{}, error) {
	return a.HostManager.VerifyHostKey(hostUUID, a.relayBaseURL)
}

func (a *App) RemoveHost(hostUUID string) error {
	return a.HostManager.RemoveHost(hostUUID)
}

// Updated TokenManager methods for single-token storage

func (a *App) SaveAuthToken(token string) error {
	return a.TokenManager.SaveToken(token)
}

func (a *App) LoadAuthToken() (string, error) {
	return a.TokenManager.LoadToken()
}

func (a *App) RemoveAuthToken() error {
	return a.TokenManager.RemoveToken()
}
