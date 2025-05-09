package spaces

import (
	"gochat/db"
	auth "gochat/users"
)

func AppendspaceChannelsAndUsers(space *Space) {
	// Fetch channels (existing code)
	channelsQuery := `SELECT id, uuid, name, space_uuid FROM channels WHERE space_uuid = ?`
	channelRows, err := db.DB.Query(channelsQuery, space.UUID)
	if err == nil {
		defer channelRows.Close()
		var channels []Channel
		for channelRows.Next() {
			var channel Channel
			if err := channelRows.Scan(&channel.ID, &channel.UUID, &channel.Name, &channel.SpaceUUID); err == nil {
				channels = append(channels, channel)
			}
		}
		space.Channels = channels
	}

	// Fetch users in space
	usersQuery := `
						SELECT u.id, u.username
						FROM space_users su
						JOIN users u ON su.user_id = u.id
						WHERE su.space_uuid = ?
					`
	userRows, err := db.DB.Query(usersQuery, space.UUID)
	if err == nil {
		defer userRows.Close()
		var users []auth.UserData
		for userRows.Next() {
			var user auth.UserData
			if err := userRows.Scan(&user.ID, &user.Username); err == nil {
				users = append(users, user)
			}
		}
		space.Users = users
	}
}
