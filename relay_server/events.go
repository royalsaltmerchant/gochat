package main

import (
	"gochat/db"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
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
			ConnByAuthorID:       make(map[string]*websocket.Conn),
			Channels:             make(map[string]*Channel),
			Spaces:               make(map[string]*Space),
			ChannelSubscriptions: make(map[*websocket.Conn]string),
			SpaceSubscriptions:   make(map[*websocket.Conn][]string),
		}
		Hosts[hostUUID] = host
	}

	if potentialAuthorID != "" {
		host.mu.Lock()
		host.ConnByAuthorID[potentialAuthorID] = conn
		host.mu.Unlock()
	}

	return host, nil
}

func registerClient(host *Host, conn *websocket.Conn) *Client {
	clientUUID := uuid.New().String()
	client := &Client{
		Conn:       conn,
		HostUUID:   host.UUID,
		ClientUUID: clientUUID,
		SendQueue:  make(chan WSMessage, 64),
		Done:       make(chan struct{}),
	}

	host.mu.Lock()
	host.ClientsByConn[conn] = client
	host.ClientConnsByUUID[clientUUID] = conn
	host.mu.Unlock()

	go client.WritePump()

	client.SendQueue <- WSMessage{
		Type: "join_ack",
		Data: "Joined host successfully",
	}

	return client
}

func handleRegisterUser(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[RegisterUser](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid registration data"}})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password), bcrypt.DefaultCost)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Password hashing failed"}})
		return
	}

	// Notify the host author
	SendToAuthor(client, WSMessage{
		Type: "register_user_request",
		Data: ApproveRegisterUser{
			Username:   data.Username,
			Email:      data.Email,
			Password:   string(hashedPassword),
			ClientUUID: client.ClientUUID,
		},
	})
}

func handleLogin(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[LoginUser](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid login data"}})
		return
	}

	SendToAuthor(client, WSMessage{ // Regular client assuming host is already logged in
		Type: "login_user_request",
		Data: ApproveLoginUser{
			Email:      data.Email,
			Password:   data.Password,
			ClientUUID: client.ClientUUID,
		},
	})

	safeSend(client, conn, WSMessage{Type: "login_user_pending"})
}

func handleLoginByToken(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[LoginUserByToken](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid login data"}})
		return
	}

	SendToAuthor(client, WSMessage{ // Regular client assuming host is already logged in
		Type: "login_user_by_token_request",
		Data: ApproveLoginUserByToken{
			Token:      data.Token,
			ClientUUID: client.ClientUUID,
		},
	})

	safeSend(client, conn, WSMessage{Type: "login_user_pending"})
}

func handleLoginApproved(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[ApprovedLoginUser](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid login approved data"}})
		return
	}

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	clientConn, ok := host.ClientConnsByUUID[data.ClientUUID]
	if !ok {
		log.Printf("SendToClient: client not connected to host %s\n", client.HostUUID)
		host.mu.Unlock()
		return
	}
	host.mu.Unlock()

	// login user
	host.ClientsByConn[clientConn].UserID = data.UserID
	host.ClientsByConn[clientConn].Username = data.Username
	host.ClientsByUserID[data.UserID] = host.ClientsByConn[clientConn]

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "login_user_success",
		Data: LoginUserToken{
			Token: data.Token,
		},
	})
}

func handleGetDashData(client *Client, conn *websocket.Conn) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
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
			UserID:     userID,
			Username:   username,
			ClientUUID: client.ClientUUID,
		},
	})
}

func handleGetDashDatRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[GetDashDataResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid dashboard data"}})
		return
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

func handleUpdateUsername(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[UpdateUsernameRequest](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid update username data"}})
		return
	}

	SendToAuthor(client, WSMessage{ // Regular client assuming host is already logged in
		Type: "update_username_request",
		Data: UpdateUsernameClient{
			UserID:     data.UserID,
			Username:   data.Username,
			ClientUUID: client.ClientUUID,
		},
	})
}

func handleUpdateUsernameApproved(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[UpdateUsernameResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid update username approved data"}})
		return
	}

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	clientConn, ok := host.ClientConnsByUUID[data.ClientUUID]
	if !ok {
		host.mu.Unlock()
		log.Printf("SendToClient: client not connected to host %s\n", client.HostUUID)
		return
	}
	host.mu.Unlock()

	// update user
	host.ClientsByConn[clientConn].Username = data.Username

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "update_username_success",
		Data: UpdateUsername{
			UserID:   data.UserID,
			Username: data.Username,
			Token:    data.Token,
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
			Name:       data.Name,
			UserID:     client.UserID,
			ClientUUID: client.ClientUUID,
		},
	})
}

func handleCreateSpaceRes(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[CreateSpaceResponse](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid create space response data"}})
		return
	}

	joinSpace(client, data.Space.UUID)

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

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "create_channel_success",
		Data: CreateChannelSuccess{
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

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "delete_channel_success",
		Data: data,
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
			Email:      data.Email,
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
			Email: data.Email,
		},
	})

	// send to invitee
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	inviteeClient, ok := host.ClientsByUserID[data.UserID]
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
			SpaceUserID: data.SpaceUserID,
			UserID:      data.UserID,
			ClientUUID:  client.ClientUUID,
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
			SpaceUserID: data.SpaceUserID,
			UserID:      data.UserID,
			ClientUUID:  client.ClientUUID,
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
			SpaceUserID: data.SpaceUserID,
			UserID:      data.UserID,
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
			SpaceUUID:  data.SpaceUUID,
			UserID:     data.UserID,
			ClientUUID: client.ClientUUID,
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
			SpaceUUID: data.SpaceUUID,
			UserID:    data.UserID,
		},
	})

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	leaveSpace(host, client, data.SpaceUUID)
}

func joinChannel(client *Client, channelUUID string) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
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
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
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
			SpaceUUID:  data.SpaceUUID,
			UserID:     data.UserID,
			ClientUUID: client.ClientUUID,
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
			SpaceUUID: data.SpaceUUID,
			UserID:    data.UserID,
		},
	})

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	removeClient, ok := host.ClientsByUserID[data.UserID]
	if !ok {
		host.mu.Unlock()
		log.Printf("SendToClient: client not connected to host\n")
		return
	}
	host.mu.Unlock()

	// tell user to leave
	SendToClient(client.HostUUID, removeClient.ClientUUID, WSMessage{
		Type: "leave_space_success",
		Data: "",
	})

	leaveSpace(host, removeClient, data.SpaceUUID)
}

func handleChatMessage(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[ChatData](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid chat message data"}})
		return
	}

	msgTimestamp := time.Now().UTC()

	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	channelUUID, ok := host.ChannelSubscriptions[conn]
	if !ok {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the channel"},
		})
		return
	}
	host.mu.Unlock()

	if channelUUID != "" {
		BroadcastToChannel(client.HostUUID, channelUUID, WSMessage{
			Type: "chat",
			Data: ChatPayload{
				Username:  client.Username,
				Content:   data.Content,
				Timestamp: msgTimestamp,
			},
		})
	}
	// save message to db
	SendToAuthor(client, WSMessage{
		Type: "save_chat_message_request",
		Data: SaveChatMessageRequest{
			UserID:      client.UserID,
			ChannelUUID: channelUUID,
			Content:     data.Content,
		},
	})
}

func handleGetMessages(client *Client, conn *websocket.Conn) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	channelUUID, ok := host.ChannelSubscriptions[conn]
	if !ok {
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
			ChannelUUID: channelUUID,
			ClientUUID:  client.ClientUUID,
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
			Messages:    data.Messages,
			ChannelUUID: data.ChannelUUID,
		},
	})
}
