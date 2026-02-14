package main

import (
	"fmt"
	"gochat/db"
	"log"
	"os"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
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
		delete(host.ClientsByConn, conn)
		close(oldClient.SendQueue)
		close(oldClient.Done)
	}

	clientUUID := uuid.New().String()
	client := &Client{
		Conn:            conn,
		HostUUID:        host.UUID,
		ClientUUID:      clientUUID,
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
	return client
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

	if err := SyncUserSpaceMemberships(client.HostUUID, data.User.ID, data.Spaces); err != nil {
		log.Printf("failed to sync user-space memberships host=%s user=%d: %v", client.HostUUID, data.User.ID, err)
	}

	if host, exists := GetHost(client.HostUUID); exists {
		host.mu.Lock()
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

	userIDs := make([]int, 0, len(data.Space.Users))
	for _, user := range data.Space.Users {
		userIDs = append(userIDs, user.ID)
	}
	if len(userIDs) == 0 && joinClient.UserID > 0 {
		userIDs = append(userIDs, joinClient.UserID)
	}
	if err := UpsertUsersInSpace(client.HostUUID, data.Space.UUID, userIDs); err != nil {
		log.Printf("failed to upsert space memberships host=%s space=%s: %v", client.HostUUID, data.Space.UUID, err)
	}

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

	go func() {
		err := sendSpaceInviteEmail(data.UserID, data.Email, client.HostUUID, data.Invite.Name, client.Username)
		if err != nil {
			log.Printf("failed to send invite email to user=%d email=%s: %v", data.UserID, data.Email, err)
		}
	}()

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

	userIDs := make([]int, 0, len(data.Space.Users)+1)
	for _, user := range data.Space.Users {
		userIDs = append(userIDs, user.ID)
	}
	if data.User.ID > 0 {
		userIDs = append(userIDs, data.User.ID)
	}
	if err := UpsertUsersInSpace(client.HostUUID, data.Space.UUID, userIDs); err != nil {
		log.Printf("failed to upsert accepted invite membership host=%s space=%s user=%d: %v", client.HostUUID, data.Space.UUID, data.User.ID, err)
	}

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
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	leaveSpace(host, client, data.SpaceUUID)

	if err := MarkUserLeftSpace(client.HostUUID, data.SpaceUUID, data.UserID); err != nil {
		log.Printf("failed to mark user left space host=%s space=%s user=%d: %v", client.HostUUID, data.SpaceUUID, data.UserID, err)
	}
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
		host.Channels[channelUUID] = &Channel{Users: make(map[*websocket.Conn]int), VoiceStreams: make(map[*websocket.Conn]string)}
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
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	removeClient, ok := host.ClientsByUserID[data.UserID]
	host.mu.Unlock()

	if ok {
		// tell connected user to leave
		SendToClient(client.HostUUID, removeClient.ClientUUID, WSMessage{
			Type: "leave_space_success",
			Data: "",
		})
		leaveSpace(host, removeClient, data.SpaceUUID)
	}

	if err := MarkUserLeftSpace(client.HostUUID, data.SpaceUUID, data.UserID); err != nil {
		log.Printf("failed to mark removed user left space host=%s space=%s user=%d: %v", client.HostUUID, data.SpaceUUID, data.UserID, err)
	}
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
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	channelUUID, ok := host.ChannelSubscriptions[conn]
	spaceUUID := ""
	if ok {
		spaceUUID = host.ChannelToSpace[channelUUID]
	}
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
				Username:  client.Username,
				Content:   data.Content,
				Timestamp: msgTimestamp,
			},
		})
	}

	connectedUserIDs := make(map[int]struct{})
	host.mu.Lock()
	for userID := range host.ClientsByUserID {
		connectedUserIDs[userID] = struct{}{}
	}
	host.mu.Unlock()
	go func() {
		if spaceUUID == "" {
			return
		}
		if err := TrackOfflineMessageActivity(client.HostUUID, spaceUUID, client.UserID, connectedUserIDs); err != nil {
			log.Printf("failed to track message activity host=%s space=%s sender=%d: %v", client.HostUUID, spaceUUID, client.UserID, err)
		}
	}()

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

