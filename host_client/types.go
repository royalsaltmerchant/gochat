package main

type UserData struct {
	ID           int
	Username     string
	PublicKey    string
	EncPublicKey string
}

type Space struct {
	ID       int
	UUID     string
	Name     string
	AuthorID int
	Channels []Channel
	Users    []UserData
}

type Channel struct {
	ID         int
	UUID       string
	Name       string
	SpaceUUID  string
	AllowVoice int
}

type Message struct {
	ID          int
	ChannelUUID string
	Content     string
	UserID      int
	Timestamp   string
}

type Host struct {
	ID               int
	UUID             string
	Name             string
	SigningPublicKey string
	Online           int
}

type SpaceUser struct {
	ID            int
	SpaceUUID     string
	UserID        int
	UserPublicKey string
	Joined        int
	Name          string
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type ChatError struct {
	Content    string `json:"error"`
	ClientUUID string `json:"client_uuid"`
}

type JoinHostPayload struct {
	UUID string `json:"uuid"`
	Role string `json:"role,omitempty"`
}

type RelayHealthCheck struct {
	Nonce string `json:"nonce"`
}

type RelayHealthCheckAck struct {
	Nonce string `json:"nonce"`
}

type HostAuthChallenge struct {
	Challenge string `json:"challenge"`
}

type HostAuthClient struct {
	Challenge string `json:"challenge"`
	Signature string `json:"signature"`
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

type GetDashDataRequest struct {
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	Username         string `json:"username"`
	ClientUUID       string `json:"client_uuid"`
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

type DashDataSpace struct {
	ID       int               `json:"id"`
	UUID     string            `json:"uuid"`
	Name     string            `json:"name"`
	AuthorID int               `json:"author_id"`
	Channels []DashDataChannel `json:"channels"`
	Users    []DashDataUser    `json:"users"`
}

type GetDashDataResponse struct {
	User         DashDataUser      `json:"user"`
	Spaces       []DashDataSpace   `json:"spaces"`
	Invites      []DashDataInvite  `json:"invites"`
	Capabilities []SpaceCapability `json:"capabilities,omitempty"`
	ClientUUID   string            `json:"client_uuid"`
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

type DeleteSpaceRequest struct {
	UUID                      string `json:"uuid"`
	RequesterUserID           int    `json:"requester_user_id"`
	RequesterUserPublicKey    string `json:"requester_user_public_key,omitempty"`
	RequesterUserEncPublicKey string `json:"requester_user_enc_public_key,omitempty"`
	ClientUUID                string `json:"client_uuid"`
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

type DeclineInviteRequest struct {
	SpaceUserID      int    `json:"space_user_id"`
	UserID           int    `json:"user_id"`
	UserPublicKey    string `json:"user_public_key,omitempty"`
	UserEncPublicKey string `json:"user_enc_public_key,omitempty"`
	ClientUUID       string `json:"client_uuid"`
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

type SaveChatMessageRequest struct {
	UserID           int                    `json:"user_id"`
	UserPublicKey    string                 `json:"user_public_key,omitempty"`
	UserEncPublicKey string                 `json:"user_enc_public_key,omitempty"`
	Username         string                 `json:"username,omitempty"`
	ChannelUUID      string                 `json:"channel_uuid"`
	Envelope         map[string]interface{} `json:"envelope"`
}

type GetMessagesRequest struct {
	ChannelUUID    string `json:"channel_uuid"`
	ClientUUID     string `json:"client_uuid"`
	BeforeUnixTime string `json:"before_unix_time"` // optional
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

type ChannelAllowVoiceRequest struct {
	UUID       string `json:"uuid"`
	Allow      int    `json:"allow"`
	ClientUUID string `json:"client_uuid"`
}
