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
type Channel struct {
	Users map[*websocket.Conn]int
	mu    sync.Mutex
}

type Space struct {
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

type NewUserPayload struct {
	ID        int    `json:"ID"`
	Username  string `json:"Username"`
	SpaceUUID string `json:"SpaceUUID"`
}

// Global state
var Channels = map[string]*Channel{}
var Spaces = map[string]*Space{}

var ChannelSubscriptions = map[*websocket.Conn]string{} // for channels
var SpaceSubscriptions = map[*websocket.Conn][]string{} // for spaces
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

	// Join all user spaces on connect
	userSpaces, err := spaces.GetUserSpaces(userID)
	if err != nil {
		log.Println("Failed to get user spaces:", err)
	} else {
		for _, space := range userSpaces {
			joinSpace(conn, userID, space.UUID)
		}
	}

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
		case "join-channel":
			channelUUID, ok := wsMsg.Data.(string)
			if !ok {
				log.Println("Invalid join data")
				continue
			}
			leaveChannel(conn)
			joinChannel(conn, userID, channelUUID)

		case "leave-channel":
			leaveChannel(conn)

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

			channelUUID := ChannelSubscriptions[conn]
			if channelUUID != "" {
				go spaces.InsertMessage(channelUUID, msgContent, username, msgUserID, msgTimestamp.Format(time.RFC3339))

				BroadcastToChannel(channelUUID, WSMessage{
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

	leaveChannel(conn)
	leaveAllSpaces(conn)
}

// ======== Channel Functions ========

func joinChannel(conn *websocket.Conn, userID int, channelUUID string) {
	if _, ok := Channels[channelUUID]; !ok {
		Channels[channelUUID] = &Channel{Users: make(map[*websocket.Conn]int)}
	}
	channel := Channels[channelUUID]

	channel.mu.Lock()
	channel.Users[conn] = userID
	channel.mu.Unlock()

	ChannelSubscriptions[conn] = channelUUID
}

func leaveChannel(conn *websocket.Conn) {
	channelUUID, ok := ChannelSubscriptions[conn]
	if ok {
		if channel, exists := Channels[channelUUID]; exists {
			channel.mu.Lock()
			delete(channel.Users, conn)
			channel.mu.Unlock()
		}
		delete(ChannelSubscriptions, conn)
	}

	client, exists := ClientsByConn[conn]
	if exists {
		delete(ConnsByUserID, client.UserID)
		delete(ClientsByConn, conn)
	}
}

func BroadcastToChannel(channelUUID string, msg WSMessage) {
	channel, exists := Channels[channelUUID]
	if !exists {
		return
	}

	jsonBytes, _ := json.Marshal(msg)

	channel.mu.Lock()
	defer channel.mu.Unlock()
	for conn := range channel.Users {
		conn.WriteMessage(websocket.TextMessage, jsonBytes)
	}
}

// ======== Space Functions ========

func joinSpace(conn *websocket.Conn, userID int, spaceUUID string) {
	if _, ok := Spaces[spaceUUID]; !ok {
		Spaces[spaceUUID] = &Space{Users: make(map[*websocket.Conn]int)}
	}
	space := Spaces[spaceUUID]

	space.mu.Lock()
	space.Users[conn] = userID
	space.mu.Unlock()

	SpaceSubscriptions[conn] = append(SpaceSubscriptions[conn], spaceUUID)
}

func leaveAllSpaces(conn *websocket.Conn) {
	spaceUUIDs, ok := SpaceSubscriptions[conn]
	if !ok {
		return
	}

	for _, spaceUUID := range spaceUUIDs {
		if space, exists := Spaces[spaceUUID]; exists {
			space.mu.Lock()
			delete(space.Users, conn)
			space.mu.Unlock()
		}
	}

	delete(SpaceSubscriptions, conn)
}

func BroadcastToSpace(spaceUUID string, msg WSMessage) {
	space, exists := Spaces[spaceUUID]
	if !exists {
		return
	}

	jsonBytes, _ := json.Marshal(msg)

	space.mu.Lock()
	defer space.mu.Unlock()
	for conn := range space.Users {
		conn.WriteMessage(websocket.TextMessage, jsonBytes)
	}
}

// ======== SendToUser ========

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
