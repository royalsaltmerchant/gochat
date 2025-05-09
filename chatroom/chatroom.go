package chatroom

import (
	"database/sql"
	"encoding/json"
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
	Users map[*websocket.Conn]int
	mu    sync.Mutex
}

type Client struct {
	Conn     *websocket.Conn
	Username string
	UserID   int
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
var ClientsByConn = map[*websocket.Conn]*Client{}
var ConnsByUserID = map[int]*websocket.Conn{}

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

	userIDRaw, exists := c.Get("userID")
	if !exists {
		log.Println("No userID in context")
		return
	}
	userID, ok := userIDRaw.(int)
	if !ok {
		log.Println("userID is not an int")
		return
	}

	client := &Client{
		Conn:     conn,
		Username: username,
		UserID:   userID,
	}

	ClientsByConn[conn] = client
	ConnsByUserID[userID] = conn

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
			leaveRoom(conn)
			joinRoom(conn, userID, roomUUID)

		case "leave":
			leaveRoom(conn)

		case "chat":
			msgContent, ok := wsMsg.Data.(string)
			if !ok {
				log.Println("Invalid chat data")
				continue
			}

			msgTimestamp := time.Now().UTC()
			msgUserID := sql.NullInt64{
				Int64: int64(userID),
				Valid: true,
			}

			roomUUID := ClientSubscriptions[conn]
			if roomUUID != "" {
				go spaces.InsertMessage(roomUUID, msgContent, username, msgUserID, msgTimestamp.Format(time.RFC3339))

				BroadcastToRoom(roomUUID, WSMessage{
					Type: "chat",
					Data: ChatPayload{
						Username:  username,
						Content:   msgContent,
						Timestamp: msgTimestamp,
					},
				})
			}

		default:
			log.Println("Unknown message type:", wsMsg.Type)
		}
	}

	leaveRoom(conn)
}

func joinRoom(conn *websocket.Conn, userID int, roomUUID string) {
	if _, ok := ChatRooms[roomUUID]; !ok {
		ChatRooms[roomUUID] = &ChatRoom{Users: make(map[*websocket.Conn]int)}
	}
	room := ChatRooms[roomUUID]

	room.mu.Lock()
	room.Users[conn] = userID
	room.mu.Unlock()

	ClientSubscriptions[conn] = roomUUID
}

func leaveRoom(conn *websocket.Conn) {
	roomUUID, ok := ClientSubscriptions[conn]
	if ok {
		if room, exists := ChatRooms[roomUUID]; exists {
			room.mu.Lock()
			delete(room.Users, conn)
			room.mu.Unlock()
		}
		delete(ClientSubscriptions, conn)
	}

	client, exists := ClientsByConn[conn]
	if exists {
		delete(ConnsByUserID, client.UserID)
		delete(ClientsByConn, conn)
	}
}

func BroadcastToRoom(roomUUID string, msg WSMessage) {
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

// SendToUser sends a message to a user by userID from outside WebSocket flow
func SendToUser(userID int, msg WSMessage) {
	conn, ok := ConnsByUserID[userID]
	if !ok {
		log.Printf("User %d not connected", userID)
		return
	}

	jsonBytes, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, jsonBytes); err != nil {
		log.Printf("Failed to send message to user %d: %v", userID, err)
	}
}