// func handleGetTurnCredentials(client *Client) {
// 	secret := os.Getenv("TURN_SECRET")

// 	unixTime := time.Now().Add(1 * time.Hour).Unix()
// 	username := fmt.Sprintf("%d", unixTime)

// 	h := hmac.New(sha1.New, []byte(secret))
// 	h.Write([]byte(username))
// 	password := base64.StdEncoding.EncodeToString(h.Sum(nil))

// 	SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
// 		Type: "get_turn_credentials_response",
// 		Data: TurnCredentialsResponse{
// 			Username: username,
// 			Password: password,
// 		},
// 	})

// }

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

func handleChannelAllowVoice(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[ChannelAllowVoiceClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid chat message response data"}})
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "channel_allow_voice_request",
		Data: ChannelAllowVoiceRequest{
			UUID:       data.UUID,
			Allow:      data.Allow,
			ClientUUID: client.ClientUUID,
		},
	})

	// broadcast new state to space
	BroadcastToSpace(client.HostUUID, data.SpaceUUID, WSMessage{
		Type: "channel_allow_voice_update",
		Data: ChannelAllowVoiceUpdate{
			UUID:      data.UUID,
			SpaceUUID: data.SpaceUUID,
			Allow:     data.Allow,
		},
	})
}

func handleJoinVoiceChannel(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[JoinVoiceChannelClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid chat message response data"}})
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
	channelUUID, ok := host.ChannelSubscriptions[client.Conn]
	if !ok {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to channel"},
		})
		return
	}

	channel, exists := host.Channels[channelUUID]
	if !exists {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to channel"},
		})
		return
	}
	channel.mu.Lock()
	channel.VoiceStreams[client.Conn] = data.StreamID
	channel.mu.Unlock()

	voiceSubs := []VoiceSub{}

	for conn, streamID := range channel.VoiceStreams {
		if cl, ok := host.ClientsByConn[conn]; ok {
			voiceSubs = append(voiceSubs, VoiceSub{
				UserID:   cl.UserID,
				Username: cl.Username,
				StreamID: streamID,
			})
		}
	}

	host.mu.Unlock()

	// Send voice credentials to the joining client (for future client updates)
	sendVoiceCredentials(client, channelUUID)

	BroadcastToChannel(client.HostUUID, channelUUID, WSMessage{
		Type: "joined_voice_channel",
		Data: JoinedOrLeftVoiceChannel{
			ChannelUUID: channelUUID,
			VoiceSubs:   voiceSubs,
		},
	})
}

// sendVoiceCredentials sends TURN credentials and SFU token to a client joining voice
func sendVoiceCredentials(client *Client, channelUUID string) {
	turnURL := os.Getenv("TURN_URL")
	turnSecret := os.Getenv("TURN_SECRET")
	sfuSecret := os.Getenv("SFU_SECRET")

	// Skip if secrets not configured
	if turnURL == "" || turnSecret == "" {
		log.Println("TURN credentials not configured, skipping voice_credentials")
		return
	}

	// Generate TURN credentials
	ttl := int64(8 * 3600) // 8 hours
	turnUsername, turnCredential := generateTurnCredentials(turnSecret, ttl)

	// Generate SFU token (if SFU_SECRET is configured)
	var sfuToken string
	if sfuSecret != "" {
		var err error
		sfuToken, err = GenerateSFUToken(client.UserID, client.Username, channelUUID, sfuSecret, 8*time.Hour)
		if err != nil {
			log.Printf("Failed to generate SFU token: %v", err)
		}
	}

	SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
		Type: "voice_credentials",
		Data: VoiceCredentials{
			TurnURL:        turnURL,
			TurnUsername:   turnUsername,
			TurnCredential: turnCredential,
			SFUToken:       sfuToken,
			ChannelUUID:    channelUUID,
		},
	})
}

