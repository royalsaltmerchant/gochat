package main

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

func isAuthorPassthroughType(msgType string) bool {
	switch msgType {
	case "get_dash_data_response",
		"update_username_response",
		"create_space_response",
		"delete_space_response",
		"create_channel_response",
		"delete_channel_response",
		"invite_user_success",
		"accept_invite_success",
		"decline_invite_success",
		"leave_space_success",
		"remove_space_user_success",
		"get_messages_response",
		"relay_health_check_ack",
		"error":
		return true
	default:
		return false
	}
}

func allowPreAuthMessage(client *Client, msgType string) bool {
	if msgType == "auth_pubkey" || msgType == "host_auth" {
		return true
	}
	return client.IsHostAuthor && isAuthorPassthroughType(msgType)
}

func touchClientLastSeen(client *Client) {
	if client == nil || client.HostUUID == "" {
		return
	}
	host, exists := GetHost(client.HostUUID)
	if !exists {
		return
	}
	host.mu.Lock()
	client.LastSeen = time.Now().UTC()
	host.mu.Unlock()
}

func dispatchMessage(client *Client, conn *websocket.Conn, wsMsg WSMessage) {
	if !client.IsAuthenticated && !allowPreAuthMessage(client, wsMsg.Type) {
		safeSend(client, conn, WSMessage{
			Type: "authentication-error",
			Data: ChatError{Content: "Authentication required"},
		})
		return
	}
	if client.IsAuthenticated {
		touchClientLastSeen(client)
	}

	switch wsMsg.Type {
	case "auth_pubkey":
		handleAuthPubKey(client, conn, &wsMsg)
	case "host_auth":
		handleHostAuth(client, conn, &wsMsg)
	case "get_dash_data":
		handleGetDashData(client, conn)
	case "get_dash_data_response":
		handleGetDashDatRes(client, conn, &wsMsg)
	case "update_username":
		handleUpdateUsername(client, conn, &wsMsg)
	case "update_username_response":
		handleUpdateUsernameRes(client, conn, &wsMsg)
	case "create_space":
		handleCreateSpace(client, conn, &wsMsg)
	case "create_space_response":
		handleCreateSpaceRes(client, conn, &wsMsg)
	case "delete_space":
		handleDeleteSpace(client, conn, &wsMsg)
	case "delete_space_response":
		handleDeleteSpaceRes(client, conn, &wsMsg)
	case "create_channel":
		handleCreateChannel(client, conn, &wsMsg)
	case "create_channel_response":
		handleCreateChannelRes(client, conn, &wsMsg)
	case "delete_channel":
		handleDeleteChannel(client, conn, &wsMsg)
	case "delete_channel_response":
		handleDeleteChannelRes(client, conn, &wsMsg)
	case "invite_user":
		handleInviteUser(client, conn, &wsMsg)
	case "invite_user_success":
		handleInviteUserRes(client, conn, &wsMsg)
	case "accept_invite":
		handleAcceptInvite(client, conn, &wsMsg)
	case "accept_invite_success":
		handleAcceptInviteRes(client, conn, &wsMsg)
	case "decline_invite":
		handleDeclineInvite(client, conn, &wsMsg)
	case "decline_invite_success":
		handleDeclineInviteRes(client, conn, &wsMsg)
	case "leave_space":
		handleLeaveSpace(client, conn, &wsMsg)
	case "leave_space_success":
		handleLeaveSpaceRes(client, conn, &wsMsg)
	// case "join_space":
	// 	handleJoinSpace(client, conn, &wsMsg)
	case "join_all_spaces":
		handleJoinAllSpaces(client, conn, &wsMsg)
	case "remove_space_user":
		handleRemoveSpaceUser(client, conn, &wsMsg)
	case "remove_space_user_success":
		handleRemoveSpaceUserRes(client, conn, &wsMsg)
	case "join_channel":
		data, err := decodeData[JoinUUID](wsMsg.Data)
		if err != nil {
			safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid join channel data"}})
			return
		}
		leaveChannel(client)
		joinChannel(client, data.UUID, data.CapabilityToken)
	case "leave_channel":
		leaveChannel(client)
	case "chat":
		handleChatMessage(client, conn, &wsMsg)
	case "get_messages":
		handleGetMessages(client, conn, &wsMsg)
	case "get_messages_response":
		handleGetMessagesRes(client, conn, &wsMsg)
	case "relay_health_check_ack":
		handleRelayHealthCheckAck(client, conn, &wsMsg)
	case "error":
		data, err := decodeData[ChatError](wsMsg.Data)
		if err != nil {
			safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid error data"}})
			return
		}
		SendToClient(client.HostUUID, data.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{
				Content: data.Content,
			},
		})

	default:
		log.Println("Unknown message type:", wsMsg.Type)
	}
}
