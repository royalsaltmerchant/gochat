package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

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

var voiceClients = map[string]*VoiceClient{}
var voiceClientsMu sync.Mutex

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

func HandleSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	// Capture client IP at connection time
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

		log.Println(wsMsg.Type)

		if wsMsg.Type == "join_host" {
			data, err := decodeData[JoinHost](wsMsg.Data)
			if err != nil {
				conn.WriteJSON(WSMessage{
					Type: "join_error",
					Data: ChatError{Content: "Invalid host uuid"},
				})
				continue
			}

			host, err := registerOrCreateHost(data.UUID, data.ID, conn)
			if err != nil {
				conn.WriteJSON(WSMessage{
					Type: "join_error",
					Data: ChatError{Content: "Failed to join host"},
				})
				continue
			}
			client = registerClient(host, conn, clientIP)
			continue
		}

		// All other messages
		if client == nil {
			conn.WriteJSON(WSMessage{
				Type: "error",
				Data: ChatError{Content: "Client not initialized"},
			})
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
	leaveAllSpaces(client) // ← This must run before locking host.mu

	voiceClientsMu.Lock()
	if vc, ok := voiceClients[client.ClientUUID]; ok {
		if vc.Peer != nil {
			_ = vc.Peer.Close()
		}
		delete(voiceClients, client.ClientUUID)
	}
	voiceClientsMu.Unlock()

	// Unregister authenticated IP session
	if client.IsAuthenticated {
		UnregisterAuthenticatedIP(client.IP)
	}

	host, exists := GetHost(client.HostUUID)
	if !exists {
		return
	}

	// NOW safe to lock
	host.mu.Lock()
	delete(host.ClientsByConn, client.Conn)
	delete(host.ClientConnsByUUID, client.ClientUUID)
	delete(host.ClientsByUserID, client.UserID)
	host.mu.Unlock()

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
		conn.WriteJSON(msg) // fallback for early-stage sends
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
		log.Printf("SendToClient: clientUUID %s not found in host %s\n", clientUUID, hostUUID)
		host.mu.Unlock()
		return
	}

	client, ok := host.ClientsByConn[conn]
	if !ok {
		log.Printf("SendToClient: connection for clientUUID %s not found in ClientsByConn\n", clientUUID)
		host.mu.Unlock()
		return
	}
	host.mu.Unlock()

	// Log type of message being sent (optional)
	log.Printf("SendToClient: sending to clientUUID=%s, type=%s\n", clientUUID, msg.Type)

	safeSend(client, conn, msg)
}

func SendToAuthor(client *Client, msg WSMessage) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		safeSend(client, client.Conn, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	hostConn, ok := host.ConnByAuthorID[host.AuthorID]
	if !ok {
		host.mu.Unlock()
		log.Printf("SendToAuthor: author not connected to host")
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}
	authorClient := host.ClientsByConn[hostConn]
	host.mu.Unlock()

	safeSend(authorClient, hostConn, msg)
}

func BroadcastToChannel(hostUUID, channelUUID string, msg WSMessage) {
	host, exists := GetHost(hostUUID)
	if !exists {
		log.Printf("Host %s not found\n", hostUUID)
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
		log.Printf("Host %s not found\n", hostUUID)
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

func joinSpace(client *Client, SpaceUUID string) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		return
	}

	host.mu.Lock()
	defer host.mu.Unlock()
	if _, ok := host.Spaces[SpaceUUID]; !ok {
		host.Spaces[SpaceUUID] = &Space{Users: make(map[*websocket.Conn]int)}
	}
	space := host.Spaces[SpaceUUID]
	space.mu.Lock()
	space.Users[client.Conn] = client.UserID
	space.mu.Unlock()
	host.SpaceSubscriptions[client.Conn] = append(host.SpaceSubscriptions[client.Conn], SpaceUUID)
}

func leaveAllSpaces(client *Client) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("SendToAuthor: host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
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
		log.Printf("Failed to remove host space %s", spaceUUID)
		return
	}
	space.mu.Lock()
	delete(space.Users, client.Conn)
	space.mu.Unlock()
}
