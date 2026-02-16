package main

import (
	"encoding/json"
	"fmt"
	"gochat/db"
	"log"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const maxPersistedEnvelopeBytes = 128 * 1024

func handleSaveChatMessage(wsMsg *WSMessage) {
	data, err := decodeData[SaveChatMessageRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding invite_user_request:", err)
		return
	}

	msgTimestamp := time.Now().UTC()

	user, err := resolveHostUserIdentity(data.UserID, data.UserPublicKey, data.UserEncPublicKey, data.Username)
	if err != nil {
		log.Println("Error resolving message user identity:", err)
		return
	}
	messageID := strings.TrimSpace(stringField(data.Envelope, "message_id"))
	senderAuthPublicKey := strings.TrimSpace(stringField(data.Envelope, "sender_auth_public_key"))
	if messageID == "" || senderAuthPublicKey == "" {
		log.Println("Rejecting message without replay-protection fields")
		return
	}
	if user.PublicKey != "" && senderAuthPublicKey != user.PublicKey {
		log.Println("Rejecting message with mismatched sender auth key")
		return
	}

	query := `INSERT INTO messages (channel_uuid, content, user_id, message_id, sender_auth_public_key, timestamp) VALUES (?, ?, ?, ?, ?, ?)`
	envelopeJSON, err := json.Marshal(data.Envelope)
	if err != nil {
		log.Println("Error marshalling encrypted message envelope:", err)
		return
	}
	if len(envelopeJSON) > maxPersistedEnvelopeBytes {
		log.Printf("Rejecting oversized encrypted envelope: %d bytes", len(envelopeJSON))
		return
	}

	_, err = db.ChatDB.Exec(
		query,
		data.ChannelUUID,
		string(envelopeJSON),
		user.ID,
		messageID,
		senderAuthPublicKey,
		msgTimestamp.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			log.Printf("Duplicate message replay ignored for channel %s message_id=%s", data.ChannelUUID, messageID)
			return
		}
		fmt.Println("Error: Database failed to insert message")
	}
}

func stringField(values map[string]interface{}, key string) string {
	if len(values) == 0 {
		return ""
	}
	raw, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := raw.(string)
	if !ok {
		return ""
	}
	return text
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
		var envelopeRaw string
		err = rows.Scan(&msg.ID, &msg.ChannelUUID, &envelopeRaw, &msg.UserID, &msg.Timestamp)
		if err != nil {
			log.Println("Error scanning message:", err)
			continue
		}
		if envelopeRaw != "" {
			if err := json.Unmarshal([]byte(envelopeRaw), &msg.Envelope); err != nil {
				log.Println("Error unmarshalling encrypted message envelope:", err)
				continue
			}
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
					messages[i].UserEncPublicKey = user.EncPublicKey
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
