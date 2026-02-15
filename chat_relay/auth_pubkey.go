package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/gorilla/websocket"
)

func newAuthChallenge() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func authMessage(hostUUID, challenge string) string {
	return fmt.Sprintf("parch-chat-auth:%s:%s", hostUUID, challenge)
}

func normalizeUsername(username string, publicKey string) string {
	name := strings.TrimSpace(username)
	if name == "" {
		suffix := publicKey
		if len(suffix) > 8 {
			suffix = suffix[:8]
		}
		name = "user-" + suffix
	}
	if len(name) > 40 {
		name = name[:40]
	}
	return name
}

func deriveSessionUserID(publicKey string) int {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(publicKey))
	userID := int(hasher.Sum32() & 0x7fffffff)
	if userID == 0 {
		userID = 1
	}
	return userID
}

func handleAuthPubKey(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[AuthPubKeyClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Invalid auth payload"}})
		return
	}

	if strings.TrimSpace(data.PublicKey) == "" || strings.TrimSpace(data.Signature) == "" {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Missing public key or signature"}})
		return
	}

	if data.Challenge == "" || client.AuthChallenge == "" || data.Challenge != client.AuthChallenge {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Invalid auth challenge"}})
		return
	}

	publicKeyBytes, err := base64.RawStdEncoding.DecodeString(data.PublicKey)
	if err != nil || len(publicKeyBytes) != ed25519.PublicKeySize {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Invalid public key format"}})
		return
	}

	signatureBytes, err := base64.RawStdEncoding.DecodeString(data.Signature)
	if err != nil || len(signatureBytes) != ed25519.SignatureSize {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Invalid signature format"}})
		return
	}

	message := authMessage(client.HostUUID, data.Challenge)
	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), []byte(message), signatureBytes) {
		safeSend(client, conn, WSMessage{Type: "authentication-error", Data: ChatError{Content: "Invalid signature"}})
		return
	}

	userID := deriveSessionUserID(data.PublicKey)
	username := normalizeUsername(data.Username, data.PublicKey)

	host, exists := GetHost(client.HostUUID)
	if !exists {
		safeSend(client, conn, WSMessage{Type: "author_error", Data: ChatError{Content: "Failed to connect to host"}})
		return
	}

	host.mu.Lock()
	client.UserID = userID
	client.Username = username
	client.PublicKey = data.PublicKey
	if userID > 0 {
		host.ClientsByUserID[userID] = client
	}
	host.ClientsByPublicKey[data.PublicKey] = client
	host.mu.Unlock()

	if !client.IsAuthenticated {
		client.IsAuthenticated = true
		RegisterAuthenticatedIP(client.IP)
	}

	client.AuthChallenge = newAuthChallenge()
	safeSend(client, conn, WSMessage{
		Type: "auth_pubkey_success",
		Data: AuthPubKeySuccess{
			UserID:    userID,
			Username:  username,
			PublicKey: data.PublicKey,
		},
	})
}

func handleUpdateUsername(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[UpdateUsernameClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid update username data"}})
		return
	}

	newName := normalizeUsername(data.Username, client.PublicKey)
	client.Username = newName
	SendToAuthor(client, WSMessage{
		Type: "update_username_request",
		Data: UpdateUsernameRequest{
			UserID:        data.UserID,
			UserPublicKey: firstNonEmpty(data.UserPublicKey, client.PublicKey),
			Username:      newName,
			ClientUUID:    client.ClientUUID,
		},
	})
}

func handleUpdateUsernameRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[UpdateUsernameResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid update username response"}})
		return
	}

	host, exists := GetHost(client.HostUUID)
	if exists {
		host.mu.Lock()
		joinConn, ok := host.ClientConnsByUUID[data.ClientUUID]
		if ok {
			joinClient := host.ClientsByConn[joinConn]
			if joinClient != nil {
				if joinClient.UserID > 0 {
					delete(host.ClientsByUserID, joinClient.UserID)
				}
				if joinClient.PublicKey != "" {
					delete(host.ClientsByPublicKey, joinClient.PublicKey)
				}
				joinClient.UserID = data.UserID
				joinClient.PublicKey = firstNonEmpty(data.UserPublicKey, joinClient.PublicKey)
				joinClient.Username = data.Username
				if joinClient.UserID > 0 {
					host.ClientsByUserID[joinClient.UserID] = joinClient
				}
				if joinClient.PublicKey != "" {
					host.ClientsByPublicKey[joinClient.PublicKey] = joinClient
				}
			}
		}
		host.mu.Unlock()
	}

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "update_username_success",
		Data: UpdateUsernameSuccess{
			UserID:        data.UserID,
			UserPublicKey: data.UserPublicKey,
			Username:      data.Username,
		},
	})
}
