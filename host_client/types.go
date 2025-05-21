package main

type UserData struct {
	ID       int
	Username string
	Email    string
	Password string
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
	ID        int
	UUID      string
	Name      string
	SpaceUUID string
}

type Message struct {
	ID          int
	ChannelUUID string
	Content     string
	UserID      int
	Timestamp   string
}

type Host struct {
	ID       int
	UUID     string
	Name     string
	AuthorID string
}

type SpaceUser struct {
	ID        int
	SpaceUUID string
	UserID    int
	Joined    int
	Name      string
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
	ID   string `json:"id"`
}

type ApproveLoginUser struct {
	Password   string `json:"password"`
	Email      string `json:"email"`
	AuthorID   string `json:"author_id"`
	ClientUUID string `json:"client_uuid"`
}

type ApproveLoginUserByToken struct {
	Token      string `json:"token"`
	ClientUUID string `json:"client_uuid"`
}

type ApprovedLoginUser struct {
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	ClientUUID string `json:"client_uuid"`
	Token      string `json:"token"`
}

type ApproveRegisterUser struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"` // already hashed
	ClientUUID string `json:"client_uuid"`
}

type UpdateUsernameRequest struct {
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	ClientUUID string `json:"client_uuid"`
}

type UpdateUsernameResponse struct {
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	Token      string `json:"token"`
	ClientUUID string `json:"client_uuid"`
}

type GetDashDataRequest struct {
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	ClientUUID string `json:"client_uuid"`
}

type DashDataUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type DashDataChannel struct {
	ID        int    `json:"id"`
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	SpaceUUID string `json:"space_uuid"`
}

type DashDataInvite struct {
	ID        int    `json:"id"`
	SpaceUUID string `json:"space_uuid"`
	UserID    int    `json:"user_id"`
	Joined    int    `json:"joined"`
	Name      string `json:"name"`
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
	User       DashDataUser     `json:"user"`
	Spaces     []DashDataSpace  `json:"spaces"`
	Invites    []DashDataInvite `json:"invites"`
	ClientUUID string           `json:"client_uuid"`
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

type DeleteSpaceRequest struct {
	UUID       string `json:"uuid"`
	ClientUUID string `json:"client_uuid"`
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

type DeclineInviteRequest struct {
	SpaceUserID int    `json:"space_user_id"`
	UserID      int    `json:"user_id"`
	ClientUUID  string `json:"client_uuid"`
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

type RemoveSpaceUserRequest struct {
	SpaceUUID  string `json:"space_uuid"`
	UserID     int    `json:"user_id"`
	ClientUUID string `json:"client_uuid"`
}

type SaveChatMessageRequest struct {
	UserID      int    `json:"user_id"`
	ChannelUUID string `json:"channel_uuid"`
	Content     string `json:"content"`
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
