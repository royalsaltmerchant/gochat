package main

import (
	"fmt"
	"gochat/db"
	"log"
	"os"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func lookupUserTier(tokenString string) (int, string, int) {
	jwtSecret := os.Getenv("JWT_SECRET")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return 0, "", 0
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", 0
	}
	userIDFloat, ok := claims["userID"].(float64)
	if !ok {
		return 0, "", 0
	}
	userID := int(userIDFloat)

	var subStatus string
	var credits int
	query := `SELECT COALESCE(subscription_status, 'none'), COALESCE(credit_minutes, 0) FROM users WHERE id = ?`
	err = db.HostDB.QueryRow(query, userID).Scan(&subStatus, &credits)
	if err != nil {
		return 0, "", 0
	}
	return userID, subStatus, credits
}

func dispatchCallRoomMessage(conn *websocket.Conn, wsMsg WSMessage) {
	switch wsMsg.Type {
	case "join_call_room":
		handleJoinCallRoom(conn, &wsMsg)
	case "leave_call_room":
		handleLeaveCallRoom(conn, &wsMsg)
	case "update_call_media":
		handleUpdateCallMedia(conn, &wsMsg)
	case "update_call_stream_id":
		handleUpdateCallStreamID(conn, &wsMsg)
	default:
		log.Println("Unknown call room message type:", wsMsg.Type)
	}
}

func handleJoinCallRoom(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[JoinCallRoomClient](wsMsg.Data)
	if err != nil {
		_ = conn.WriteJSON(WSMessage{Type: "error", Data: ChatError{Content: "Invalid join call room data"}})
		return
	}

	callRoomsMu.Lock()
	room, exists := CallRooms[data.RoomID]
	if !exists {
		tier := "free"
		maxDuration := 40 * time.Minute
		creatorID := 0

		if data.Token != "" {
			userID, subStatus, credits := lookupUserTier(data.Token)
			log.Printf("Room creation: token present, userID=%d subStatus=%q credits=%d", userID, subStatus, credits)
			if userID > 0 {
				creatorID = userID
				if subStatus == "active" {
					tier = "premium"
					maxDuration = 6 * time.Hour
				} else if credits > 0 {
					tier = "premium"
					creditDuration := time.Duration(credits) * time.Minute
					if creditDuration > 6*time.Hour {
						creditDuration = 6 * time.Hour
					}
					maxDuration = creditDuration
				}
			}
		} else {
			log.Printf("Room creation: no token provided, creating free room")
		}

		log.Printf("Room created: id=%s tier=%s creatorID=%d maxDuration=%v", data.RoomID, tier, creatorID, maxDuration)
		room = &CallRoom{
			ID:           data.RoomID,
			CreatorID:    creatorID,
			CreatorToken: data.AnonToken,
			Tier:         tier,
			CreatedAt:    time.Now(),
			MaxDuration:  maxDuration,
			TimerDone:    make(chan struct{}),
			Participants: make(map[*websocket.Conn]*CallRoomParticipant),
		}
		CallRooms[data.RoomID] = room
		go startRoomTimer(room)
	}
	callRoomsMu.Unlock()

	participantID := uuid.New().String()
	participant := &CallRoomParticipant{
		ID:          participantID,
		DisplayName: data.DisplayName,
		StreamID:    data.StreamID,
		IsAudioOn:   true,
		IsVideoOn:   true,
		Conn:        conn,
		SendQueue:   make(chan WSMessage, 64),
		Done:        make(chan struct{}),
	}

	room.mu.Lock()

	// Build current participant list for the new joiner.
	currentParticipants := []CallParticipant{}
	for _, p := range room.Participants {
		currentParticipants = append(currentParticipants, CallParticipant{
			ID:          p.ID,
			DisplayName: p.DisplayName,
			StreamID:    p.StreamID,
			IsAudioOn:   p.IsAudioOn,
			IsVideoOn:   p.IsVideoOn,
		})
	}

	room.Participants[conn] = participant
	room.mu.Unlock()

	go participant.WritePump()

	participant.SendQueue <- WSMessage{
		Type: "call_room_state",
		Data: CallRoomState{
			RoomID:       data.RoomID,
			Participants: currentParticipants,
		},
	}

	participant.SendQueue <- WSMessage{
		Type: "call_room_joined",
		Data: map[string]string{
			"room_id":        data.RoomID,
			"participant_id": participantID,
		},
	}

	sendCallRoomVoiceCredentials(participant, data.RoomID)

	elapsed := time.Since(room.CreatedAt)
	remaining := room.MaxDuration - elapsed
	if remaining < 0 {
		remaining = 0
	}
	participant.SendQueue <- WSMessage{
		Type: "call_time_remaining",
		Data: CallTimeRemaining{
			RoomID:         data.RoomID,
			SecondsLeft:    int(remaining.Seconds()),
			Tier:           room.Tier,
			MaxDurationSec: int(room.MaxDuration.Seconds()),
		},
	}

	newParticipant := CallParticipant{
		ID:          participantID,
		DisplayName: data.DisplayName,
		StreamID:    data.StreamID,
		IsAudioOn:   true,
		IsVideoOn:   true,
	}

	broadcastToCallRoom(room, conn, WSMessage{
		Type: "call_participant_joined",
		Data: CallParticipantJoined{
			RoomID:      data.RoomID,
			Participant: newParticipant,
		},
	})
}

