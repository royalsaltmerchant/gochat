package main

import (
	"encoding/json"
	"fmt"
	"gochat/db"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

func handleSaveChatMessage(wsMsg *WSMessage) {
	data, err := decodeData[SaveChatMessageRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding invite_user_request:", err)
		return
	}

	msgTimestamp := time.Now().UTC()

	query := `INSERT INTO messages (channel_uuid, content, user_id, timestamp) VALUES (?, ?, ?, ?)`
	_, err = db.ChatDB.Exec(query, data.ChannelUUID, data.Content, data.UserID, msgTimestamp.Format(time.RFC3339))
	if err != nil {
		fmt.Println("Error: Database failed to insert message")
	}
}

func handleGetMessages(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[GetMessagesRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding get_messages_request:", err)
		return
	}

	rows, err := db.ChatDB.Query(`
		SELECT id, channel_uuid, content, user_id, timestamp
		FROM messages
		WHERE channel_uuid = ?
		ORDER BY timestamp ASC
		LIMIT 100
	`, data.ChannelUUID)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Messages not found in database",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	defer rows.Close()

	var messages []GetMessagesMessage
	userIDSet := make(map[int]struct{})

	for rows.Next() {
		var msg GetMessagesMessage
		err = rows.Scan(&msg.ID, &msg.ChannelUUID, &msg.Content, &msg.UserID, &msg.Timestamp)
		if err != nil {
			log.Println("Error scanning message:", err)
			continue
		}
		userIDSet[msg.UserID] = struct{}{}
		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		log.Println("Row iteration error:", err)
	}

	// Deduplicate user_ids
	var userIDs []int
	for uid := range userIDSet {
		userIDs = append(userIDs, uid)
	}

	// Make batch API request
	payload := map[string][]int{
		"user_ids": userIDs,
	}
	resp, err := PostJSON(relayBaseURL.String()+"/api/users_by_ids", payload, nil)
	if err != nil {
		log.Println("Error fetching user info:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 400 {
		log.Println("User not found by ID")
	}

	var users []DashDataUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		log.Println("Error decoding user list:", err)
		return
	}

	// Create user_id → username map
	userMap := make(map[int]string)
	for _, user := range users {
		userMap[user.ID] = user.Username
	}

	// Assign usernames
	for i := range messages {
		messages[i].Username = userMap[messages[i].UserID]
	}

	// Send response
	sendToConn(conn, WSMessage{
		Type: "get_messages_response",
		Data: GetMessagesResponse{
			Messages:    messages,
			ChannelUUID: data.ChannelUUID,
			ClientUUID:  data.ClientUUID,
		},
	})
}
