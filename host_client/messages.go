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

	query := `INSERT INTO messages (channel_uuid, content, user_id, timestamp) VALUES (?, ?, ?, ?)`
	_, err = db.ChatDB.Exec(query, data.ChannelUUID, data.Content, data.UserID, msgTimestamp.Format(time.RFC3339))
	if err != nil {
		fmt.Println("Error: Database failed to insert message")
	}
}

func handleGetMessages(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[GetMessagesRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding invite_user_request:", err)
		return
	}

	// Get channel messages limit by 100 for now maybe paginate?
	rows, err := db.ChatDB.Query(`
	SELECT m.id, m.channel_uuid, m.content, m.user_id, m.timestamp, u.username
	FROM messages m
	JOIN users u ON u.id = m.user_id
	WHERE m.channel_uuid = ?
	ORDER BY m.timestamp ASC
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
	var username string
	for rows.Next() {
		var message GetMessagesMessage
		err = rows.Scan(&message.ID, &message.ChannelUUID, &message.Content, &message.UserID, &message.Timestamp, &username)
		if err != nil {
			fmt.Println("Error scanning message:", err)
			continue
		}
		message.Username = username
		messages = append(messages, message)
	}
	if err = rows.Err(); err != nil {
		fmt.Println("Row iteration error:", err)
	}

	sendToConn(conn, WSMessage{
		Type: "get_messages_response",
		Data: GetMessagesResponse{
			Messages:    messages,
			ChannelUUID: data.ChannelUUID,
			ClientUUID:  data.ClientUUID,
		},
	})
}