func handleLeaveCallRoom(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[LeaveCallRoomClient](wsMsg.Data)
	if err != nil {
		_ = conn.WriteJSON(WSMessage{Type: "error", Data: ChatError{Content: "Invalid leave call room data"}})
		return
	}

	callRoomsMu.Lock()
	room, exists := CallRooms[data.RoomID]
	callRoomsMu.Unlock()

	if !exists {
		return
	}

	room.mu.Lock()
	participant, ok := room.Participants[conn]
	if !ok {
		room.mu.Unlock()
		return
	}

	participantID := participant.ID
	delete(room.Participants, conn)

	if len(room.Participants) == 0 {
		close(room.TimerDone)
		callRoomsMu.Lock()
		delete(CallRooms, data.RoomID)
		callRoomsMu.Unlock()
	}
	room.mu.Unlock()

	close(participant.SendQueue)
	close(participant.Done)

	broadcastToCallRoom(room, nil, WSMessage{
		Type: "call_participant_left",
		Data: CallParticipantLeft{
			RoomID:        data.RoomID,
			ParticipantID: participantID,
		},
	})
}

func handleUpdateCallMedia(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[UpdateCallMediaClient](wsMsg.Data)
	if err != nil {
		_ = conn.WriteJSON(WSMessage{Type: "error", Data: ChatError{Content: "Invalid update call media data"}})
		return
	}

	callRoomsMu.Lock()
	room, exists := CallRooms[data.RoomID]
	callRoomsMu.Unlock()

	if !exists {
		return
	}

	room.mu.Lock()
	participant, ok := room.Participants[conn]
	if !ok {
		room.mu.Unlock()
		return
	}

	participant.IsAudioOn = data.IsAudioOn
	participant.IsVideoOn = data.IsVideoOn
	participantID := participant.ID
	room.mu.Unlock()

	broadcastToCallRoom(room, nil, WSMessage{
		Type: "call_media_updated",
		Data: CallMediaUpdated{
			RoomID:        data.RoomID,
			ParticipantID: participantID,
			IsAudioOn:     data.IsAudioOn,
			IsVideoOn:     data.IsVideoOn,
		},
	})
}

func handleUpdateCallStreamID(conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[UpdateCallStreamIDClient](wsMsg.Data)
	if err != nil {
		_ = conn.WriteJSON(WSMessage{Type: "error", Data: ChatError{Content: "Invalid update call stream ID data"}})
		return
	}

	callRoomsMu.Lock()
	room, exists := CallRooms[data.RoomID]
	callRoomsMu.Unlock()

	if !exists {
		return
	}

	room.mu.Lock()
	participant, ok := room.Participants[conn]
	if !ok {
		room.mu.Unlock()
		return
	}

	participant.StreamID = data.StreamID
	participantID := participant.ID
	room.mu.Unlock()

	broadcastToCallRoom(room, conn, WSMessage{
		Type: "call_stream_id_updated",
		Data: CallStreamIDUpdated{
			RoomID:        data.RoomID,
			ParticipantID: participantID,
			StreamID:      data.StreamID,
		},
	})
}

func broadcastToCallRoom(room *CallRoom, exclude *websocket.Conn, msg WSMessage) {
	room.mu.Lock()
	defer room.mu.Unlock()

	for conn, participant := range room.Participants {
		if conn == exclude {
			continue
		}
		select {
		case participant.SendQueue <- msg:
		default:
			log.Printf("broadcastToCallRoom: send queue full for participant %s", participant.ID)
		}
	}
}

func cleanupCallRoomParticipant(conn *websocket.Conn) {
	callRoomsMu.Lock()
	defer callRoomsMu.Unlock()

	for roomID, room := range CallRooms {
		room.mu.Lock()
		participant, ok := room.Participants[conn]
		if ok {
			participantID := participant.ID
			delete(room.Participants, conn)

			close(participant.SendQueue)
			close(participant.Done)

			for _, p := range room.Participants {
				select {
				case p.SendQueue <- WSMessage{
					Type: "call_participant_left",
					Data: CallParticipantLeft{
						RoomID:        roomID,
						ParticipantID: participantID,
					},
				}:
				default:
				}
			}

			if len(room.Participants) == 0 {
				close(room.TimerDone)
				delete(CallRooms, roomID)
			}
		}
		room.mu.Unlock()
	}
}

// sendCallRoomVoiceCredentials sends TURN credentials to a call room participant.
func sendCallRoomVoiceCredentials(participant *CallRoomParticipant, roomID string) {
	turnURL := os.Getenv("TURN_URL")
	turnSecret := os.Getenv("TURN_SECRET")
	sfuSecret := os.Getenv("SFU_SECRET")

	if turnURL == "" || turnSecret == "" {
		log.Println("TURN credentials not configured, skipping voice_credentials for call room")
		return
	}

	ttl := int64(8 * 3600)
	turnUsername, turnCredential := generateTurnCredentials(turnSecret, ttl)

	var sfuToken string
	if sfuSecret != "" {
		var err error
		sfuToken, err = GenerateSFUToken(0, participant.DisplayName, roomID, sfuSecret, 8*time.Hour)
		if err != nil {
			log.Printf("Failed to generate SFU token for call room: %v", err)
		}
	}

	participant.SendQueue <- WSMessage{
		Type: "voice_credentials",
		Data: VoiceCredentials{
			TurnURL:        turnURL,
			TurnUsername:   turnUsername,
			TurnCredential: turnCredential,
			SFUToken:       sfuToken,
			ChannelUUID:    roomID,
		},
	}
}
