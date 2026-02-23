package main

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Host struct {
	UUID                 string
	SigningPublicKey     string
	AuthorConn           *websocket.Conn
	ClientsByConn        map[*websocket.Conn]*Client
	ClientConnsByUUID    map[string]*websocket.Conn
	ClientsByUserID      map[int]*Client
	ClientsByPublicKey   map[string]*Client
	Channels             map[string]*Channel
	ChannelToSpace       map[string]string
	Spaces               map[string]*Space
	ChannelSubscriptions map[*websocket.Conn]string
	SpaceSubscriptions   map[*websocket.Conn][]string
	mu                   sync.Mutex
}

type Channel struct {
	Users map[*websocket.Conn]int
	mu    sync.Mutex
}

type Space struct {
	Users map[*websocket.Conn]int
	mu    sync.Mutex
}

type Client struct {
	Conn              *websocket.Conn
	Username          string
	UserID            int
	HostUUID          string
	ClientUUID        string
	PublicKey         string
	EncPublicKey      string
	DeviceID          string
	DeviceName        string
	LastSeen          time.Time
	AuthChallenge     string
	HostAuthChallenge string
	IP                string
	IsHostCandidate   bool
	IsHostAuthor      bool
	IsAuthenticated   bool
	SendQueue         chan WSMessage
	Done              chan struct{}
}

func (c *Client) WritePump() {
	defer c.Conn.Close()

	for {
		select {
		case msg, ok := <-c.SendQueue:
			if !ok {
				return
			}

			if err := c.Conn.WriteJSON(msg); err != nil {
				log.Println("WritePump error:", err)
				return
			}
		case <-c.Done:
			return
		}
	}
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type JoinHost struct {
	UUID string `json:"uuid"`
	Role string `json:"role,omitempty"`
}

type JoinUUID struct {
	UUID            string `json:"uuid"`
	CapabilityToken string `json:"capability_token,omitempty"`
}

type ChatData struct {
	Envelope        map[string]interface{} `json:"envelope"`
	CapabilityToken string                 `json:"capability_token,omitempty"`
}

type ChatPayload struct {
	Envelope  map[string]interface{} `json:"envelope"`
	Timestamp time.Time              `json:"timestamp"`
}

type ChatError struct {
	Content    string `json:"error"`
	ClientUUID string `json:"client_uuid"`
}

type UpdateUsernameClient struct {
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	Username         string `json:"username"`
}

type UpdateUsernameRequest struct {
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	Username         string `json:"username"`
	ClientUUID       string `json:"client_uuid"`
}

type UpdateUsernameResponse struct {
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	Username         string `json:"username"`
	ClientUUID       string `json:"client_uuid"`
}

type UpdateUsernameSuccess struct {
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	Username         string `json:"username"`
}

type DashDataUser struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PublicKey    string `json:"public_key,omitempty"`
	EncPublicKey string `json:"enc_public_key,omitempty"`
}

type DashDataChannel struct {
	ID         int    `json:"id"`
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	SpaceUUID  string `json:"space_uuid"`
	AllowVoice int    `json:"allow_voice"`
}

type DashDataSpace struct {
	ID       int               `json:"id"`
	UUID     string            `json:"uuid"`
	Name     string            `json:"name"`
	AuthorID int               `json:"author_id"`
	Channels []DashDataChannel `json:"channels"`
	Users    []DashDataUser    `json:"users"`
}

type DashDataInvite struct {
	ID            int    `json:"id"`
	SpaceUUID     string `json:"space_uuid"`
	UserID        int    `json:"user_id"`
	UserPublicKey string `json:"user_public_key,omitempty"`
	Joined        int    `json:"joined"`
	Name          string `json:"name"`
}

type SpaceCapability struct {
	SpaceUUID string   `json:"space_uuid"`
	Token     string   `json:"token"`
	Scopes    []string `json:"scopes"`
	ExpiresAt int64    `json:"expires_at"`
}

type SpaceCapabilityClaims struct {
	Version      int      `json:"v"`
	HostUUID     string   `json:"host_uuid"`
	SpaceUUID    string   `json:"space_uuid"`
	SubjectKey   string   `json:"sub"`
	Scopes       []string `json:"scopes"`
	ExpiresAt    int64    `json:"exp"`
	IssuedAt     int64    `json:"iat"`
	TokenID      string   `json:"jti"`
	ChannelScope string   `json:"channel_scope"`
}