func handleLeaveVoiceChannel(client *Client) {
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
	if !ok {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to channel"},
		})
		return
	}

	channel, exists := host.Channels[channelUUID]
	if !exists {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to channel"},
		})
		return
	}
	channel.mu.Lock()
	delete(channel.VoiceStreams, client.Conn)
	channel.mu.Unlock()

	voiceSubs := []VoiceSub{}

	for conn, streamID := range channel.VoiceStreams {
		if cl, ok := host.ClientsByConn[conn]; ok {
			voiceSubs = append(voiceSubs, VoiceSub{
				UserID:   cl.UserID,
				Username: cl.Username,
				StreamID: streamID,
			})
		}
	}

	host.mu.Unlock()

	BroadcastToChannel(client.HostUUID, channelUUID, WSMessage{
		Type: "left_voice_channel",
		Data: JoinedOrLeftVoiceChannel{
			ChannelUUID: channelUUID,
			VoiceSubs:   voiceSubs,
		},
	})
}

// ============================================================================
// Call Room Handlers (Standalone Video Calls)
// ============================================================================

func lookupUserTier(tokenString string) (int, string, int) {
	jwtSecret := os.Getenv("JWT_SECRET")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return 0, "", 0
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", 0
	}
	userIDFloat, ok := claims["userID"].(float64)
	if !ok {
		return 0, "", 0
	}
	userID := int(userIDFloat)

	var subStatus string
	var credits int
	query := `SELECT COALESCE(subscription_status, 'none'), COALESCE(credit_minutes, 0) FROM users WHERE id = ?`
	err = db.HostDB.QueryRow(query, userID).Scan(&subStatus, &credits)
	if err != nil {
		return 0, "", 0
	}
	return userID, subStatus, credits
}

func dispatchCallRoomMessage(conn *websocket.Conn, wsMsg WSMessage) {
	switch wsMsg.Type {
	case "join_call_room":
		handleJoinCallRoom(conn, &wsMsg)
	case "leave_call_room":
		handleLeaveCallRoom(conn, &wsMsg)
	case "update_call_media":
		handleUpdateCallMedia(conn, &wsMsg)
	case "update_call_stream_id":
		handleUpdateCallStreamID(conn, &wsMsg)
	default:
		log.Println("Unknown call room message type:", wsMsg.Type)
	}
}

