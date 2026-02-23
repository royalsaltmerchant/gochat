package main

import (
	"fmt"
	"gochat/db"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gorilla/websocket"
)

func registerOrCreateHost(hostUUID string) (*Host, error) {
	hostsMu.Lock()
	defer hostsMu.Unlock()

	host, exists := Hosts[hostUUID]
	if !exists {
		var signingPublicKey string
		err := db.HostDB.QueryRow("SELECT signing_public_key FROM hosts WHERE uuid = ?", hostUUID).Scan(&signingPublicKey)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(signingPublicKey) == "" {
			return nil, ErrHostSigningKeyMissing
		}
		host = &Host{
			UUID:                 hostUUID,
			SigningPublicKey:     strings.TrimSpace(signingPublicKey),
			ClientsByConn:        make(map[*websocket.Conn]*Client),
			ClientConnsByUUID:    make(map[string]*websocket.Conn),
			ClientsByUserID:      make(map[int]*Client),
			ClientsByPublicKey:   make(map[string]*Client),
			Channels:             make(map[string]*Channel),
			ChannelToSpace:       make(map[string]string),
			Spaces:               make(map[string]*Space),
			ChannelSubscriptions: make(map[*websocket.Conn]string),
			SpaceSubscriptions:   make(map[*websocket.Conn][]string),
		}
		Hosts[hostUUID] = host
	}

	return host, nil
}

func registerClient(host *Host, conn *websocket.Conn, clientIP string, isHostCandidate bool) *Client {
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
	userChallenge := newAuthChallenge()
	hostChallenge := ""
	if isHostCandidate {
		hostChallenge = newAuthChallenge()
	}
	client := &Client{
		Conn:              conn,
		HostUUID:          host.UUID,
		ClientUUID:        clientUUID,
		AuthChallenge:     userChallenge,
		HostAuthChallenge: hostChallenge,
		IP:                clientIP,
		IsHostCandidate:   isHostCandidate,
		IsHostAuthor:      false,
		IsAuthenticated:   false,
		SendQueue:         make(chan WSMessage, 64),
		Done:              make(chan struct{}),
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
		Data: AuthChallenge{Challenge: userChallenge},
	}
	if isHostCandidate && hostChallenge != "" {
		client.SendQueue <- WSMessage{
			Type: "host_auth_challenge",
			Data: HostAuthChallenge{Challenge: hostChallenge},
		}
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

func activeDevicesForPublicKeyLocked(host *Host, publicKey string, currentClientUUID string) []ActiveDevice {
	if publicKey == "" {
		return nil
	}
	type deviceState struct {
		device   ActiveDevice
		lastSeen time.Time
	}
	byDeviceID := make(map[string]deviceState)
	for _, candidate := range host.ClientsByConn {
		if candidate == nil || !candidate.IsAuthenticated || candidate.PublicKey != publicKey {
			continue
		}
		deviceID := strings.TrimSpace(candidate.DeviceID)
		if deviceID == "" {
			deviceID = "unknown-" + candidate.ClientUUID
		}
		deviceName := strings.TrimSpace(candidate.DeviceName)
		if deviceName == "" {
			deviceName = "Unknown Device"
		}
		lastSeen := candidate.LastSeen
		if lastSeen.IsZero() {
			lastSeen = time.Now().UTC()
		}
		next := ActiveDevice{
			DeviceID:   deviceID,
			DeviceName: deviceName,
			LastSeen:   lastSeen.Format(time.RFC3339),
			IsCurrent:  candidate.ClientUUID == currentClientUUID,
		}
		existing, ok := byDeviceID[deviceID]
		if !ok || lastSeen.After(existing.lastSeen) {
			byDeviceID[deviceID] = deviceState{device: next, lastSeen: lastSeen}
			continue
		}
		if next.IsCurrent {
			existing.device.IsCurrent = true
			byDeviceID[deviceID] = existing
		}
	}
	devices := make([]ActiveDevice, 0, len(byDeviceID))
	for _, state := range byDeviceID {
		devices = append(devices, state.device)
	}
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].LastSeen > devices[j].LastSeen
	})
	return devices
}

func spaceUUIDSet(spaces []DashDataSpace) map[string]struct{} {
	allowed := make(map[string]struct{}, len(spaces))
	for _, space := range spaces {
		spaceUUID := strings.TrimSpace(space.UUID)
		if spaceUUID == "" {
			continue
		}
		allowed[spaceUUID] = struct{}{}
	}
	return allowed
}