type GetDashDataRequest struct {
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	Username         string `json:"username"`
	ClientUUID       string `json:"client_uuid"`
}

type GetDashDataResponse struct {
	User         DashDataUser      `json:"user"`
	Spaces       []DashDataSpace   `json:"spaces"`
	Invites      []DashDataInvite  `json:"invites"`
	Capabilities []SpaceCapability `json:"capabilities,omitempty"`
	ClientUUID   string            `json:"client_uuid"`
}

type GetDashDataSuccess struct {
	User          DashDataUser      `json:"user"`
	Spaces        []DashDataSpace   `json:"spaces"`
	Invites       []DashDataInvite  `json:"invites"`
	Capabilities  []SpaceCapability `json:"capabilities,omitempty"`
	ActiveDevices []ActiveDevice    `json:"active_devices,omitempty"`
}

type ActiveDevice struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	LastSeen   string `json:"last_seen"`
	IsCurrent  bool   `json:"is_current"`
}

type CreateSpaceClient struct {
	Name string `json:"name"`
}

type CreateSpaceRequest struct {
	Name             string `json:"name"`
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	Username         string `json:"username,omitempty"`
	ClientUUID       string `json:"client_uuid"`
}

type CreateSpaceResponse struct {
	Space      DashDataSpace   `json:"space"`
	Capability SpaceCapability `json:"capability,omitempty"`
	ClientUUID string          `json:"client_uuid"`
}

type CreateSpaceSuccess struct {
	Space      DashDataSpace   `json:"space"`
	Capability SpaceCapability `json:"capability,omitempty"`
}

type DeleteSpaceClient struct {
	UUID            string `json:"uuid"`
	CapabilityToken string `json:"capability_token,omitempty"`
}

type DeleteSpaceRequest struct {
	UUID                      string `json:"uuid"`
	RequesterUserID           int    `json:"requester_user_id"`
	RequesterUserPublicKey    string `json:"requester_user_public_key,omitempty"`
	RequesterUserEncPublicKey string `json:"requester_user_enc_public_key,omitempty"`
	ClientUUID                string `json:"client_uuid"`
}

type CreateChannelClient struct {
	Name            string `json:"name"`
	SpaceUUID       string `json:"space_uuid"`
	CapabilityToken string `json:"capability_token,omitempty"`
}

type CreateChannelRequest struct {
	Name                      string `json:"name"`
	SpaceUUID                 string `json:"space_uuid"`
	RequesterUserID           int    `json:"requester_user_id"`
	RequesterUserPublicKey    string `json:"requester_user_public_key,omitempty"`
	RequesterUserEncPublicKey string `json:"requester_user_enc_public_key,omitempty"`
	ClientUUID                string `json:"client_uuid"`
}

type CreateChannelResponse struct {
	Channel    DashDataChannel `json:"channel"`
	SpaceUUID  string          `json:"space_uuid"`
	ClientUUID string          `json:"client_uuid"`
}

type CreateChannelSuccess struct {
	SpaceUUID string          `json:"space_uuid"`
	Channel   DashDataChannel `json:"channel"`
}

type CreateChannelUpdate struct {
	SpaceUUID string          `json:"space_uuid"`
	Channel   DashDataChannel `json:"channel"`
}

type DeleteChannelClient struct {
	UUID            string `json:"uuid"`
	SpaceUUID       string `json:"space_uuid,omitempty"`
	CapabilityToken string `json:"capability_token,omitempty"`
}

type DeleteChannelRequest struct {
	UUID                      string `json:"uuid"`
	RequesterUserID           int    `json:"requester_user_id"`
	RequesterUserPublicKey    string `json:"requester_user_public_key,omitempty"`
	RequesterUserEncPublicKey string `json:"requester_user_enc_public_key,omitempty"`
	ClientUUID                string `json:"client_uuid"`
}

type DeleteChannelResponse struct {
	ID         int    `json:"id"`
	UUID       string `json:"uuid"`
	SpaceUUID  string `json:"space_uuid"`
	ClientUUID string `json:"client_uuid"`
}

