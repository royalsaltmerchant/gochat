package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var Hosts = map[string]*Host{}
var hostsMu sync.Mutex

func decodeData[T any](raw interface{}) (T, error) {
	var data T
	bytes, err := json.Marshal(raw)
	if err != nil {
		return data, err
	}
	err = json.Unmarshal(bytes, &data)
	return data, err
}

func HandleSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	conn.SetReadLimit(256 * 1024)
	defer conn.Close()

	clientIP := c.ClientIP()
	var client *Client

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msgBytes, &wsMsg); err != nil {
			log.Println("Invalid message format:", err)
			continue
		}

		if wsMsg.Type == "join_host" {
			data, err := decodeData[JoinHost](wsMsg.Data)
			if err != nil {
				conn.WriteJSON(WSMessage{Type: "join_error", Data: ChatError{Content: "Invalid host uuid"}})
				continue
			}

			host, err := registerOrCreateHost(data.UUID, data.ID, conn)
			if err != nil {
				conn.WriteJSON(WSMessage{Type: "join_error", Data: ChatError{Content: "Failed to join host"}})
				continue
			}
			isHostAuthor := data.ID != "" && data.ID == host.AuthorID
			if !isHostAuthor {
				if err := ensureHostResponsive(host, 2*time.Second); err != nil {
					conn.WriteJSON(WSMessage{Type: "join_error", Data: ChatError{Content: "Host is offline or unresponsive"}})
					continue
				}
			}
			client = registerClient(host, conn, clientIP, isHostAuthor)
			continue
		}

		if client == nil {
			conn.WriteJSON(WSMessage{Type: "error", Data: ChatError{Content: "Client not initialized"}})
			continue
		}

		dispatchMessage(client, conn, wsMsg)
	}

	if client != nil {
		cleanupClient(client)
	}
}

func cleanupClient(client *Client) {
	leaveChannel(client)
	leaveAllSpaces(client)

	if client.IsAuthenticated {
		UnregisterAuthenticatedIP(client.IP)
	}

	host, exists := GetHost(client.HostUUID)
	if !exists {
		return
	}

	host.mu.Lock()
	delete(host.ClientsByConn, client.Conn)
	delete(host.ClientConnsByUUID, client.ClientUUID)
	delete(host.ClientsByUserID, client.UserID)
	if client.IsHostAuthor {
		delete(host.ConnByAuthorID, host.AuthorID)
	}
	if client.PublicKey != "" {
		delete(host.ClientsByPublicKey, client.PublicKey)
	}
	host.mu.Unlock()
	clearChatMessageLimiter(client.ClientUUID)

	close(client.SendQueue)
	close(client.Done)
}

func GetHost(uuid string) (*Host, bool) {
	hostsMu.Lock()
	defer hostsMu.Unlock()
	host, ok := Hosts[uuid]
	return host, ok
}

func safeSend(client *Client, conn *websocket.Conn, msg WSMessage) {
	if client != nil && client.SendQueue != nil {
		select {
		case client.SendQueue <- msg:
		default:
			log.Printf("safeSend: send queue full for client")
			close(client.SendQueue)
		}
	} else {
		_ = conn.WriteJSON(msg)
	}
}

func SendToClient(hostUUID, clientUUID string, msg WSMessage) {
	host, exists := GetHost(hostUUID)
	if !exists {
		log.Printf("SendToClient: host %s not found\n", hostUUID)
		return
	}

	host.mu.Lock()
	conn, ok := host.ClientConnsByUUID[clientUUID]
	if !ok {
		host.mu.Unlock()
		return
	}

	client, ok := host.ClientsByConn[conn]
	if !ok {
		host.mu.Unlock()
		return
	}
	host.mu.Unlock()

	safeSend(client, conn, msg)
}

func SendToAuthor(client *Client, msg WSMessage) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		safeSend(client, client.Conn, WSMessage{Type: "author_error", Data: ChatError{Content: "Failed to connect to the host"}})
		return
	}

	host.mu.Lock()
	hostConn, ok := host.ConnByAuthorID[host.AuthorID]
	if !ok {
		host.mu.Unlock()
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{Type: "author_error", Data: ChatError{Content: "Failed to connect to the host"}})
		return
	}
	authorClient := host.ClientsByConn[hostConn]
	host.mu.Unlock()

	safeSend(authorClient, hostConn, msg)
}

func BroadcastToChannel(hostUUID, channelUUID string, msg WSMessage) {
	host, exists := GetHost(hostUUID)
	if !exists {
		return
	}
	host.mu.Lock()
	defer host.mu.Unlock()
	channel, exists := host.Channels[channelUUID]
	if !exists {
		return
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()

	for conn := range channel.Users {
		client := host.ClientsByConn[conn]
		safeSend(client, conn, msg)
	}
}

func BroadcastToSpace(hostUUID, spaceUUID string, msg WSMessage) {
	host, exists := GetHost(hostUUID)
	if !exists {
		return
	}
	host.mu.Lock()
	defer host.mu.Unlock()
	space, exists := host.Spaces[spaceUUID]
	if !exists {
		return
	}
	space.mu.Lock()
	defer space.mu.Unlock()
	for conn := range space.Users {
		client := host.ClientsByConn[conn]
		safeSend(client, conn, msg)
	}
}

func joinSpace(client *Client, spaceUUID string) bool {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		return false
	}

	host.mu.Lock()
	defer host.mu.Unlock()
	space, ok := host.Spaces[spaceUUID]
	if !ok {
		return false
	}
	space.mu.Lock()
	space.Users[client.Conn] = client.UserID
	space.mu.Unlock()
	subs := host.SpaceSubscriptions[client.Conn]
	for _, existing := range subs {
		if existing == spaceUUID {
			return true
		}
	}
	host.SpaceSubscriptions[client.Conn] = append(subs, spaceUUID)
	return true
}

func leaveAllSpaces(client *Client) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		return
	}

	host.mu.Lock()
	defer host.mu.Unlock()
	spaceUUIDs, ok := host.SpaceSubscriptions[client.Conn]
	if !ok {
		return
	}
	for _, spaceUUID := range spaceUUIDs {
		if space, exists := host.Spaces[spaceUUID]; exists {
			space.mu.Lock()
			delete(space.Users, client.Conn)
			space.mu.Unlock()
		}
	}
	delete(host.SpaceSubscriptions, client.Conn)
}

func leaveSpace(host *Host, client *Client, spaceUUID string) {
	host.mu.Lock()
	defer host.mu.Unlock()
	space, ok := host.Spaces[spaceUUID]
	if !ok {
		return
	}
	space.mu.Lock()
	delete(space.Users, client.Conn)
	space.mu.Unlock()
}
