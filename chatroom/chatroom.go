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
		return true
	},
}

// Types
type ChatRoom struct {
	Users map[*websocket.Conn]string
	mu    sync.Mutex
}

type Client struct {
	Conn     *websocket.Conn
	Username string
}

type WSMessage struct {
	Type string      `json:"type"` // "join", "leave", "chat"
	Data interface{} `json:"data"`
}

type ChatPayload struct {
	Username  string    `json:"Username"`
	Content   string    `json:"Content"`
	Timestamp time.Time `json:"Timestamp"`
}

// Global state
var ChatRooms = map[string]*ChatRoom{}
var ClientSubscriptions = map[*websocket.Conn]string{}
var Clients = map[*websocket.Conn]*Client{}

func HandleSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	usernameRaw, exists := c.Get("userUsername")
	if !exists {
		log.Println("No username in context")
		return
	}
	username := usernameRaw.(string)

	Clients[conn] = &Client{Conn: conn, Username: username}

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

		switch wsMsg.Type {
		case "join":
			roomUUID, ok := wsMsg.Data.(string)
			if !ok {
				log.Println("Invalid join data")
				continue
			}

			// Leave old room
			leaveRoom(conn)

			// Join new room
			joinRoom(conn, username, roomUUID)

		case "leave":
			leaveRoom(conn)

		case "chat":
			msgContent, ok := wsMsg.Data.(string)
			if !ok {
				log.Println("Invalid chat data")
				continue
			}
			msgTimestamp := time.Now().UTC()

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
			roomUUID := ClientSubscriptions[conn]
			if roomUUID != "" {
				// Store message in DB
				go spaces.InsertMessage(roomUUID, msgContent, username, msgUserID, msgTimestamp.Format(time.RFC3339))

				broadcast(roomUUID, WSMessage{
					Type: "chat",
					Data: ChatPayload{
						Username:  username,
						Content:   msgContent,
						Timestamp: time.Now().UTC(),
					},
				})
			}

		default:
			log.Println("Unknown message type:", wsMsg.Type)
		}
	}

	leaveRoom(conn)
	delete(Clients, conn)
	conn.Close()
}

// Helpers

func joinRoom(conn *websocket.Conn, username, roomUUID string) {
	if _, ok := ChatRooms[roomUUID]; !ok {
		ChatRooms[roomUUID] = &ChatRoom{Users: make(map[*websocket.Conn]string)}
	}
	room := ChatRooms[roomUUID]

	room.mu.Lock()
	room.Users[conn] = username
	room.mu.Unlock()

	ClientSubscriptions[conn] = roomUUID

	broadcast(roomUUID, WSMessage{
		Type: "join",
		Data: ChatPayload{
			Username:  "System",
			Content:   fmt.Sprintf("%s has joined", username),
			Timestamp: time.Now().UTC(),
		},
	})
}

func leaveRoom(conn *websocket.Conn) {
	roomUUID, ok := ClientSubscriptions[conn]
	if !ok {
		return
	}

	room, exists := ChatRooms[roomUUID]
	if !exists {
		return
	}

	username := Clients[conn].Username

	room.mu.Lock()
	delete(room.Users, conn)
	room.mu.Unlock()

	delete(ClientSubscriptions, conn)

	broadcast(roomUUID, WSMessage{
		Type: "leave",
		Data: ChatPayload{
			Username:  "System",
			Content:   fmt.Sprintf("%s has left", username),
			Timestamp: time.Now().UTC(),
		},
	})
}

func broadcast(roomUUID string, msg WSMessage) {
	room, exists := ChatRooms[roomUUID]
	if !exists {
		return
	}

	jsonBytes, _ := json.Marshal(msg)

	room.mu.Lock()
	defer room.mu.Unlock()
	for conn := range room.Users {
		conn.WriteMessage(websocket.TextMessage, jsonBytes)
	}
}