type DeleteChannelUpdate struct {
	UUID      string `json:"uuid"`
	SpaceUUID string `json:"space_uuid"`
}

type InviteUserClient struct {
	PublicKey       string `json:"public_key"`
	SpaceUUID       string `json:"space_uuid"`
	CapabilityToken string `json:"capability_token,omitempty"`
}

type InviteUserRequest struct {
	PublicKey                 string `json:"public_key"`
	SpaceUUID                 string `json:"space_uuid"`
	RequesterUserID           int    `json:"requester_user_id"`
	RequesterUserPublicKey    string `json:"requester_user_public_key,omitempty"`
	RequesterUserEncPublicKey string `json:"requester_user_enc_public_key,omitempty"`
	ClientUUID                string `json:"client_uuid"`
}

type InviteUserResponse struct {
	PublicKey     string         `json:"public_key"`
	UserID        int            `json:"user_id"`
	UserPublicKey string         `json:"user_public_key,omitempty"`
	SpaceUUID     string         `json:"space_uuid"`
	Invite        DashDataInvite `json:"invite"`
	ClientUUID    string         `json:"client_uuid"`
}

type InviteUserSuccess struct {
	PublicKey string `json:"public_key"`
}

type InviteUserUpdate struct {
	Invite DashDataInvite `json:"invite"`
}

type AcceptInviteClient struct {
	SpaceUserID      int    `json:"space_user_id"`
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
}

type AcceptInviteRequest struct {
	SpaceUserID      int    `json:"space_user_id"`
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	ClientUUID       string `json:"client_uuid"`
}

type AcceptInviteResponse struct {
	SpaceUserID int             `json:"space_user_id"`
	User        DashDataUser    `json:"user"`
	Space       DashDataSpace   `json:"space"`
	Capability  SpaceCapability `json:"capability,omitempty"`
	ClientUUID  string          `json:"client_uuid"`
}

type AcceptInviteSuccess struct {
	SpaceUserID int             `json:"space_user_id"`
	User        DashDataUser    `json:"user"`
	Space       DashDataSpace   `json:"space"`
	Capability  SpaceCapability `json:"capability,omitempty"`
}

type AcceptInviteUpdate struct {
	SpaceUUID string       `json:"space_uuid"`
	User      DashDataUser `json:"user"`
}

type DeclineInviteClient struct {
	SpaceUserID      int    `json:"space_user_id"`
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
}

type DeclineInviteRequest struct {
	SpaceUserID      int    `json:"space_user_id"`
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	ClientUUID       string `json:"client_uuid"`
}

type DeclineInviteResponse struct {
	SpaceUserID   int    `json:"space_user_id"`
	UserID        int    `json:"user_id"`
	UserPublicKey string `json:"user_public_key,omitempty"`
	ClientUUID    string `json:"client_uuid"`
}

type DeclineInviteSuccess struct {
	SpaceUserID   int    `json:"space_user_id"`
	UserID        int    `json:"user_id"`
	UserPublicKey string `json:"user_public_key,omitempty"`
}

type LeaveSpaceClient struct {
	SpaceUUID        string `json:"space_uuid"`
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
}

type LeaveSpaceRequest struct {
	SpaceUUID        string `json:"space_uuid"`
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	ClientUUID       string `json:"client_uuid"`
}

type LeaveSpaceResponse struct {
	SpaceUUID     string `json:"space_uuid"`
	UserID        int    `json:"user_id"`
	UserPublicKey string `json:"user_public_key,omitempty"`
	ClientUUID    string `json:"client_uuid"`
}

type LeaveSpaceUpdate struct {
	SpaceUUID     string `json:"space_uuid"`
	UserID        int    `json:"user_id"`
	UserPublicKey string `json:"user_public_key,omitempty"`
}

type JoinAllSpacesClient struct {
	SpaceUUIDs       []string          `json:"space_uuids"`
	CapabilityTokens map[string]string `json:"capability_tokens,omitempty"`
}

type RemoveSpaceUserClient struct {
	SpaceUUID        string `json:"space_uuid"`
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	CapabilityToken  string `json:"capability_token,omitempty"`
}

