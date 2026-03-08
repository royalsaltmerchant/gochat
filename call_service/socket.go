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

type CallRoom struct {
	ID           string
	CreatorID    int    // user ID if logged in, 0 if anonymous
	Tier         string // "free" or "premium"
	CreatedAt    time.Time
	MaxDuration  time.Duration // 40min for free, up to 6hr for premium
	TimerDone    chan struct{} // signal to stop the timer goroutine
	Participants map[*websocket.Conn]*CallRoomParticipant
	mu           sync.Mutex
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
				select {
				case p.SendQueue <- WSMessage{
					Type: "call_time_warning",
					Data: CallTimeWarning{
						RoomID:      room.ID,
						SecondsLeft: 300,
					},
				}:
				default:
				}
			}
			room.mu.Unlock()

		case <-expireTimer.C:
			room.mu.Lock()
			for conn, p := range room.Participants {
				select {
				case p.SendQueue <- WSMessage{
					Type: "call_time_expired",
					Data: CallTimeExpired{RoomID: room.ID},
				}:
				default:
				}
				close(p.SendQueue)
				close(p.Done)
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
	SendQueue   chan WSMessage
	Done        chan struct{}
}

func (p *CallRoomParticipant) WritePump() {
	defer p.Conn.Close()
	for {
		select {
		case msg, ok := <-p.SendQueue:
			if !ok {
				return
			}
			if err := p.Conn.WriteJSON(msg); err != nil {
				log.Println("CallRoomParticipant WritePump error:", err)
				return
			}
		case <-p.Done:
			return
		}
	}
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
	defer conn.Close()

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
			_ = conn.WriteJSON(WSMessage{
				Type: "error",
				Data: ChatError{Content: "Unknown websocket message type"},
			})
		}
	}

	// Clean up any call room participation on disconnect.
	cleanupCallRoomParticipant(conn)
}
