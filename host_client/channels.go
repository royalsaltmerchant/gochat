package main

import (
	"gochat/db"
	"log"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func handleCreateChannel(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[CreateChannelRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding create_channel_request:", err)
		return
	}

	// Get UUID and Author ID
	channelUUID := uuid.New()
	requester, err := resolveHostUserIdentityStrict(
		data.RequesterUserID,
		data.RequesterUserPublicKey,
		data.RequesterUserEncPublicKey,
	)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Failed to resolve requester identity",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	if err := ensureSpaceAuthor(data.SpaceUUID, requester.ID); err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Not authorized to create channels in this space",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	var channel DashDataChannel

	query := `INSERT INTO channels (uuid, name, space_uuid) VALUES (?, ?, ?) RETURNING *`
	err = db.ChatDB.QueryRow(query, channelUUID, data.Name, data.SpaceUUID).Scan(&channel.ID, &channel.UUID, &channel.Name, &channel.SpaceUUID, &channel.AllowVoice)

	if err != nil {
		// Check if the error message contains "UNIQUE constraint failed"
		if err.Error() == "UNIQUE constraint failed: channels.uuid" {
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{
					Content:    "UUID for new channel is already taken",
					ClientUUID: data.ClientUUID,
				},
			})
			return
		}

		// For other database errors
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error inserting new channel",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	sendToConn(conn, WSMessage{
		Type: "create_channel_response",
		Data: CreateChannelResponse{
			Channel:    channel,
			SpaceUUID:  data.SpaceUUID,
			ClientUUID: data.ClientUUID,
		},
	})
}

func handleDeleteChannel(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[DeleteChannelRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding delete_channel_request:", err)
		return
	}

	requester, err := resolveHostUserIdentityStrict(
		data.RequesterUserID,
		data.RequesterUserPublicKey,
		data.RequesterUserEncPublicKey,
	)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Failed to resolve requester identity",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	// Get channel info before deleting.
	var channelID int
	var spaceUUID string
	err = db.ChatDB.QueryRow(`SELECT id, space_uuid FROM channels WHERE uuid = ?`, data.UUID).Scan(&channelID, &spaceUUID)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Channel not found in database by UUID",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	if err := ensureSpaceAuthor(spaceUUID, requester.ID); err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Not authorized to delete this channel",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	// Delete the channel
	res, err := db.ChatDB.Exec(`DELETE FROM channels WHERE uuid = ?`, data.UUID)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error deleting channel",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Channel not found",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	sendToConn(conn, WSMessage{
		Type: "delete_channel_response",
		Data: DeleteChannelResponse{
			ID:         channelID,
			UUID:       data.UUID,
			SpaceUUID:  spaceUUID,
			ClientUUID: data.ClientUUID,
		},
	})
}

func handleChannelAllowVoice(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[ChannelAllowVoiceRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding channel_allow_voice_request:", err)
		return
	}

	query := `UPDATE channels SET allow_voice = ? WHERE uuid = ?`
	res, err := db.ChatDB.Exec(query, data.Allow, data.UUID)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error updating channel allow voice",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error updating channel allow voice",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
}