func handleJoinCallRoom(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[JoinCallRoomClient](wsMsg.Data)
	if err != nil {
		conn.WriteJSON(WSMessage{Type: "error", Data: ChatError{Content: "Invalid join call room data"}})
		return
	}

	callRoomsMu.Lock()
	room, exists := CallRooms[data.RoomID]
	if !exists {
		tier := "free"
		maxDuration := 40 * time.Minute
		creatorID := 0

		if data.Token != "" {
			userID, subStatus, credits := lookupUserTier(data.Token)
			log.Printf("Room creation: token present, userID=%d subStatus=%q credits=%d", userID, subStatus, credits)
			if userID > 0 {
				creatorID = userID
				if subStatus == "active" {
					tier = "premium"
					maxDuration = 6 * time.Hour
				} else if credits > 0 {
					tier = "premium"
					creditDuration := time.Duration(credits) * time.Minute
					if creditDuration > 6*time.Hour {
						creditDuration = 6 * time.Hour
					}
					maxDuration = creditDuration
				}
			}
		} else {
			log.Printf("Room creation: no token provided, creating free room")
		}

		log.Printf("Room created: id=%s tier=%s creatorID=%d maxDuration=%v", data.RoomID, tier, creatorID, maxDuration)
		room = &CallRoom{
			ID:           data.RoomID,
			CreatorID:    creatorID,
			CreatorToken: data.AnonToken,
			Tier:         tier,
			CreatedAt:    time.Now(),
			MaxDuration:  maxDuration,
			TimerDone:    make(chan struct{}),
			Participants: make(map[*websocket.Conn]*CallRoomParticipant),
		}
		CallRooms[data.RoomID] = room
		go startRoomTimer(room)
	}
	callRoomsMu.Unlock()

	participantID := uuid.New().String()
	participant := &CallRoomParticipant{
		ID:          participantID,
		DisplayName: data.DisplayName,
		StreamID:    data.StreamID,
		IsAudioOn:   true,
		IsVideoOn:   true,
		Conn:        conn,
		SendQueue:   make(chan WSMessage, 64),
		Done:        make(chan struct{}),
	}

	room.mu.Lock()

	// Build current participant list for the new joiner
	currentParticipants := []CallParticipant{}
	for _, p := range room.Participants {
		currentParticipants = append(currentParticipants, CallParticipant{
			ID:          p.ID,
			DisplayName: p.DisplayName,
			StreamID:    p.StreamID,
			IsAudioOn:   p.IsAudioOn,
			IsVideoOn:   p.IsVideoOn,
		})
	}

	// Add the new participant
	room.Participants[conn] = participant
	room.mu.Unlock()

	// Start the write pump
	go participant.WritePump()

	// Send room state to the new participant
	participant.SendQueue <- WSMessage{
		Type: "call_room_state",
		Data: CallRoomState{
			RoomID:       data.RoomID,
			Participants: currentParticipants,
		},
	}

	// Send join acknowledgment with their participant ID
	participant.SendQueue <- WSMessage{
		Type: "call_room_joined",
		Data: map[string]string{
			"room_id":        data.RoomID,
			"participant_id": participantID,
		},
	}

	// Send TURN credentials
	sendCallRoomVoiceCredentials(participant, data.RoomID)

	// Send time remaining to the new participant
	elapsed := time.Since(room.CreatedAt)
	remaining := room.MaxDuration - elapsed
	if remaining < 0 {
		remaining = 0
	}
	participant.SendQueue <- WSMessage{
		Type: "call_time_remaining",
		Data: CallTimeRemaining{
			RoomID:         data.RoomID,
			SecondsLeft:    int(remaining.Seconds()),
			Tier:           room.Tier,
			MaxDurationSec: int(room.MaxDuration.Seconds()),
		},
	}

	// Broadcast to other participants that someone joined
	newParticipant := CallParticipant{
		ID:          participantID,
		DisplayName: data.DisplayName,
		StreamID:    data.StreamID,
		IsAudioOn:   true,
		IsVideoOn:   true,
	}

	broadcastToCallRoom(room, conn, WSMessage{
		Type: "call_participant_joined",
		Data: CallParticipantJoined{
			RoomID:      data.RoomID,
			Participant: newParticipant,
		},
	})
}

func handleLeaveCallRoom(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[LeaveCallRoomClient](wsMsg.Data)
	if err != nil {
		conn.WriteJSON(WSMessage{Type: "error", Data: ChatError{Content: "Invalid leave call room data"}})
		return
	}

	callRoomsMu.Lock()
	room, exists := CallRooms[data.RoomID]
	callRoomsMu.Unlock()

	if !exists {
		return
	}

	room.mu.Lock()
	participant, ok := room.Participants[conn]
	if !ok {
		room.mu.Unlock()
		return
	}

	participantID := participant.ID
	delete(room.Participants, conn)

	// Clean up empty room
	if len(room.Participants) == 0 {
		close(room.TimerDone)
		callRoomsMu.Lock()
		delete(CallRooms, data.RoomID)
		callRoomsMu.Unlock()
	}
	room.mu.Unlock()

	// Close the participant's channels
	close(participant.SendQueue)
	close(participant.Done)

	// Broadcast to remaining participants
	broadcastToCallRoom(room, nil, WSMessage{
		Type: "call_participant_left",
		Data: CallParticipantLeft{
			RoomID:        data.RoomID,
			ParticipantID: participantID,
		},
	})
}

func handleUpdateCallMedia(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[UpdateCallMediaClient](wsMsg.Data)
	if err != nil {
		conn.WriteJSON(WSMessage{Type: "error", Data: ChatError{Content: "Invalid update call media data"}})
		return
	}

	callRoomsMu.Lock()
	room, exists := CallRooms[data.RoomID]
	callRoomsMu.Unlock()

	if !exists {
		return
	}

	room.mu.Lock()
	participant, ok := room.Participants[conn]
	if !ok {
		room.mu.Unlock()
		return
	}

	participant.IsAudioOn = data.IsAudioOn
	participant.IsVideoOn = data.IsVideoOn
	participantID := participant.ID
	room.mu.Unlock()

	// Broadcast media state update to all participants
	broadcastToCallRoom(room, nil, WSMessage{
		Type: "call_media_updated",
		Data: CallMediaUpdated{
			RoomID:        data.RoomID,
			ParticipantID: participantID,
			IsAudioOn:     data.IsAudioOn,
			IsVideoOn:     data.IsVideoOn,
		},
	})
}

