package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"gochat/db"
	"log"
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

func upsertChatIdentity(publicKey, username string) (int, string, error) {
	resolvedUsername := normalizeUsername(username, publicKey)
	_, err := db.HostDB.Exec(
		`INSERT INTO chat_identities (public_key, username)
		 VALUES (?, ?)
		 ON CONFLICT(public_key) DO UPDATE SET
		   username = CASE
		     WHEN excluded.username <> '' THEN excluded.username
		     ELSE chat_identities.username
		   END,
		   updated_at = CURRENT_TIMESTAMP`,
		publicKey,
		resolvedUsername,
	)
	if err != nil {
		return 0, "", err
	}

	var userID int
	var currentUsername string
	err = db.HostDB.QueryRow(
		`SELECT id, username FROM chat_identities WHERE public_key = ?`,
		publicKey,
	).Scan(&userID, &currentUsername)
	if err != nil {
		return 0, "", err
	}
	return userID, currentUsername, nil
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

	userID, username, err := upsertChatIdentity(data.PublicKey, data.Username)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Failed to authenticate identity"}})
		return
	}

	host, exists := GetHost(client.HostUUID)
	if !exists {
		safeSend(client, conn, WSMessage{Type: "author_error", Data: ChatError{Content: "Failed to connect to host"}})
		return
	}

	host.mu.Lock()
	client.UserID = userID
	client.Username = username
	client.PublicKey = data.PublicKey
	host.ClientsByUserID[userID] = client
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
	data, err := decodeData[UpdateUsernameRequest](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid update username data"}})
		return
	}

	if data.UserID <= 0 || data.UserID != client.UserID {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Unauthorized username update"}})
		return
	}

	newName := normalizeUsername(data.Username, client.PublicKey)
	res, err := db.HostDB.Exec(`UPDATE chat_identities SET username = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newName, data.UserID)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Database error updating username"}})
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Identity not found"}})
		return
	}

	client.Username = newName
	safeSend(client, conn, WSMessage{
		Type: "update_username_success",
		Data: UpdateUsername{UserID: data.UserID, Username: newName},
	})
}

func lookupIdentityByPublicKey(publicKey string) (DashDataUser, error) {
	var user DashDataUser
	err := db.HostDB.QueryRow(`SELECT id, username, public_key FROM chat_identities WHERE public_key = ?`, publicKey).Scan(&user.ID, &user.Username, &user.PublicKey)
	if err != nil {
		return DashDataUser{}, err
	}
	return user, nil
}

func lookupIdentityByID(userID int) (DashDataUser, error) {
	var user DashDataUser
	err := db.HostDB.QueryRow(`SELECT id, username, public_key FROM chat_identities WHERE id = ?`, userID).Scan(&user.ID, &user.Username, &user.PublicKey)
	if err != nil {
		return DashDataUser{}, err
	}
	return user, nil
}

func lookupIdentitiesByIDs(userIDs []int) ([]DashDataUser, error) {
	if len(userIDs) == 0 {
		return []DashDataUser{}, nil
	}

	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`SELECT id, username, public_key FROM chat_identities WHERE id IN (%s)`, strings.Join(placeholders, ","))
	rows, err := db.HostDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]DashDataUser, 0, len(userIDs))
	for rows.Next() {
		var user DashDataUser
		if err := rows.Scan(&user.ID, &user.Username, &user.PublicKey); err != nil {
			log.Printf("lookupIdentitiesByIDs scan error: %v", err)
			continue
		}
		users = append(users, user)
	}

	return users, rows.Err()
}