func pruneClientFromSpaceLocked(host *Host, target *Client, spaceUUID string) {
	if host == nil || target == nil || strings.TrimSpace(spaceUUID) == "" {
		return
	}
	if space, ok := host.Spaces[spaceUUID]; ok {
		space.mu.Lock()
		delete(space.Users, target.Conn)
		space.mu.Unlock()
	}

	subs := host.SpaceSubscriptions[target.Conn]
	if len(subs) == 0 {
		return
	}
	filtered := make([]string, 0, len(subs))
	for _, sub := range subs {
		if sub != spaceUUID {
			filtered = append(filtered, sub)
		}
	}
	if len(filtered) == 0 {
		delete(host.SpaceSubscriptions, target.Conn)
		return
	}
	host.SpaceSubscriptions[target.Conn] = filtered
}

func removeSpaceStateLocked(host *Host, spaceUUID string) {
	if host == nil || strings.TrimSpace(spaceUUID) == "" {
		return
	}

	if space, ok := host.Spaces[spaceUUID]; ok {
		space.mu.Lock()
		for conn := range space.Users {
			delete(space.Users, conn)
		}
		space.mu.Unlock()
	}

	for conn, subs := range host.SpaceSubscriptions {
		filtered := make([]string, 0, len(subs))
		for _, sub := range subs {
			if sub != spaceUUID {
				filtered = append(filtered, sub)
			}
		}
		if len(filtered) == 0 {
			delete(host.SpaceSubscriptions, conn)
			continue
		}
		host.SpaceSubscriptions[conn] = filtered
	}

	for channelUUID, mappedSpaceUUID := range host.ChannelToSpace {
		if mappedSpaceUUID != spaceUUID {
			continue
		}
		delete(host.ChannelToSpace, channelUUID)
		if channel, ok := host.Channels[channelUUID]; ok {
			channel.mu.Lock()
			for conn := range channel.Users {
				delete(channel.Users, conn)
				if current := host.ChannelSubscriptions[conn]; current == channelUUID {
					delete(host.ChannelSubscriptions, conn)
				}
			}
			channel.mu.Unlock()
			delete(host.Channels, channelUUID)
		}
	}

	delete(host.Spaces, spaceUUID)
}

func requireSpaceCapability(
	client *Client,
	spaceUUID string,
	channelUUID string,
	capabilityToken string,
	requiredScope string,
	unauthorizedError string,
) bool {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return false
	}

	host.mu.Lock()
	signingPublicKey := host.SigningPublicKey
	host.mu.Unlock()

	if err := verifySpaceCapability(
		client,
		client.HostUUID,
		signingPublicKey,
		spaceUUID,
		channelUUID,
		capabilityToken,
		requiredScope,
		time.Now().UTC(),
	); err != nil {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: unauthorizedError},
		})
		return false
	}
	return true
}

func resolveChannelSpaceUUID(client *Client, channelUUID string, fallbackSpaceUUID string) (string, error) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		return "", fmt.Errorf("host not found")
	}

	host.mu.Lock()
	mappedSpaceUUID, mapped := host.ChannelToSpace[channelUUID]
	host.mu.Unlock()

	fallbackSpaceUUID = strings.TrimSpace(fallbackSpaceUUID)
	if mapped {
		if fallbackSpaceUUID != "" && fallbackSpaceUUID != mappedSpaceUUID {
			return "", fmt.Errorf("space mismatch")
		}
		return mappedSpaceUUID, nil
	}
	if fallbackSpaceUUID == "" {
		return "", fmt.Errorf("unknown channel mapping")
	}
	return fallbackSpaceUUID, nil
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
		var activeDevices []ActiveDevice
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
				allowedSpaces := spaceUUIDSet(data.Spaces)
				currentSubs := append([]string(nil), host.SpaceSubscriptions[joinClient.Conn]...)
				for _, subscribedSpaceUUID := range currentSubs {
					if _, stillAllowed := allowedSpaces[subscribedSpaceUUID]; !stillAllowed {
						pruneClientFromSpaceLocked(host, joinClient, subscribedSpaceUUID)
					}
				}
				activeDevices = activeDevicesForPublicKeyLocked(host, joinClient.PublicKey, joinClient.ClientUUID)
			}
		}
		for _, space := range data.Spaces {
			if _, ok := host.Spaces[space.UUID]; !ok {
				host.Spaces[space.UUID] = &Space{Users: make(map[*websocket.Conn]int)}
			}
			for _, channel := range space.Channels {
				host.ChannelToSpace[channel.UUID] = space.UUID
			}
		}
		host.mu.Unlock()

		SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
			Type: "dash_data_payload",
			Data: GetDashDataSuccess{
				User:          data.User,
				Spaces:        data.Spaces,
				Invites:       data.Invites,
				Capabilities:  data.Capabilities,
				ActiveDevices: activeDevices,
			},
		})
		return
	}

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "dash_data_payload",
		Data: GetDashDataSuccess{
			User:         data.User,
			Spaces:       data.Spaces,
			Invites:      data.Invites,
			Capabilities: data.Capabilities,
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
	if joinClient == nil {
		host.mu.Unlock()
		log.Printf("SendToClient: client state missing\n")
		return
	}
	if _, ok := host.Spaces[data.Space.UUID]; !ok {
		host.Spaces[data.Space.UUID] = &Space{Users: make(map[*websocket.Conn]int)}
	}
	for _, channel := range data.Space.Channels {
		host.ChannelToSpace[channel.UUID] = data.Space.UUID
	}
	host.mu.Unlock()

	joinSpace(joinClient, data.Space.UUID)

	SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
		Type: "create_space_success",
		Data: CreateSpaceSuccess{
			Space:      data.Space,
			Capability: data.Capability,
		},
	})

}

