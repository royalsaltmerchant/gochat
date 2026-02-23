package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"

	"github.com/gorilla/websocket"
)

func hostAuthMessage(hostUUID, challenge string) string {
	return fmt.Sprintf("parch-host-auth:%s:%s", hostUUID, challenge)
}

func handleHostAuth(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[HostAuthClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Invalid host auth payload"}})
		return
	}
	if !client.IsHostCandidate {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Host auth not allowed for this connection"}})
		return
	}
	if data.Challenge == "" || client.HostAuthChallenge == "" || data.Challenge != client.HostAuthChallenge {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Invalid host auth challenge"}})
		return
	}

	signatureBytes, err := base64.RawStdEncoding.DecodeString(data.Signature)
	if err != nil || len(signatureBytes) != ed25519.SignatureSize {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Invalid host auth signature format"}})
		return
	}

	host, exists := GetHost(client.HostUUID)
	if !exists {
		safeSend(client, conn, WSMessage{Type: "author_error", Data: ChatError{Content: "Failed to connect to host"}})
		return
	}

	publicKey, err := parseHostSigningPublicKey(host.SigningPublicKey)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Host signing key is invalid"}})
		return
	}
	if !ed25519.Verify(publicKey, []byte(hostAuthMessage(client.HostUUID, data.Challenge)), signatureBytes) {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Invalid host auth signature"}})
		return
	}

	host.mu.Lock()
	if host.AuthorConn != nil && host.AuthorConn != conn {
		if prev, ok := host.ClientsByConn[host.AuthorConn]; ok && prev != nil {
			prev.IsHostAuthor = false
		}
	}
	host.AuthorConn = conn
	client.IsHostAuthor = true
	client.HostAuthChallenge = newAuthChallenge()
	host.mu.Unlock()

	HandleUpdateHostOnline(host.UUID)
	safeSend(client, conn, WSMessage{
		Type: "host_auth_success",
		Data: "Host authenticated",
	})
}
