package main

import (
	"gochat/db"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/gorilla/websocket"
)

func registerOrCreateHost(hostUUID, potentialAuthorID string, conn *websocket.Conn) (*Host, error) {
	hostsMu.Lock()
	defer hostsMu.Unlock()

	host, exists := Hosts[hostUUID]
	if !exists {
		var authorID string
		err := db.HostDB.QueryRow("SELECT author_id FROM hosts WHERE uuid = ?", hostUUID).Scan(&authorID)
		if err != nil {
			return nil, err
		}
		host = &Host{
			UUID:                 hostUUID,
			AuthorID:             authorID,
			ClientsByConn:        make(map[*websocket.Conn]*Client),
			ClientConnsByUUID:    make(map[string]*websocket.Conn),
			ClientsByUserID:      make(map[int]*Client),
			ClientsByPublicKey:   make(map[string]*Client),
			ConnByAuthorID:       make(map[string]*websocket.Conn),
			Channels:             make(map[string]*Channel),
			ChannelToSpace:       make(map[string]string),
			Spaces:               make(map[string]*Space),
			ChannelSubscriptions: make(map[*websocket.Conn]string),
			SpaceSubscriptions:   make(map[*websocket.Conn][]string),
		}
		Hosts[hostUUID] = host
	}

	if potentialAuthorID != "" {
		host.ConnByAuthorID[potentialAuthorID] = conn

		// Set host to online
		HandleUpdateHostOnline(hostUUID)
	}

	return host, nil
}

func registerClient(host *Host, conn *websocket.Conn, clientIP string) *Client {
	host.mu.Lock()

	if oldClient, ok := host.ClientsByConn[conn]; ok {
		// If old client was authenticated, unregister their IP
		if oldClient.IsAuthenticated {
			UnregisterAuthenticatedIP(oldClient.IP)
		}
		delete(host.ClientConnsByUUID, oldClient.ClientUUID)
		delete(host.ClientsByUserID, oldClient.UserID)
		if oldClient.PublicKey != "" {
			delete(host.ClientsByPublicKey, oldClient.PublicKey)
		}
		delete(host.ClientsByConn, conn)
		close(oldClient.SendQueue)
		close(oldClient.Done)
	}

	clientUUID := uuid.New().String()
	challenge := newAuthChallenge()
	client := &Client{
		Conn:            conn,
		HostUUID:        host.UUID,
		ClientUUID:      clientUUID,
		AuthChallenge:   challenge,
		IP:              clientIP,
		IsAuthenticated: false,
		SendQueue:       make(chan WSMessage, 64),
		Done:            make(chan struct{}),
	}

	host.ClientsByConn[conn] = client
	host.ClientConnsByUUID[clientUUID] = conn
	host.mu.Unlock()

	go client.WritePump()

	client.SendQueue <- WSMessage{
		Type: "join_ack",
		Data: "Joined host successfully",
	}
	client.SendQueue <- WSMessage{
		Type: "auth_challenge",
		Data: AuthChallenge{Challenge: challenge},
	}
	return client
}

func findHostClientByIdentity(host *Host, userID int, userPublicKey string) (*Client, bool) {
	if userPublicKey != "" {
		if c, ok := host.ClientsByPublicKey[userPublicKey]; ok {
			return c, true
		}
	}
	if userID > 0 {
		if c, ok := host.ClientsByUserID[userID]; ok {
			return c, true
		}
	}
	return nil, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func handleGetDashData(client *Client, conn *websocket.Conn) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	userID := host.ClientsByConn[conn].UserID
	username := host.ClientsByConn[conn].Username
	host.mu.Unlock()

	SendToAuthor(client, WSMessage{ // Regular client assuming host is already logged in
		Type: "get_dash_data_request",
		Data: GetDashDataRequest{
			UserID:           userID,
			UserPublicKey:    client.PublicKey,
			UserEncPublicKey: client.EncPublicKey,
			Username:         username,
			ClientUUID:       client.ClientUUID,
		},
	})
}

func handleGetDashDatRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[GetDashDataResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid dashboard data"}})
		return
	}

	if host, exists := GetHost(client.HostUUID); exists {
		host.mu.Lock()
		joinConn, ok := host.ClientConnsByUUID[data.ClientUUID]
		if ok {
			joinClient := host.ClientsByConn[joinConn]
			if joinClient != nil {
				if joinClient.UserID > 0 {
					delete(host.ClientsByUserID, joinClient.UserID)
				}
				if joinClient.PublicKey != "" {
					delete(host.ClientsByPublicKey, joinClient.PublicKey)
				}
				joinClient.UserID = data.User.ID
				joinClient.PublicKey = firstNonEmpty(data.User.PublicKey, joinClient.PublicKey)
				joinClient.EncPublicKey = firstNonEmpty(data.User.EncPublicKey, joinClient.EncPublicKey)
				joinClient.Username = data.User.Username
				if joinClient.UserID > 0 {
					host.ClientsByUserID[joinClient.UserID] = joinClient
				}
				if joinClient.PublicKey != "" {
					host.ClientsByPublicKey[joinClient.PublicKey] = joinClient
				}
			}
		}
		for _, space := range data.Spaces {
			for _, channel := range space.Channels {
				host.ChannelToSpace[channel.UUID] = space.UUID
			}
		}
		host.mu.Unlock()
	}

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "dash_data_payload",
		Data: GetDashDataSuccess{
			User:    data.User,
			Spaces:  data.Spaces,
			Invites: data.Invites,
		},
	})
}

func handleCreateSpace(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[CreateSpaceClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid create space request data"}})
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "create_space_request",
		Data: CreateSpaceRequest{
			Name:             data.Name,
			UserID:           client.UserID,
			UserPublicKey:    client.PublicKey,
			UserEncPublicKey: client.EncPublicKey,
			Username:         client.Username,
			ClientUUID:       client.ClientUUID,
		},
	})
}

func handleCreateSpaceRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[CreateSpaceResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid create space response data"}})
		return
	}

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	joinConn, ok := host.ClientConnsByUUID[data.ClientUUID]
	if !ok {
		host.mu.Unlock()
		log.Printf("SendToClient: client not connected to host\n")
		return
	}
	joinClient := host.ClientsByConn[joinConn]
	host.mu.Unlock()

	joinSpace(joinClient, data.Space.UUID)

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "create_space_success",
		Data: CreateSpaceSuccess{
			Space: data.Space,
		},
	})

}

func handleDeleteSpace(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[DeleteSpaceClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid delete space response data"}})
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "delete_space_request",
		Data: DeleteSpaceRequest{
			UUID:       data.UUID,
			ClientUUID: client.ClientUUID,
		},
	})
}

func handleDeleteSpaceRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[map[string]string](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid delete space success data"}})
		return
	}

	clientUUID := data["ClientUUID"]

	SendToClient(client.HostUUID, clientUUID, WSMessage{
		Type: "delete_space_success",
		Data: "",
	})
}

func handleCreateChannel(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[CreateChannelClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid create channel request data"}})
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "create_channel_request",
		Data: CreateChannelRequest{
			Name:       data.Name,
			SpaceUUID:  data.SpaceUUID,
			ClientUUID: client.ClientUUID,
		},
	})
}

func handleCreateChannelRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[CreateChannelResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid create channel response data"}})
		return
	}

	if host, exists := GetHost(client.HostUUID); exists {
		host.mu.Lock()
		host.ChannelToSpace[data.Channel.UUID] = data.SpaceUUID
		host.mu.Unlock()
	}

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "create_channel_success",
		Data: CreateChannelSuccess{
			SpaceUUID: data.SpaceUUID,
			Channel:   data.Channel,
		},
	})

	BroadcastToSpace(client.HostUUID, data.SpaceUUID, WSMessage{
		Type: "create_channel_update",
		Data: CreateChannelUpdate{
			SpaceUUID: data.SpaceUUID,
			Channel:   data.Channel,
		},
	})
}

func handleDeleteChannel(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[DeleteChannelClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid delete channel response data"}})
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "delete_channel_request",
		Data: DeleteChannelRequest{
			UUID:       data.UUID,
			ClientUUID: client.ClientUUID,
		},
	})
}

func handleDeleteChannelRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[DeleteChannelResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid delete channel success data"}})
		return
	}

	if host, exists := GetHost(client.HostUUID); exists {
		host.mu.Lock()
		delete(host.ChannelToSpace, data.UUID)
		host.mu.Unlock()
	}

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "delete_channel_success",
		Data: data,
	})

	BroadcastToSpace(client.HostUUID, data.SpaceUUID, WSMessage{
		Type: "delete_channel_update",
		Data: DeleteChannelUpdate{
			UUID:      data.UUID,
			SpaceUUID: data.SpaceUUID,
		},
	})
}

func handleInviteUser(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[InviteUserClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid invite user data"}})
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "invite_user_request",
		Data: InviteUserRequest{
			PublicKey:  data.PublicKey,
			SpaceUUID:  data.SpaceUUID,
			ClientUUID: client.ClientUUID,
		},
	})
}

func handleInviteUserRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[InviteUserResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid invite user response data"}})
		return
	}
	// send back to inviter
	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "invite_user_success",
		Data: InviteUserSuccess{
			PublicKey: data.PublicKey,
		},
	})

	// send to invitee
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	inviteeClient, ok := findHostClientByIdentity(host, data.UserID, data.UserPublicKey)
	if !ok {
		host.mu.Unlock()
		log.Println("SendToClient: invitee not connected to host")
		return
	}
	host.mu.Unlock()

	SendToClient(client.HostUUID, inviteeClient.ClientUUID, WSMessage{
		Type: "invite_user_update",
		Data: InviteUserUpdate{
			Invite: data.Invite,
		},
	})
}

func handleAcceptInvite(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[AcceptInviteClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid accept invite data"}})
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "accept_invite_request",
		Data: AcceptInviteRequest{
			SpaceUserID:      data.SpaceUserID,
			UserID:           data.UserID,
			UserPublicKey:    firstNonEmpty(data.UserPublicKey, client.PublicKey),
			UserEncPublicKey: firstNonEmpty(data.UserEncPublicKey, client.EncPublicKey),
			ClientUUID:       client.ClientUUID,
		},
	})
}

func handleAcceptInviteRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[AcceptInviteResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid accept invite response data"}})
		return
	}

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToClient: host %s not found\n", client.HostUUID)
		return
	}

	host.mu.Lock()
	joinConn, ok := host.ClientConnsByUUID[data.ClientUUID]
	if !ok {
		host.mu.Unlock()
		log.Printf("SendToClient: client not connected to host\n")
		return
	}
	joinClient := host.ClientsByConn[joinConn]
	host.mu.Unlock()

	joinSpace(joinClient, data.Space.UUID)

	// send to invited client
	SendToClient(client.HostUUID, joinClient.ClientUUID, WSMessage{
		Type: "accept_invite_success",
		Data: AcceptInviteSuccess{
			SpaceUserID: data.SpaceUserID,
			User:        data.User,
			Space:       data.Space,
		},
	})
	// broadcast to space
	BroadcastToSpace(client.HostUUID, data.Space.UUID, WSMessage{
		Type: "accept_invite_update",
		Data: AcceptInviteUpdate{
			SpaceUUID: data.Space.UUID,
			User:      data.User,
		},
	})
}

func handleDeclineInvite(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[DeclineInviteClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid decline invite data"}})
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "decline_invite_request",
		Data: DeclineInviteRequest{
			SpaceUserID:      data.SpaceUserID,
			UserID:           data.UserID,
			UserPublicKey:    firstNonEmpty(data.UserPublicKey, client.PublicKey),
			UserEncPublicKey: firstNonEmpty(data.UserEncPublicKey, client.EncPublicKey),
			ClientUUID:       client.ClientUUID,
		},
	})
}

func handleDeclineInviteRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[DeclineInviteResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid decline invite response data"}})
		return
	}
	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "decline_invite_success",
		Data: DeclineInviteSuccess{
			SpaceUserID:   data.SpaceUserID,
			UserID:        data.UserID,
			UserPublicKey: data.UserPublicKey,
		},
	})
}

func handleLeaveSpace(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[LeaveSpaceClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid leave space data"}})
		return
	}
	SendToAuthor(client, WSMessage{
		Type: "leave_space_request",
		Data: LeaveSpaceRequest{
			SpaceUUID:        data.SpaceUUID,
			UserID:           data.UserID,
			UserPublicKey:    firstNonEmpty(data.UserPublicKey, client.PublicKey),
			UserEncPublicKey: firstNonEmpty(data.UserEncPublicKey, client.EncPublicKey),
			ClientUUID:       client.ClientUUID,
		},
	})
}

func handleLeaveSpaceRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[LeaveSpaceResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid leave space response data"}})
		return
	}

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "leave_space_success",
		Data: "",
	})

	BroadcastToSpace(client.HostUUID, data.SpaceUUID, WSMessage{
		Type: "leave_space_update",
		Data: LeaveSpaceUpdate{
			SpaceUUID:     data.SpaceUUID,
			UserID:        data.UserID,
			UserPublicKey: data.UserPublicKey,
		},
	})

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	leaveSpace(host, client, data.SpaceUUID)

}