func handleDeleteSpace(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[DeleteSpaceClient](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid delete space response data"}})
		return
	}
	if !requireSpaceCapability(client, data.UUID, "", data.CapabilityToken, scopeDeleteSpace, "Unauthorized space access") {
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "delete_space_request",
		Data: DeleteSpaceRequest{
			UUID:                      data.UUID,
			RequesterUserID:           client.UserID,
			RequesterUserPublicKey:    client.PublicKey,
			RequesterUserEncPublicKey: client.EncPublicKey,
			ClientUUID:                client.ClientUUID,
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
	spaceUUID := data["SpaceUUID"]

	if spaceUUID != "" {
		if host, exists := GetHost(client.HostUUID); exists {
			host.mu.Lock()
			removeSpaceStateLocked(host, spaceUUID)
			host.mu.Unlock()
		}
	}

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
	if !requireSpaceCapability(client, data.SpaceUUID, "", data.CapabilityToken, scopeCreateChannel, "Unauthorized channel create") {
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "create_channel_request",
		Data: CreateChannelRequest{
			Name:                      data.Name,
			SpaceUUID:                 data.SpaceUUID,
			RequesterUserID:           client.UserID,
			RequesterUserPublicKey:    client.PublicKey,
			RequesterUserEncPublicKey: client.EncPublicKey,
			ClientUUID:                client.ClientUUID,
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
	spaceUUID, err := resolveChannelSpaceUUID(client, data.UUID, data.SpaceUUID)
	if err != nil {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unauthorized channel access"},
		})
		return
	}
	if !requireSpaceCapability(client, spaceUUID, data.UUID, data.CapabilityToken, scopeDeleteChannel, "Unauthorized channel access") {
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "delete_channel_request",
		Data: DeleteChannelRequest{
			UUID:                      data.UUID,
			RequesterUserID:           client.UserID,
			RequesterUserPublicKey:    client.PublicKey,
			RequesterUserEncPublicKey: client.EncPublicKey,
			ClientUUID:                client.ClientUUID,
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
	if !requireSpaceCapability(client, data.SpaceUUID, "", data.CapabilityToken, scopeInviteUser, "Unauthorized invite access") {
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "invite_user_request",
		Data: InviteUserRequest{
			PublicKey:                 data.PublicKey,
			SpaceUUID:                 data.SpaceUUID,
			RequesterUserID:           client.UserID,
			RequesterUserPublicKey:    client.PublicKey,
			RequesterUserEncPublicKey: client.EncPublicKey,
			ClientUUID:                client.ClientUUID,
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
			UserID:           client.UserID,
			UserPublicKey:    client.PublicKey,
			UserEncPublicKey: client.EncPublicKey,
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
	if joinClient == nil {
		host.mu.Unlock()
		log.Printf("SendToClient: client state missing\n")
		return
	}
	if _, ok := host.Spaces[data.Space.UUID]; !ok {
		host.Spaces[data.Space.UUID] = &Space{Users: make(map[*websocket.Conn]int)}
	}
	for _, channel := range data.Space.Channels {
		host.ChannelToSpace[channel.UUID] = data.Space.UUID
	}
	host.mu.Unlock()

	joinSpace(joinClient, data.Space.UUID)

	// send to invited client
	SendToClient(client.HostUUID, joinClient.ClientUUID, WSMessage{
		Type: "accept_invite_success",
		Data: AcceptInviteSuccess{
			SpaceUserID: data.SpaceUserID,
			User:        data.User,
			Space:       data.Space,
			Capability:  data.Capability,
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
			UserID:           client.UserID,
			UserPublicKey:    client.PublicKey,
			UserEncPublicKey: client.EncPublicKey,
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
			UserID:           client.UserID,
			UserPublicKey:    client.PublicKey,
			UserEncPublicKey: client.EncPublicKey,
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

	host.mu.Lock()
	leaveConn, ok := host.ClientConnsByUUID[data.ClientUUID]
	if ok {
		leaveClient := host.ClientsByConn[leaveConn]
		if leaveClient != nil {
			pruneClientFromSpaceLocked(host, leaveClient, data.SpaceUUID)
		}
	}
	host.mu.Unlock()

}

func joinChannel(client *Client, channelUUID string, capabilityToken string) {
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
	spaceUUID, mapped := host.ChannelToSpace[channelUUID]
	if !mapped {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Channel is not accessible"},
		})
		return
	}
	space, spaceExists := host.Spaces[spaceUUID]
	if !spaceExists {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Space is not accessible"},
		})
		return
	}
	signingPublicKey := host.SigningPublicKey
	space.mu.Lock()
	_, isMember := space.Users[client.Conn]
	space.mu.Unlock()
	host.mu.Unlock()

	if err := verifySpaceCapability(
		client,
		client.HostUUID,
		signingPublicKey,
		spaceUUID,
		channelUUID,
		capabilityToken,
		scopeJoinChannel,
		time.Now().UTC(),
	); err != nil {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unauthorized channel join"},
		})
		return
	}

	if !isMember {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unauthorized channel join"},
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
		capabilityToken := ""
		if data.CapabilityTokens != nil {
			capabilityToken = data.CapabilityTokens[uuid]
		}
		if !requireSpaceCapability(client, uuid, "", capabilityToken, scopeReadHistory, "Unauthorized space access") {
			return
		}
		if !joinSpace(client, uuid) {
			safeSend(client, conn, WSMessage{
				Type: "error",
				Data: ChatError{Content: "One or more spaces are not accessible"},
			})
			return
		}
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
	if !requireSpaceCapability(client, data.SpaceUUID, "", data.CapabilityToken, scopeRemoveSpaceUser, "Unauthorized space access") {
		return
	}

	SendToAuthor(client, WSMessage{
		Type: "remove_space_user_request",
		Data: RemoveSpaceUserRequest{
			SpaceUUID:                 data.SpaceUUID,
			UserID:                    data.UserID,
			UserPublicKey:             data.UserPublicKey,
			UserEncPublicKey:          data.UserEncPublicKey,
			RequesterUserID:           client.UserID,
			RequesterUserPublicKey:    client.PublicKey,
			RequesterUserEncPublicKey: client.EncPublicKey,
			ClientUUID:                client.ClientUUID,
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
	if ok && removeClient != nil {
		pruneClientFromSpaceLocked(host, removeClient, data.SpaceUUID)
	} else {
		ok = false
	}
	host.mu.Unlock()

	if ok {
		// tell connected user to leave
		SendToClient(client.HostUUID, removeClient.ClientUUID, WSMessage{
			Type: "leave_space_success",
			Data: "",
		})
	}

}

func handleChatMessage(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[ChatData](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid chat message data"}})
		return
	}
	if !allowChatMessage(client.ClientUUID, time.Now().UTC()) {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Chat rate limit exceeded"}})
		return
	}
	if err := validateEnvelopeForRelay(data.Envelope); err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: err.Error()}})
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
	if !ok {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Failed to connect to the channel"},
		})
		return
	}
	spaceUUID, mapped := host.ChannelToSpace[channelUUID]
	if !mapped {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unknown channel mapping"},
		})
		return
	}
	space, spaceExists := host.Spaces[spaceUUID]
	if !spaceExists {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unauthorized space access"},
		})
		return
	}
	space.mu.Lock()
	_, isMember := space.Users[conn]
	space.mu.Unlock()
	signingPublicKey := host.SigningPublicKey
	host.mu.Unlock()
	if err := verifySpaceCapability(
		client,
		client.HostUUID,
		signingPublicKey,
		spaceUUID,
		channelUUID,
		data.CapabilityToken,
		scopeSendMessage,
		time.Now().UTC(),
	); err != nil {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unauthorized channel access"},
		})
		return
	}
	if !isMember {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unauthorized channel access"},
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
	spaceUUID, mapped := host.ChannelToSpace[channelUUID]
	if !mapped {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unknown channel mapping"},
		})
		return
	}
	space, spaceExists := host.Spaces[spaceUUID]
	if !spaceExists {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unauthorized space access"},
		})
		return
	}
	space.mu.Lock()
	_, isMember := space.Users[conn]
	space.mu.Unlock()
	signingPublicKey := host.SigningPublicKey
	host.mu.Unlock()
	if err := verifySpaceCapability(
		client,
		client.HostUUID,
		signingPublicKey,
		spaceUUID,
		channelUUID,
		data.CapabilityToken,
		scopeReadHistory,
		time.Now().UTC(),
	); err != nil {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unauthorized channel access"},
		})
		return
	}
	if !isMember {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{Content: "Unauthorized channel access"},
		})
		return
	}

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
