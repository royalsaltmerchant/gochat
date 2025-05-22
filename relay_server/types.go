package main

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type Host struct {
	UUID                 string
	AuthorID             string
	AuthorUserID         int
	ClientsByConn        map[*websocket.Conn]*Client
	ClientConnsByUUID    map[string]*websocket.Conn
	ClientsByUserID      map[int]*Client
	ConnByAuthorID       map[string]*websocket.Conn
	Channels             map[string]*Channel
	Spaces               map[string]*Space
	ChannelSubscriptions map[*websocket.Conn]string
	SpaceSubscriptions   map[*websocket.Conn][]string
	mu                   sync.Mutex
}

type UserData struct {
	ID       int
	Username string
	Email    string
	Password string
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
	Conn       *websocket.Conn
	Username   string
	UserID     int
	HostUUID   string
	ClientUUID string
	SendQueue  chan WSMessage // queue for outbound messages
	Done       chan struct{}  // to signal shutdown
}

type VoiceClient struct {
	Conn       *websocket.Conn
	Peer       *webrtc.PeerConnection
	ChannelID  string
	ClientUUID string
}

func (c *Client) WritePump() {
	defer c.Conn.Close()

	for {
		select {
		case msg, ok := <-c.SendQueue:
			if !ok {
				return // channel closed
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

type ChatPayload struct {
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type HostStatusResult struct {
	UUID   string `json:"uuid"`
	Online bool   `json:"online"`
}

type RegisterUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type ApproveRegisterUser struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"` // already hashed
	ClientUUID string `json:"client_uuid"`
}

type LoginUser struct {
	Password string `json:"password"`
	Email    string `json:"email"`
}

type LoginUserByToken struct {
	Token string `json:"token"`
}

type ApproveLoginUser struct {
	Password   string `json:"password"`
	Email      string `json:"email"`
	AuthorID   string `json:"author_id"`
	ClientUUID string `json:"client_uuid"`
}

type ApproveLoginUserByToken struct {
	Token      string `json:"token"`
	AuthorID   string `json:"author_id"`
	ClientUUID string `json:"client_uuid"`
}

type ApprovedLoginUser struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type GetDashDataRequest struct {
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	ClientUUID string `json:"client_uuid"`
}

type LoginUserToken struct {
	Token string `json:"token"`
}

type JoinHost struct {
	UUID string `json:"uuid"`
	ID   string `json:"id"`
}

type JoinUUID struct {
	UUID string `json:"uuid"`
}

type ChatData struct {
	Content string `json:"content"`
}

type ChatError struct {
	Content    string `json:"error"`
	ClientUUID string `json:"client_uuid"`
}

type UpdateUsernameClient struct {
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	ClientUUID string `json:"client_uuid"`
}

type UpdateUsernameRequest struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
}

type UpdateUsernameResponse struct {
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	Token      string `json:"token"`
	ClientUUID string `json:"client_uuid"`
}

type UpdateUsername struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type DashDataUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
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
	ID        int    `json:"id"`
	SpaceUUID string `json:"space_uuid"`
	UserID    int    `json:"user_id"`
	Joined    int    `json:"joined"`
	Name      string `json:"name"`
}

type GetDashDataResponse struct {
	User       DashDataUser     `json:"user"`
	Spaces     []DashDataSpace  `json:"spaces"`
	Invites    []DashDataInvite `json:"invites"`
	ClientUUID string           `json:"client_uuid"`
}

type GetDashDataSuccess struct {
	User    DashDataUser     `json:"user"`
	Spaces  []DashDataSpace  `json:"spaces"`
	Invites []DashDataInvite `json:"invites"`
}

type CreateSpaceClient struct {
	Name string `json:"name"`
}

type CreateSpaceRequest struct {
	Name       string `json:"name"`
	UserID     int    `json:"user_id"`
	ClientUUID string `json:"client_uuid"`
}

type CreateSpaceResponse struct {
	Space      DashDataSpace `json:"space"`
	ClientUUID string        `json:"client_uuid"`
}

type CreateSpaceSuccess struct {
	Space DashDataSpace `json:"space"`
}

type DeleteSpaceClient struct {
	UUID string `json:"uuid"`
}

type DeleteSpaceRequest struct {
	UUID       string `json:"uuid"`
	ClientUUID string `json:"client_uuid"`
}

type CreateChannelClient struct {
	Name      string `json:"name"`
	SpaceUUID string `json:"space_uuid"`
}

type CreateChannelRequest struct {
	Name       string `json:"name"`
	SpaceUUID  string `json:"space_uuid"`
	ClientUUID string `json:"client_uuid"`
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

type DeleteChannelClient struct {
	UUID string `json:"uuid"`
}

type DeleteChannelRequest struct {
	UUID       string `json:"uuid"`
	ClientUUID string `json:"client_uuid"`
}

type DeleteChannelResponse struct {
	ID         int    `json:"id"`
	UUID       string `json:"uuid"`
	SpaceUUID  string `json:"space_uuid"`
	ClientUUID string `json:"client_uuid"`
}

type InviteUserClient struct {
	Email     string `json:"email"`
	SpaceUUID string `json:"space_uuid"`
}

type InviteUserRequest struct {
	Email      string `json:"email"`
	SpaceUUID  string `json:"space_uuid"`
	ClientUUID string `json:"client_uuid"`
}

type InviteUserResponse struct {
	Email      string         `json:"email"`
	UserID     int            `json:"user_id"`
	SpaceUUID  string         `json:"space_uuid"`
	Invite     DashDataInvite `json:"invite"`
	ClientUUID string         `json:"client_uuid"`
}

type InviteUserSuccess struct {
	Email string `json:"email"`
}

type InviteUserUpdate struct {
	Invite DashDataInvite `json:"invite"`
}

type AcceptInviteClient struct {
	SpaceUserID int `json:"space_user_id"`
	UserID      int `json:"user_id"`
}

type AcceptInviteRequest struct {
	SpaceUserID int    `json:"space_user_id"`
	UserID      int    `json:"user_id"`
	ClientUUID  string `json:"client_uuid"`
}

type AcceptInviteResponse struct {
	SpaceUserID int           `json:"space_user_id"`
	User        DashDataUser  `json:"user"`
	Space       DashDataSpace `json:"space"`
	ClientUUID  string        `json:"client_uuid"`
}

type AcceptInviteSuccess struct {
	SpaceUserID int           `json:"space_user_id"`
	User        DashDataUser  `json:"user"`
	Space       DashDataSpace `json:"space"`
}

type AcceptInviteUpdate struct {
	SpaceUUID string       `json:"space_uuid"`
	User      DashDataUser `json:"user"`
}

type DeclineInviteClient struct {
	SpaceUserID int `json:"space_user_id"`
	UserID      int `json:"user_id"`
}

type DeclineInviteRequest struct {
	SpaceUserID int    `json:"space_user_id"`
	UserID      int    `json:"user_id"`
	ClientUUID  string `json:"client_uuid"`
}

type DeclineInviteResponse struct {
	SpaceUserID int    `json:"space_user_id"`
	UserID      int    `json:"user_id"`
	ClientUUID  string `json:"client_uuid"`
}

type DeclineInviteSuccess struct {
	SpaceUserID int `json:"space_user_id"`
	UserID      int `json:"user_id"`
}

type LeaveSpaceClient struct {
	SpaceUUID string `json:"space_uuid"`
	UserID    int    `json:"user_id"`
}

type LeaveSpaceRequest struct {
	SpaceUUID  string `json:"space_uuid"`
	UserID     int    `json:"user_id"`
	ClientUUID string `json:"client_uuid"`
}

type LeaveSpaceResponse struct {
	SpaceUUID  string `json:"space_uuid"`
	UserID     int    `json:"user_id"`
	ClientUUID string `json:"client_uuid"`
}

type LeaveSpaceUpdate struct {
	SpaceUUID string `json:"space_uuid"`
	UserID    int    `json:"user_id"`
}

type JoinSpaceClient struct {
	SpaceUUID string `json:"space_uuid"`
}

type JoinAllSpacesClient struct {
	SpaceUUIDs []string `json:"space_uuids"`
}

type RemoveSpaceUserClient struct {
	SpaceUUID string `json:"space_uuid"`
	UserID    int    `json:"user_id"`
}

type RemoveSpaceUserRequest struct {
	SpaceUUID  string `json:"space_uuid"`
	UserID     int    `json:"user_id"`
	ClientUUID string `json:"client_uuid"`
}

type RemoveSpaceUserResponse struct {
	SpaceUUID  string `json:"space_uuid"`
	UserID     int    `json:"user_id"`
	ClientUUID string `json:"client_uuid"`
}

type RemoveSpaceUserUpdate struct {
	SpaceUUID string `json:"space_uuid"`
	UserID    int    `json:"user_id"`
}

type SaveChatMessageRequest struct {
	UserID      int    `json:"user_id"`
	ChannelUUID string `json:"channel_uuid"`
	Content     string `json:"content"`
}

type GetMessagesClient struct {
	BeforeUnixTime string `json:"before_unix_time"` // optional
}

type GetMessagesRequest struct {
	ChannelUUID    string `json:"channel_uuid"`
	ClientUUID     string `json:"client_uuid"`
	BeforeUnixTime string `json:"before_unix_time"` // optional
}

type GetMessagesMessage struct {
	ID          int    `json:"id"`
	ChannelUUID string `json:"channel_uuid"`
	Username    string `json:"username"`
	Content     string `json:"content"`
	UserID      int    `json:"user_id"`
	Timestamp   string `json:"timestamp"`
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

type TurnCredentialsResponse struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SDPOfferClient struct {
	Offer webrtc.SessionDescription `json:"offer"`
}

type ClientHost struct {
	ID       int    `json:"id"`
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	AuthorID string `json:"author_id"`
	Online   bool   `json:"online"`
}

type UUIDListRequest struct {
	UUIDs []string `json:"uuids"`
}

type ChannelAllowVoiceClient struct {
	UUID  string `json:"uuid"`
	Allow int    `json:"allow"`
}

type ChannelAllowVoiceRequest struct {
	UUID       string `json:"uuid"`
	Allow      int    `json:"allow"`
	ClientUUID string `json:"client_uuid"`
}
