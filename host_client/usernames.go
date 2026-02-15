package main

import (
	"gochat/db"
	"log"

	"github.com/gorilla/websocket"
)

func handleUpdateUsername(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[UpdateUsernameRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding update_username_request:", err)
		return
	}

	user, err := resolveHostUserIdentity(data.UserID, data.UserPublicKey, data.UserEncPublicKey, data.Username)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Failed to resolve user identity",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	newName := normalizeUsername(data.Username, user.PublicKey)
	res, err := db.ChatDB.Exec(
		`UPDATE chat_users SET username = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		newName,
		user.ID,
	)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error updating username",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Identity not found",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	sendToConn(conn, WSMessage{
		Type: "update_username_response",
		Data: UpdateUsernameResponse{
			UserID:           user.ID,
			UserPublicKey:    user.PublicKey,
			UserEncPublicKey: user.EncPublicKey,
			Username:         newName,
			ClientUUID:       data.ClientUUID,
		},
	})
}
