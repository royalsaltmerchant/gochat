package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// Helper to decode WSMessage.Data into a typed struct
func decodeData[T any](raw interface{}) (T, error) {
	var data T
	bytes, err := json.Marshal(raw)
	if err != nil {
		return data, err
	}
	err = json.Unmarshal(bytes, &data)
	return data, err
}

// Entry point for the client
func SocketClient(ctx context.Context, hostUUID string, authorID string) error {
	runClientLoop(ctx, hostUUID, authorID)
	return nil
}

// Reconnect loop
func runClientLoop(ctx context.Context, hostUUID string, authorID string) {

	for {
		select {
		case <-ctx.Done():
			log.Println("SocketClient: context cancelled before connection")
			return
		default:
			log.Printf("Attempting to connect to %s...", wsRelayURL.String())
			conn, _, err := websocket.DefaultDialer.Dial(wsRelayURL.String(), nil)
			if err != nil {
				log.Printf("Connection failed: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}
			log.Println("Connected successfully")

			if err := joinHost(conn, hostUUID, authorID); err != nil {
				log.Println("Failed to join host:", err)
				conn.Close()
				time.Sleep(2 * time.Second)
				continue
			}

			if err := handleSocketMessages(ctx, conn); err != nil {
				log.Printf("Socket closed: %v", err)
			}
			conn.Close()

			log.Println("Reconnecting in 2 seconds...")
			time.Sleep(2 * time.Second)
		}
	}
}

// Read loop
func handleSocketMessages(ctx context.Context, conn *websocket.Conn) error {
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, stopping socket read loop")
			return nil
		default:
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				return err
			}

			var wsMsg WSMessage
			if err := json.Unmarshal(msgBytes, &wsMsg); err != nil {
				log.Println("Invalid WSMessage JSON:", err)
				continue
			}
			log.Println(wsMsg.Type)
			switch wsMsg.Type {
			case "join_ack":
				data, err := decodeData[string](wsMsg.Data)
				if err != nil {
					log.Println("Error decoding join_ack:", err)
					continue
				}
				log.Println("join_ack:", data)
			case "auth_challenge":
				// Host author sessions do not use pubkey auth challenges.
				continue
			case "relay_health_check":
				data, err := decodeData[RelayHealthCheck](wsMsg.Data)
				if err != nil || data.Nonce == "" {
					continue
				}
				sendToConn(conn, WSMessage{
					Type: "relay_health_check_ack",
					Data: RelayHealthCheckAck{Nonce: data.Nonce},
				})
			case "update_username_request":
				handleUpdateUsername(conn, &wsMsg)
			case "get_dash_data_request":
				handleGetDashData(conn, &wsMsg)
			case "create_space_request":
				handleCreateSpace(conn, &wsMsg)
			case "delete_space_request":
				handleDeleteSpace(conn, &wsMsg)
			case "create_channel_request":
				handleCreateChannel(conn, &wsMsg)
			case "delete_channel_request":
				handleDeleteChannel(conn, &wsMsg)
			case "invite_user_request":
				handleInviteUser(conn, &wsMsg)
			case "accept_invite_request":
				handleAcceptInvite(conn, &wsMsg)
			case "decline_invite_request":
				handleDeclineInvite(conn, &wsMsg)
			case "leave_space_request":
				handleLeaveSpace(conn, &wsMsg)
			case "remove_space_user_request":
				handleRemoveSpaceUser(conn, &wsMsg)
			case "save_chat_message_request":
				handleSaveChatMessage(&wsMsg)
			case "get_messages_request":
				handleGetMessages(conn, &wsMsg)
			case "channel_allow_voice_request":
				handleChannelAllowVoice(conn, &wsMsg)

			default:
				log.Println("Unhandled message type:", wsMsg.Type)
			}
		}
	}
}

func sendToConn(conn *websocket.Conn, msg WSMessage) {
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("SendToClient: failed to send message to client: %v\n", err)
	}
}

// Join host message
func joinHost(conn *websocket.Conn, hostUUID string, authorID string) error {
	payload := WSMessage{
		Type: "join_host",
		Data: JoinHostPayload{
			UUID: hostUUID,
			ID:   authorID,
		},
	}
	msgBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, msgBytes)
}