func handleUpdateCallStreamID(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[UpdateCallStreamIDClient](wsMsg.Data)
	if err != nil {
		conn.WriteJSON(WSMessage{Type: "error", Data: ChatError{Content: "Invalid update call stream ID data"}})
		return
	}

	callRoomsMu.Lock()
	room, exists := CallRooms[data.RoomID]
	callRoomsMu.Unlock()

	if !exists {
		return
	}

	room.mu.Lock()
	participant, ok := room.Participants[conn]
	if !ok {
		room.mu.Unlock()
		return
	}

	participant.StreamID = data.StreamID
	participantID := participant.ID
	room.mu.Unlock()

	// Broadcast stream ID update to other participants (not the sender)
	broadcastToCallRoom(room, conn, WSMessage{
		Type: "call_stream_id_updated",
		Data: CallStreamIDUpdated{
			RoomID:        data.RoomID,
			ParticipantID: participantID,
			StreamID:      data.StreamID,
		},
	})
}

func broadcastToCallRoom(room *CallRoom, exclude *websocket.Conn, msg WSMessage) {
	room.mu.Lock()
	defer room.mu.Unlock()

	for conn, participant := range room.Participants {
		if conn == exclude {
			continue
		}
		select {
		case participant.SendQueue <- msg:
		default:
			log.Printf("broadcastToCallRoom: send queue full for participant %s", participant.ID)
		}
	}
}

func cleanupCallRoomParticipant(conn *websocket.Conn) {
	callRoomsMu.Lock()
	defer callRoomsMu.Unlock()

	for roomID, room := range CallRooms {
		room.mu.Lock()
		participant, ok := room.Participants[conn]
		if ok {
			participantID := participant.ID
			delete(room.Participants, conn)

			// Close channels
			close(participant.SendQueue)
			close(participant.Done)

			// Broadcast departure
			for _, p := range room.Participants {
				select {
				case p.SendQueue <- WSMessage{
					Type: "call_participant_left",
					Data: CallParticipantLeft{
						RoomID:        roomID,
						ParticipantID: participantID,
					},
				}:
				default:
				}
			}

			// Clean up empty room
			if len(room.Participants) == 0 {
				close(room.TimerDone)
				delete(CallRooms, roomID)
			}
		}
		room.mu.Unlock()
	}
}

// sendCallRoomVoiceCredentials sends TURN credentials to a call room participant
func sendCallRoomVoiceCredentials(participant *CallRoomParticipant, roomID string) {
	turnURL := os.Getenv("TURN_URL")
	turnSecret := os.Getenv("TURN_SECRET")
	sfuSecret := os.Getenv("SFU_SECRET")

	if turnURL == "" || turnSecret == "" {
		log.Println("TURN credentials not configured, skipping voice_credentials for call room")
		return
	}

	ttl := int64(8 * 3600)
	turnUsername, turnCredential := generateTurnCredentials(turnSecret, ttl)

	var sfuToken string
	if sfuSecret != "" {
		var err error
		sfuToken, err = GenerateSFUToken(0, participant.DisplayName, roomID, sfuSecret, 8*time.Hour)
		if err != nil {
			log.Printf("Failed to generate SFU token for call room: %v", err)
		}
	}

	participant.SendQueue <- WSMessage{
		Type: "voice_credentials",
		Data: VoiceCredentials{
			TurnURL:        turnURL,
			TurnUsername:   turnUsername,
			TurnCredential: turnCredential,
			SFUToken:       sfuToken,
			ChannelUUID:    roomID,
		},
	}
}
