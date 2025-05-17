package main

import (
	"database/sql"
	"encoding/json"
	"gochat/db"
	"log"

	"github.com/gorilla/websocket"
)

func handleAcceptInvite(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[AcceptInviteRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding create_space_request:", err)
		return
	}

	// Get space user
	var spaceUUID string
	query := `UPDATE space_users SET joined = 1 WHERE id = ? AND user_id = ? RETURNING space_uuid` // Checking by user_id also ensures they are authorized
	err = db.ChatDB.QueryRow(query, data.SpaceUserID, data.UserID).Scan(&spaceUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{
					Content:    "Space user not found by id",
					ClientUUID: data.ClientUUID,
				},
			})
			return
		} else {
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{
					Content:    "Database failed to query space user",
					ClientUUID: data.ClientUUID,
				},
			})
		}
	}

	var space DashDataSpace
	query = `SELECT * FROM spaces WHERE uuid = ?`
	err = db.ChatDB.QueryRow(query, spaceUUID).Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{
					Content:    "Space not found by uuid",
					ClientUUID: data.ClientUUID,
				},
			})
			return
		} else {
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{
					Content:    "Database failed to query space",
					ClientUUID: data.ClientUUID,
				},
			})
		}
	}

	payload := map[string]int{
		"user_id": data.UserID,
	}

	resp, err := PostJSON(relayBaseURL.String()+"/api/user_by_id", payload, nil)
	if err != nil {
		log.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	var user DashDataUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		log.Println("Error decoding user list:", err)
		return
	}

	AppendspaceChannelsAndUsers(&space)

	sendToConn(conn, WSMessage{
		Type: "accept_invite_success",
		Data: AcceptInviteResponse{
			SpaceUserID: data.SpaceUserID,
			User:        user,
			Space:       space,
			ClientUUID:  data.ClientUUID,
		},
	})
}

func handleDeclineInvite(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[DeclineInviteRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding create_space_request:", err)
		return
	}

	res, err := db.ChatDB.Exec(`DELETE FROM space_users WHERE id = ? AND user_id = ?`, data.SpaceUserID, data.UserID) // Checking by user_id also ensures they are authorized
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database failed to detele space user",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Failed to find space user by id and user id",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	sendToConn(conn, WSMessage{
		Type: "decline_invite_success",
		Data: data,
	})
}
