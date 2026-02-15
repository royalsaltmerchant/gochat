package main

import (
	"gochat/db"
	"log"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func handleCreateSpace(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[CreateSpaceRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding create_space_request:", err)
		return
	}

	// Get UUID and Author ID
	spaceUUID := uuid.New()

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

	var space DashDataSpace

	query := `INSERT INTO spaces (uuid, name, author_id) VALUES (?, ?, ?) RETURNING *`
	err = db.ChatDB.QueryRow(query, spaceUUID, data.Name, user.ID).Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID)

	if err != nil {
		// Check if the error message contains "UNIQUE constraint failed"
		if err.Error() == "UNIQUE constraint failed: spaces.uuid" {
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{
					Content:    "UUID for new space is already taken",
					ClientUUID: data.ClientUUID,
				},
			})
			return
		}

		// For other database errors
		log.Println(err)
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error inserting new space",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	channelUUID := uuid.New()
	initalChannelName := "Initial Channel"

	var channel DashDataChannel

	query = `INSERT INTO channels (uuid, name, space_uuid) VALUES (?, ?, ?) RETURNING *`
	err = db.ChatDB.QueryRow(query, channelUUID, initalChannelName, space.UUID).Scan(&channel.ID, &channel.UUID, &channel.Name, &channel.SpaceUUID, &channel.AllowVoice)

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

	AppendspaceChannelsAndUsers(&space)

	sendToConn(conn, WSMessage{
		Type: "create_space_response",
		Data: CreateSpaceResponse{
			Space:      space,
			ClientUUID: data.ClientUUID,
		},
	})

}

func handleDeleteSpace(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[DeleteSpaceRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding delete_space_request:", err)
		return
	}

	// Delete the space (cascades to channels, messages, space_users)
	res, err := db.ChatDB.Exec("DELETE FROM spaces WHERE uuid = ?", data.UUID)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error deleting space",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Space not found",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	sendToConn(conn, WSMessage{
		Type: "delete_space_response",
		Data: map[string]interface{}{
			"ClientUUID": data.ClientUUID,
		},
	})
}

func handleGetDashData(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[GetDashDataRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding get_dash_data_request:", err)
		return
	}

	// 1. Use helper
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
	if user.Username == "" {
		user.Username = data.Username
	}

	userSpaces, err := GetUserSpaces(user.ID)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database failed to get user spaces",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	// 2. Enrich with channels/users
	for i := range userSpaces {
		AppendspaceChannelsAndUsers(&userSpaces[i])
	}

	// Collect invites (space_users.joined = 0) + space.name
	query := `
				SELECT su.id, su.space_uuid, su.user_id, su.joined, s.name
				FROM space_users su
				JOIN spaces s ON su.space_uuid = s.uuid
				WHERE su.user_id = ? AND su.joined = 0
			`

	rows, err := db.ChatDB.Query(query, user.ID)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error fetching invites",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}
	defer rows.Close()

	var spaceInvites []DashDataInvite
	for rows.Next() {
		var spaceInvite DashDataInvite
		err := rows.Scan(&spaceInvite.ID, &spaceInvite.SpaceUUID, &spaceInvite.UserID, &spaceInvite.Joined, &spaceInvite.Name)
		if err != nil {
			continue
		}
		spaceInvites = append(spaceInvites, spaceInvite)
	}
	spaceInvites = hydrateInvitePublicKeys(spaceInvites)

	// Official host auto-invite: every authenticated user gets a pending invite
	// to the official community space (if they are not already a member/invited).
	if isOfficialHostInstance() {
		alreadyInOfficialSpace := false
		for _, s := range userSpaces {
			if s.UUID == officialSpaceUUID {
				alreadyInOfficialSpace = true
				break
			}
		}
		if !alreadyInOfficialSpace {
			for _, inv := range spaceInvites {
				if inv.SpaceUUID == officialSpaceUUID {
					alreadyInOfficialSpace = true
					break
				}
			}
		}
		if !alreadyInOfficialSpace {
			var autoInvite DashDataInvite
			err := db.ChatDB.QueryRow(
				`INSERT INTO space_users (space_uuid, user_id) VALUES (?, ?) RETURNING id, space_uuid, user_id, joined`,
				officialSpaceUUID, user.ID,
			).Scan(&autoInvite.ID, &autoInvite.SpaceUUID, &autoInvite.UserID, &autoInvite.Joined)
			if err != nil {
				log.Println("Auto-invite insert error:", err)
			} else {
				autoInvite.Name = officialSpaceName
				autoInvite.UserPublicKey = user.PublicKey
				spaceInvites = append(spaceInvites, autoInvite)

				// Emit invite update back through relay so the client sees the new invite
				sendToConn(conn, WSMessage{
					Type: "invite_user_success",
					Data: InviteUserResponse{
						PublicKey:     "",
						UserID:        user.ID,
						UserPublicKey: user.PublicKey,
						SpaceUUID:     officialSpaceUUID,
						Invite:        autoInvite,
						ClientUUID:    data.ClientUUID,
					},
				})
			}
		}
	}

	sendToConn(conn, WSMessage{
		Type: "get_dash_data_response",
		Data: GetDashDataResponse{
			User: DashDataUser{
				ID:           user.ID,
				Username:     user.Username,
				PublicKey:    user.PublicKey,
				EncPublicKey: user.EncPublicKey,
			},
			Spaces:     userSpaces,
			Invites:    spaceInvites,
			ClientUUID: data.ClientUUID,
		},
	})
}
