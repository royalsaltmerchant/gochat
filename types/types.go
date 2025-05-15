package types

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
	SpaceUUID string // space UUID
}

type Message struct {
	ID          int
	ChannelUUID string
	Content     string
	Username    string
	UserID      int
	Timestamp   string
}

type Host struct {
	ID       int
	UUID     string
	Name     string
	AuthorID string
}
