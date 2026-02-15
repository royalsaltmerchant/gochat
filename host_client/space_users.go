package main

import (
	"gochat/db"
	"log"

	"github.com/gorilla/websocket"
)

func handleInviteUser(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[InviteUserRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding invite_user_request:", err)
		return
	}

	user, err := upsertHostUserByPublicKey(data.PublicKey, "")
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Failed to resolve invite user identity",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	var spaceUser DashDataInvite

	// First, insert into space_users
	query := `
INSERT INTO space_users (space_uuid, user_id)
VALUES (?, ?)
RETURNING id, space_uuid, user_id, joined
`
	err = db.ChatDB.QueryRow(query, data.SpaceUUID, user.ID).Scan(
		&spaceUser.ID,
		&spaceUser.SpaceUUID,
		&spaceUser.UserID,
		&spaceUser.Joined,
	)
	if err != nil {
		log.Println("Insert error:", err)
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error inserting space_user",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	spaceUser.UserPublicKey = user.PublicKey

	// Then, query for space name
	query = `SELECT name FROM spaces WHERE uuid = ?`
	err = db.ChatDB.QueryRow(query, spaceUser.SpaceUUID).Scan(&spaceUser.Name)
	if err != nil {
		log.Println("Space name lookup error:", err)
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error fetching space name",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	sendToConn(conn, WSMessage{
		Type: "invite_user_success",
		Data: InviteUserResponse{
			PublicKey:     data.PublicKey,
			UserID:        user.ID,
			UserPublicKey: user.PublicKey,
			SpaceUUID:     data.SpaceUUID,
			Invite:        spaceUser,
			ClientUUID:    data.ClientUUID,
		},
	})
}

func handleRemoveSpaceUser(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[RemoveSpaceUserRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding invite_user_request:", err)
		return
	}

	user, err := resolveHostUserIdentityStrict(data.UserID, data.UserPublicKey)
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

	res, err := db.ChatDB.Exec("DELETE FROM space_users WHERE space_uuid = ? AND user_id = ?", data.SpaceUUID, user.ID)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Space User not found in database",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Space User failed to be removed in database",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	data.UserID = user.ID
	data.UserPublicKey = user.PublicKey

	sendToConn(conn, WSMessage{
		Type: "remove_space_user_success",
		Data: data,
	})
}

func handleLeaveSpace(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[LeaveSpaceRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding invite_user_request:", err)
		return
	}

	user, err := resolveHostUserIdentityStrict(data.UserID, data.UserPublicKey)
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

	res, err := db.ChatDB.Exec("DELETE FROM space_users WHERE space_uuid = ? AND user_id = ?", data.SpaceUUID, user.ID)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database failed to remove space user",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Space user not found in database",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	data.UserID = user.ID
	data.UserPublicKey = user.PublicKey

	sendToConn(conn, WSMessage{
		Type: "leave_space_success",
		Data: data,
	})
}
