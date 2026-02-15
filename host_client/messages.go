package main

import (
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

	user, err := resolveHostUserIdentity(data.UserID, data.UserPublicKey, data.Username)
	if err != nil {
		log.Println("Error resolving message user identity:", err)
		return
	}

	query := `INSERT INTO messages (channel_uuid, content, user_id, timestamp) VALUES (?, ?, ?, ?)`
	_, err = db.ChatDB.Exec(query, data.ChannelUUID, data.Content, user.ID, msgTimestamp.Format(time.RFC3339))
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

	const messageRequestSize = 50

	rows, err := db.ChatDB.Query(`
		SELECT id, channel_uuid, content, user_id, timestamp
		FROM messages
		WHERE channel_uuid = ? AND timestamp < ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, data.ChannelUUID, data.BeforeUnixTime, messageRequestSize+1) // Get one extra to check if there are more
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

	if len(messages) > 0 {
		// Deduplicate user_ids
		var userIDs []int
		for uid := range userIDSet {
			if uid <= 0 {
				continue
			}
			userIDs = append(userIDs, uid)
		}

		if len(userIDs) > 0 {
			users, err := lookupHostUsersByIDs(userIDs)
			if err != nil {
				log.Println("Error fetching user info from host DB:", err)
				return
			}

			// Create user_id → user map
			userMap := make(map[int]DashDataUser)
			for _, user := range users {
				userMap[user.ID] = user
			}

			// Assign usernames/public keys
			for i := range messages {
				if user, ok := userMap[messages[i].UserID]; ok {
					messages[i].Username = user.Username
					messages[i].UserPublicKey = user.PublicKey
				}
			}
		}
	}

	hasMoreMessages := false

	if len(messages) > messageRequestSize {
		hasMoreMessages = true
		messages = messages[:messageRequestSize] // Reduce by one
	}

	// Reverse so messages are sent oldest → newest
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	// Send response
	sendToConn(conn, WSMessage{
		Type: "get_messages_response",
		Data: GetMessagesResponse{
			Messages:        messages,
			HasMoreMessages: hasMoreMessages,
			ChannelUUID:     data.ChannelUUID,
			ClientUUID:      data.ClientUUID,
		},
	})
}
