package main

import (
	"encoding/json"
	"gochat/db"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return isWebSocketOriginAllowed(r.Header.Get("Origin"))
	},
}

// Call Rooms (standalone video calls)
var CallRooms = map[string]*CallRoom{}
var callRoomsMu sync.Mutex
var socketClients = map[*websocket.Conn]*SocketClient{}
var socketClientsMu sync.RWMutex

type CallRoom struct {
	ID            string
	CreatorID     int    // user ID if logged in, 0 if anonymous
	Tier          string // "free" or "premium"
	CreatedAt     time.Time
	MaxDuration   time.Duration // 40min for free, up to 6hr for premium
	TimerDone     chan struct{} // signal to stop the timer goroutine
	Participants  map[*websocket.Conn]*CallRoomParticipant
	mu            sync.Mutex
	timerDoneOnce sync.Once
}

func startRoomTimer(room *CallRoom) {
	warningDuration := room.MaxDuration - 5*time.Minute
	if warningDuration < 0 {
		warningDuration = 0
	}

	warningTimer := time.NewTimer(warningDuration)
	expireTimer := time.NewTimer(room.MaxDuration)
	defer warningTimer.Stop()
	defer expireTimer.Stop()

	for {
		select {
		case <-warningTimer.C:
			room.mu.Lock()
			for _, p := range room.Participants {
				sendToParticipant(p, WSMessage{
					Type: "call_time_warning",
					Data: CallTimeWarning{
						RoomID:      room.ID,
						SecondsLeft: 300,
					},
				})
			}
			room.mu.Unlock()

		case <-expireTimer.C:
			room.mu.Lock()
			for conn, p := range room.Participants {
				sendToParticipant(p, WSMessage{
					Type: "call_time_expired",
					Data: CallTimeExpired{RoomID: room.ID},
				})
				delete(room.Participants, conn)
			}
			room.mu.Unlock()

			if room.Tier == "premium" && room.CreatorID > 0 {
				actualMinutes := int(time.Since(room.CreatedAt).Minutes()) + 1
				deductCredits(room.CreatorID, actualMinutes)
			}

			callRoomsMu.Lock()
			delete(CallRooms, room.ID)
			callRoomsMu.Unlock()
			return

		case <-room.TimerDone:
			log.Printf("Room ended (all left): id=%s tier=%s creatorID=%d elapsed=%v", room.ID, room.Tier, room.CreatorID, time.Since(room.CreatedAt))
			if room.Tier == "premium" && room.CreatorID > 0 {
				actualMinutes := int(time.Since(room.CreatedAt).Minutes()) + 1
				log.Printf("Deducting %d minutes from user %d", actualMinutes, room.CreatorID)
				deductCredits(room.CreatorID, actualMinutes)
			}
			return
		}
	}
}

func deductCredits(userID int, minutes int) {
	query := `UPDATE users SET credit_minutes = MAX(0, credit_minutes - ?) WHERE id = ? AND subscription_status != 'active'`
	db.HostDB.Exec(query, minutes, userID)
}

type CallRoomParticipant struct {
	ID          string
	DisplayName string
	StreamID    string
	IsAudioOn   bool
	IsVideoOn   bool
	Conn        *websocket.Conn
	Client      *SocketClient
}

type SocketClient struct {
	Conn      *websocket.Conn
	SendQueue chan WSMessage
	Done      chan struct{}
	closeOnce sync.Once
}

func (c *SocketClient) WritePump() {
	defer c.Close()
	for {
		select {
		case msg := <-c.SendQueue:
			if err := c.Conn.WriteJSON(msg); err != nil {
				log.Println("SocketClient WritePump error:", err)
				return
			}
		case <-c.Done:
			return
		}
	}
}

func (c *SocketClient) Close() {
	c.closeOnce.Do(func() {
		close(c.Done)
		_ = c.Conn.Close()
	})
}

func registerSocketClient(conn *websocket.Conn) *SocketClient {
	client := &SocketClient{
		Conn:      conn,
		SendQueue: make(chan WSMessage, 64),
		Done:      make(chan struct{}),
	}

	socketClientsMu.Lock()
	socketClients[conn] = client
	socketClientsMu.Unlock()

	go client.WritePump()
	return client
}

func getSocketClient(conn *websocket.Conn) *SocketClient {
	socketClientsMu.RLock()
	client := socketClients[conn]
	socketClientsMu.RUnlock()
	return client
}

func unregisterSocketClient(conn *websocket.Conn) {
	socketClientsMu.Lock()
	client := socketClients[conn]
	delete(socketClients, conn)
	socketClientsMu.Unlock()

	if client != nil {
		client.Close()
	}
}

func enqueueSocketClientMessage(client *SocketClient, msg WSMessage) bool {
	if client == nil {
		return false
	}

	select {
	case <-client.Done:
		return false
	default:
	}

	select {
	case client.SendQueue <- msg:
		return true
	case <-client.Done:
		return false
	default:
		log.Printf("socket client send queue full")
		return false
	}
}

func sendToConn(conn *websocket.Conn, msg WSMessage) bool {
	return enqueueSocketClientMessage(getSocketClient(conn), msg)
}

func sendToParticipant(participant *CallRoomParticipant, msg WSMessage) bool {
	if participant == nil {
		return false
	}
	return enqueueSocketClientMessage(participant.Client, msg)
}

func signalRoomTimerDone(room *CallRoom) {
	room.timerDoneOnce.Do(func() {
		close(room.TimerDone)
	})
}

// decodeData decodes WSMessage.Data into a typed struct.
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
	registerSocketClient(conn)
	defer func() {
		cleanupCallRoomParticipant(conn)
		unregisterSocketClient(conn)
	}()

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
		case "join_call_room", "leave_call_room", "update_call_media", "update_call_stream_id":
			dispatchCallRoomMessage(conn, wsMsg)
		default:
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{Content: "Unknown websocket message type"},
			})
		}
	}
}
