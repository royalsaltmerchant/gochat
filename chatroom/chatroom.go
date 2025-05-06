package chatroom

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"gochat/spaces"
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
	mu    sync.Mutex
}

type WSMessage struct {
	Type string      `json:"type"` // "system", "chat", "join", "leave"
	Data interface{} `json:"data"`
}

type ChatPayload struct {
	Username  string    `json:"Username"`
	Content   string    `json:"Content"`
	Timestamp time.Time `json:"Timestamp"`
}

var ChatRooms = map[string]*ChatRoom{}

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

	// Create chat room if it doesn't exist
	chatRoom, exists := ChatRooms[roomUUID]
	if !exists {
		log.Println("Creating new chat room for:", roomUUID)
		chatRoom = &ChatRoom{Users: make(map[*websocket.Conn]string)}
		ChatRooms[roomUUID] = chatRoom
	}

	// Get username from context. This should be stored from Authentication middleware
	usernameRaw, exists := c.Get("userUsername")
	if !exists {
		log.Println("Error: Username does not exist")
		return
	}
	username, ok := usernameRaw.(string)
	if !ok {
		log.Println("Error: Username is not a string")
		return
	}

	// Add user to the chat room
	chatRoom.mu.Lock()
	chatRoom.Users[conn] = username
	chatRoom.mu.Unlock()

	msgUsername := "System"
	msgContent := fmt.Sprintf("%s has joined the chat", username)
	msgTimestamp := time.Now().UTC()

	msg := WSMessage{
		Type: "join",
		Data: ChatPayload{
			Username:  msgUsername,
			Content:   msgContent,
			Timestamp: msgTimestamp,
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

		msgUsername = username
		msgContent = string(message)
		msgTimestamp = time.Now().UTC()

		userIDRaw, exists := c.Get("userID")
		if !exists {
			fmt.Println("Failed to retrieve user ID")
		}

		userIDInt, ok := userIDRaw.(int)
		if !ok {
			fmt.Println("Failed to convert user ID to int")
		}

		msgUserID := sql.NullInt64{
			Int64: int64(userIDInt),
			Valid: true,
		}

		// Store message in DB
		go spaces.InsertMessage(roomUUID, msgContent, msgUsername, msgUserID, msgTimestamp.Format(time.RFC3339))

		msg = WSMessage{
			Type: "chat",
			Data: ChatPayload{
				Username:  msgUsername,
				Content:   msgContent,
				Timestamp: msgTimestamp,
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
	msgUsername = "System"
	msgContent = fmt.Sprintf("%s has left the chat", username)
	msgTimestamp = time.Now().UTC()

	msg = WSMessage{
		Type: "leave",
		Data: ChatPayload{
			Username:  msgUsername,
			Content:   msgContent,
			Timestamp: msgTimestamp,
		},
	}
	jsonBytes, _ = json.Marshal(msg)

	chatRoom.mu.Lock()
	for userConn := range chatRoom.Users {
		userConn.WriteMessage(websocket.TextMessage, jsonBytes)
	}
	chatRoom.mu.Unlock()
}
