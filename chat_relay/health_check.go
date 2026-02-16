package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	hostHealthChecksMu sync.Mutex
	hostHealthChecks   = make(map[string]chan struct{})
)

func registerHostHealthCheck(nonce string) chan struct{} {
	hostHealthChecksMu.Lock()
	defer hostHealthChecksMu.Unlock()
	ch := make(chan struct{}, 1)
	hostHealthChecks[nonce] = ch
	return ch
}

func popHostHealthCheck(nonce string) (chan struct{}, bool) {
	hostHealthChecksMu.Lock()
	defer hostHealthChecksMu.Unlock()
	ch, ok := hostHealthChecks[nonce]
	if ok {
		delete(hostHealthChecks, nonce)
	}
	return ch, ok
}

func resolveHostHealthCheck(nonce string) {
	if ch, ok := popHostHealthCheck(nonce); ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func ensureHostResponsive(host *Host, timeout time.Duration) error {
	host.mu.Lock()
	authorConn, ok := host.ConnByAuthorID[host.AuthorID]
	if !ok {
		host.mu.Unlock()
		return fmt.Errorf("host author is offline")
	}
	authorClient := host.ClientsByConn[authorConn]
	host.mu.Unlock()
	if authorClient == nil {
		return fmt.Errorf("host author session missing")
	}

	nonce := uuid.NewString()
	done := registerHostHealthCheck(nonce)

	safeSend(authorClient, authorConn, WSMessage{
		Type: "relay_health_check",
		Data: RelayHealthCheck{Nonce: nonce},
	})

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		popHostHealthCheck(nonce)
		return fmt.Errorf("host author is unresponsive")
	}
}

func handleRelayHealthCheckAck(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[RelayHealthCheckAck](wsMsg.Data)
	if err != nil {
		return
	}
	if data.Nonce == "" {
		return
	}
	resolveHostHealthCheck(data.Nonce)
}
