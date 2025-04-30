package chatroom

import (
	"fmt"
	"log"
	"net/http"
	"sync"

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

var ChatRooms = map[string]*ChatRoom{
	"general": {Users: make(map[*websocket.Conn]string)},
	"random":  {Users: make(map[*websocket.Conn]string)},
}

// List all chat rooms
func ListChatRooms(c *gin.Context) {
	var rooms []string
	for name := range ChatRooms {
		rooms = append(rooms, name)
	}
	userEmail, _ := c.Get("userEmail")
	c.JSON(200, gin.H{"rooms": rooms, "userEmail": userEmail})
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

	// Extract chat room and username from the URL query parameters
	roomName := c.DefaultQuery("room", "")
	username := c.DefaultQuery("username", "")

	// Ensure the room exists
	chatRoom, exists := ChatRooms[roomName]
	if !exists {
		log.Println("Error: Chat room does not exist")
		return
	}

	// Add user to the chat room
	chatRoom.mu.Lock()
	chatRoom.Users[conn] = username
	chatRoom.mu.Unlock()

	// Send a welcome message to the new user
	conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Welcome %s to the %s chat room!", username, roomName)))

	// Broadcast the new user's arrival to all users in the room
	chatRoom.mu.Lock()
	for userConn := range chatRoom.Users {
		if userConn != conn {
			userConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("%s has joined the chat", username)))
		}
	}
	chatRoom.mu.Unlock()

	// Handle incoming messages from the user
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}

		// Broadcast the message to everyone in the chat room
		chatRoom.mu.Lock()
		for userConn := range chatRoom.Users {
			userConn.WriteMessage(websocket.TextMessage, message)
		}
		chatRoom.mu.Unlock()
	}

	// Remove the user from the room on disconnect
	chatRoom.mu.Lock()
	delete(chatRoom.Users, conn)
	chatRoom.mu.Unlock()

	// Notify others that the user has left the room
	chatRoom.mu.Lock()
	for userConn := range chatRoom.Users {
		userConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("%s has left the chat", username)))
	}
	chatRoom.mu.Unlock()
}
