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
						WHERE su.space_uuid = ? AND su.joined = 1
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

func GetUserSpaces(userID int) ([]Space, error) {
	query := `
		SELECT DISTINCT s.id, s.uuid, s.name, s.author_id
		FROM spaces s
		LEFT JOIN space_users su ON su.space_uuid = s.uuid
		WHERE s.author_id = ?
			 OR (su.user_id = ? AND su.joined = 1)
	`

	rows, err := db.DB.Query(query, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userSpaces []Space
	for rows.Next() {
		var space Space
		if err := rows.Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID); err != nil {
			continue
		}
		userSpaces = append(userSpaces, space)
	}

	return userSpaces, nil
}