type RemoveSpaceUserRequest struct {
	SpaceUUID                 string `json:"space_uuid"`
	UserID                    int    `json:"user_id"`
	UserPublicKey             string `json:"user_public_key,omitempty"`
	UserEncPublicKey          string `json:"user_enc_public_key,omitempty"`
	RequesterUserID           int    `json:"requester_user_id"`
	RequesterUserPublicKey    string `json:"requester_user_public_key,omitempty"`
	RequesterUserEncPublicKey string `json:"requester_user_enc_public_key,omitempty"`
	ClientUUID                string `json:"client_uuid"`
}

type RemoveSpaceUserResponse struct {
	SpaceUUID     string `json:"space_uuid"`
	UserID        int    `json:"user_id"`
	UserPublicKey string `json:"user_public_key,omitempty"`
	ClientUUID    string `json:"client_uuid"`
}

type RemoveSpaceUserUpdate struct {
	SpaceUUID     string `json:"space_uuid"`
	UserID        int    `json:"user_id"`
	UserPublicKey string `json:"user_public_key,omitempty"`
}

type SaveChatMessageRequest struct {
	UserID           int                    `json:"user_id"`
	UserPublicKey    string                 `json:"user_public_key,omitempty"`
	UserEncPublicKey string                 `json:"user_enc_public_key,omitempty"`
	Username         string                 `json:"username,omitempty"`
	ChannelUUID      string                 `json:"channel_uuid"`
	Envelope         map[string]interface{} `json:"envelope"`
}

type GetMessagesClient struct {
	BeforeUnixTime  string `json:"before_unix_time"`
	CapabilityToken string `json:"capability_token,omitempty"`
}

type GetMessagesRequest struct {
	ChannelUUID    string `json:"channel_uuid"`
	ClientUUID     string `json:"client_uuid"`
	BeforeUnixTime string `json:"before_unix_time"`
}

type GetMessagesMessage struct {
	ID               int                    `json:"id"`
	ChannelUUID      string                 `json:"channel_uuid"`
	Username         string                 `json:"username"`
	Envelope         map[string]interface{} `json:"envelope"`
	UserID           int                    `json:"user_id"`
	UserPublicKey    string                 `json:"user_public_key,omitempty"`
	UserEncPublicKey string                 `json:"user_enc_public_key,omitempty"`
	Timestamp        string                 `json:"timestamp"`
}

type GetMessagesResponse struct {
	Messages        []GetMessagesMessage `json:"messages"`
	HasMoreMessages bool                 `json:"has_more_messages"`
	ChannelUUID     string               `json:"channel_uuid"`
	ClientUUID      string               `json:"client_uuid"`
}

type GetMessagesSuccess struct {
	Messages        []GetMessagesMessage `json:"messages"`
	HasMoreMessages bool                 `json:"has_more_messages"`
	ChannelUUID     string               `json:"channel_uuid"`
}

type ClientHost struct {
	ID               int    `json:"id"`
	UUID             string `json:"uuid"`
	Name             string `json:"name"`
	SigningPublicKey string `json:"signing_public_key,omitempty"`
	Online           bool   `json:"online"`
}

type UUIDListRequest struct {
	UUIDs []string `json:"uuids"`
}

type AuthPubKeyClient struct {
	PublicKey    string `json:"public_key"`
	EncPublicKey string `json:"enc_public_key"`
	DeviceID     string `json:"device_id,omitempty"`
	DeviceName   string `json:"device_name,omitempty"`
	Username     string `json:"username"`
	Challenge    string `json:"challenge"`
	Signature    string `json:"signature"`
}

type AuthChallenge struct {
	Challenge string `json:"challenge"`
}

type HostAuthChallenge struct {
	Challenge string `json:"challenge"`
}

type HostAuthClient struct {
	Challenge string `json:"challenge"`
	Signature string `json:"signature"`
}

type RelayHealthCheck struct {
	Nonce string `json:"nonce"`
}

type RelayHealthCheckAck struct {
	Nonce string `json:"nonce"`
}

type AuthPubKeySuccess struct {
	UserID       int    `json:"user_id"`
	Username     string `json:"username"`
	PublicKey    string `json:"public_key"`
	EncPublicKey string `json:"enc_public_key"`
}
