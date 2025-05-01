package chatroom

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections (you can add custom logic here)
	},
}

// Struct for chat rooms and their users
type ChatRoom struct {
	Users map[*websocket.Conn]string
	Name  string
	mu    sync.Mutex
}

type WSMessage struct {
	Type string      `json:"type"` // "system", "chat", "join", "leave"
	Data interface{} `json:"data"`
}

type ChatPayload struct {
	Username  string    `json:"username"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

var ChatRooms = map[string]*ChatRoom{
	"a3f2b19e-879f-4b8f-9c16-b8ecf3a7cf5b": {Users: make(map[*websocket.Conn]string), Name: "First"},
	"5d3ce2f4-b2a0-4a74-a91f-2a0c9b568f42": {Users: make(map[*websocket.Conn]string), Name: "Second"},
}

// List all chat rooms with their UUID and Name
func ListChatRooms(c *gin.Context) {
	type RoomInfo struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}

	var rooms []RoomInfo
	for uuid, room := range ChatRooms {
		rooms = append(rooms, RoomInfo{
			UUID: uuid,
			Name: room.Name,
		})
	}

	userEmail, _ := c.Get("userEmail")

	c.JSON(200, gin.H{
		"rooms":     rooms,
		"userEmail": userEmail,
	})
}

// Join a specific chat room and handle WebSocket communication
func JoinChatRoom(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Error upgrading connection:", err)
		return
	}
	defer conn.Close()

	// Extract room from the URL query parameters
	roomUUID := c.Param("uuid")
	// Ensure the room exists
	chatRoom, exists := ChatRooms[roomUUID]
	if !exists {
		log.Println("Error: Chat room does not exist")
		return
	}

	// Get username from context. This should be stored from Authentication middleware
	usernameRaw, exists := c.Get("userUsername")
	if !exists {
		log.Println("Error: Username does not exist")
	}
	username, ok := usernameRaw.(string)
	if !ok {
		log.Println("Error: Username is not a string")
	}

	// Add user to the chat room
	chatRoom.mu.Lock()
	chatRoom.Users[conn] = username
	chatRoom.mu.Unlock()

	msg := WSMessage{
		Type: "join",
		Data: ChatPayload{
			Username:  "System",
			Message:   fmt.Sprintf("%s has joined the chat", username),
			Timestamp: time.Now().UTC(),
		},
	}
	jsonBytes, _ := json.Marshal(msg)

	// Broadcast the new user's arrival to all users in the room
	chatRoom.mu.Lock()
	for userConn := range chatRoom.Users {
		userConn.WriteMessage(websocket.TextMessage, jsonBytes)
	}
	chatRoom.mu.Unlock()

	// Handle incoming messages from the user
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}

		msg = WSMessage{
			Type: "chat",
			Data: ChatPayload{
				Username:  username,
				Message:   string(message),
				Timestamp: time.Now().UTC(),
			},
		}

		jsonBytes, _ := json.Marshal(msg)

		// Broadcast the message to everyone in the chat room
		chatRoom.mu.Lock()
		for userConn := range chatRoom.Users {
			userConn.WriteMessage(websocket.TextMessage, jsonBytes)
		}
		chatRoom.mu.Unlock()
	}

	// Remove the user from the room on disconnect
	chatRoom.mu.Lock()
	delete(chatRoom.Users, conn)
	chatRoom.mu.Unlock()

	// Notify others that the user has left the room
	chatRoom.mu.Lock()
	msg = WSMessage{
		Type: "leave",
		Data: ChatPayload{
			Username:  "System",
			Message:   fmt.Sprintf("%s has left the chat", username),
			Timestamp: time.Now().UTC(),
		},
	}
	jsonBytes, _ = json.Marshal(msg)

	for userConn := range chatRoom.Users {
		userConn.WriteMessage(websocket.TextMessage, jsonBytes)
	}
	chatRoom.mu.Unlock()
}