func joinChannel(client *Client, channelUUID string) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()

	if _, ok := host.Channels[channelUUID]; !ok {
		host.Channels[channelUUID] = &Channel{Users: make(map[*websocket.Conn]int)}
	}
	channel := host.Channels[channelUUID]
	channel.mu.Lock()
	channel.Users[client.Conn] = client.UserID
	channel.mu.Unlock()
	host.ChannelSubscriptions[client.Conn] = channelUUID

	host.mu.Unlock()
	SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
		Type: "joined_channel",
		Data: "",
	})
}

func leaveChannel(client *Client) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	channelUUID, ok := host.ChannelSubscriptions[client.Conn]
	if ok {
		if channel, exists := host.Channels[channelUUID]; exists {
			channel.mu.Lock()
			delete(channel.Users, client.Conn)
			channel.mu.Unlock()
		}
		delete(host.ChannelSubscriptions, client.Conn)
	}

	host.mu.Unlock()

	SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
		Type: "left_channel",
		Data: "",
	})
}

func handleJoinAllSpaces(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[JoinAllSpacesClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid join_all_spaces data"}})
		return
	}

	for _, uuid := range data.SpaceUUIDs {
		joinSpace(client, uuid)
	}

	SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
		Type: "join_all_spaces_success",
		Data: "",
	})
}

// func handleJoinSpace(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
// 	data, err := decodeData[JoinSpaceClient](wsMsg.Data)
// 	if err != nil {
// 		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid join_space data"}})
// 		return
// 	}

// 	joinSpace(client, data.SpaceUUID)

// SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
// 	Type: "join_space_success",
// 	Data: "",
// })
// }

func handleRemoveSpaceUser(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[RemoveSpaceUserClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid remove space user data"}})
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "remove_space_user_request",
		Data: RemoveSpaceUserRequest{
			SpaceUUID:        data.SpaceUUID,
			UserID:           data.UserID,
			UserPublicKey:    data.UserPublicKey,
			UserEncPublicKey: data.UserEncPublicKey,
			ClientUUID:       client.ClientUUID,
		},
	})
}

func handleRemoveSpaceUserRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[RemoveSpaceUserResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid remove space user response data"}})
		return
	}

	// same as leave space
	BroadcastToSpace(client.HostUUID, data.SpaceUUID, WSMessage{
		Type: "leave_space_update",
		Data: RemoveSpaceUserUpdate{
			SpaceUUID:     data.SpaceUUID,
			UserID:        data.UserID,
			UserPublicKey: data.UserPublicKey,
		},
	})

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	removeClient, ok := findHostClientByIdentity(host, data.UserID, data.UserPublicKey)
	host.mu.Unlock()

	if ok {
		// tell connected user to leave
		SendToClient(client.HostUUID, removeClient.ClientUUID, WSMessage{
			Type: "leave_space_success",
			Data: "",
		})
		leaveSpace(host, removeClient, data.SpaceUUID)
	}

}

func handleChatMessage(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[ChatData](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid chat message data"}})
		return
	}
	if len(data.Envelope) == 0 {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Missing encrypted envelope"}})
		return
	}

	msgTimestamp := time.Now().UTC()

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	channelUUID, ok := host.ChannelSubscriptions[conn]
	host.mu.Unlock()
	if !ok {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the channel"},
		})
		return
	}

	if channelUUID != "" {
		BroadcastToChannel(client.HostUUID, channelUUID, WSMessage{
			Type: "chat",
			Data: ChatPayload{
				Envelope:  data.Envelope,
				Timestamp: msgTimestamp,
			},
		})
	}

	// save message to db
	SendToAuthor(client, WSMessage{
		Type: "save_chat_message_request",
		Data: SaveChatMessageRequest{
			UserID:           client.UserID,
			UserPublicKey:    client.PublicKey,
			UserEncPublicKey: client.EncPublicKey,
			Username:         client.Username,
			ChannelUUID:      channelUUID,
			Envelope:         data.Envelope,
		},
	})
}

func handleGetMessages(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[GetMessagesClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid chat message data"}})
		return
	}

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	channelUUID, ok := host.ChannelSubscriptions[conn]
	if !ok {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the channel"},
		})
		return
	}
	host.mu.Unlock()

	SendToAuthor(client, WSMessage{
		Type: "get_messages_request",
		Data: GetMessagesRequest{
			ChannelUUID:    channelUUID,
			ClientUUID:     client.ClientUUID,
			BeforeUnixTime: data.BeforeUnixTime,
		},
	})
}

func handleGetMessagesRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[GetMessagesResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid chat message response data"}})
		return
	}

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "get_messages_success",
		Data: GetMessagesSuccess{
			Messages:        data.Messages,
			ChannelUUID:     data.ChannelUUID,
			HasMoreMessages: data.HasMoreMessages,
		},
	})
}
