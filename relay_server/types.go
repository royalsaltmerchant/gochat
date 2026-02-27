package main

type UserData struct {
	ID       int
	Username string
	Email    string
	Password string
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type ChatError struct {
	Content string `json:"error"`
}

type CallParticipant struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	StreamID    string `json:"stream_id"`
	IsAudioOn   bool   `json:"is_audio_on"`
	IsVideoOn   bool   `json:"is_video_on"`
}

type CallRoomState struct {
	RoomID       string            `json:"room_id"`
	Participants []CallParticipant `json:"participants"`
}

type JoinCallRoomClient struct {
	RoomID      string `json:"room_id"`
	DisplayName string `json:"display_name"`
	StreamID    string `json:"stream_id"`
	Token       string `json:"token,omitempty"`
}

type LeaveCallRoomClient struct {
	RoomID string `json:"room_id"`
}

type CallParticipantJoined struct {
	RoomID      string          `json:"room_id"`
	Participant CallParticipant `json:"participant"`
}

type CallParticipantLeft struct {
	RoomID        string `json:"room_id"`
	ParticipantID string `json:"participant_id"`
}

type UpdateCallMediaClient struct {
	RoomID    string `json:"room_id"`
	IsAudioOn bool   `json:"is_audio_on"`
	IsVideoOn bool   `json:"is_video_on"`
}

type CallMediaUpdated struct {
	RoomID        string `json:"room_id"`
	ParticipantID string `json:"participant_id"`
	IsAudioOn     bool   `json:"is_audio_on"`
	IsVideoOn     bool   `json:"is_video_on"`
}

type UpdateCallStreamIDClient struct {
	RoomID   string `json:"room_id"`
	StreamID string `json:"stream_id"`
}

type CallStreamIDUpdated struct {
	RoomID        string `json:"room_id"`
	ParticipantID string `json:"participant_id"`
	StreamID      string `json:"stream_id"`
}

type CallTimeRemaining struct {
	RoomID         string `json:"room_id"`
	SecondsLeft    int    `json:"seconds_left"`
	Tier           string `json:"tier"`
	MaxDurationSec int    `json:"max_duration_sec"`
}

type CallTimeWarning struct {
	RoomID      string `json:"room_id"`
	SecondsLeft int    `json:"seconds_left"`
}

type CallTimeExpired struct {
	RoomID string `json:"room_id"`
}
