package main

import (
	"encoding/json"
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

	payload := map[string]string{"email": data.Email}
	resp, err := PostJSON(relayBaseURL.String()+"/api/user_by_email", payload, nil)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Failed to reach user lookup API",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 400 {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "User not found by email",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	var user DashDataUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		log.Println("Error decoding user_by_id response:", err)
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
			Email:      data.Email,
			UserID:     user.ID,
			SpaceUUID:  data.SpaceUUID,
			Invite:     spaceUser,
			ClientUUID: data.ClientUUID,
		},
	})
}

func handleRemoveSpaceUser(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[RemoveSpaceUserRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding invite_user_request:", err)
		return
	}

	res, err := db.ChatDB.Exec("DELETE FROM space_users WHERE space_uuid = ? AND user_id = ?", data.SpaceUUID, data.UserID)
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

	res, err := db.ChatDB.Exec("DELETE FROM space_users WHERE space_uuid = ? AND user_id = ?", data.SpaceUUID, data.UserID)
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

	sendToConn(conn, WSMessage{
		Type: "leave_space_success",
		Data: data,
	})
}
