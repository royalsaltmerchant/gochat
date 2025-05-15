package main

import (
	"gochat/db"
)

func AppendspaceChannelsAndUsers(space *DashDataSpace) {
	// Fetch channels (existing code)
	channelsQuery := `SELECT id, uuid, name, space_uuid FROM channels WHERE space_uuid = ?`
	channelRows, err := db.ChatDB.Query(channelsQuery, space.UUID)
	if err == nil {
		defer channelRows.Close()
		var channels []DashDataChannel
		for channelRows.Next() {
			var channel DashDataChannel
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
	userRows, err := db.ChatDB.Query(usersQuery, space.UUID)
	var users []DashDataUser
	if err == nil {
		defer userRows.Close()
		for userRows.Next() {
			var user DashDataUser
			if err := userRows.Scan(&user.ID, &user.Username); err == nil {
				users = append(users, user)
			}
		}
	}

	// Check if author is already in users
	found := false
	for _, user := range users {
		if user.ID == space.AuthorID {
			found = true
			break
		}
	}

	// Fetch and append author if not in list
	if !found {
		authorQuery := `SELECT id, username FROM users WHERE id = ?`
		row := db.ChatDB.QueryRow(authorQuery, space.AuthorID)
		var author DashDataUser
		if err := row.Scan(&author.ID, &author.Username); err == nil {
			users = append(users, author)
		}
	}

	space.Users = users
}

func GetUserSpaces(userID int) ([]DashDataSpace, error) {
	query := `
		SELECT DISTINCT s.id, s.uuid, s.name, s.author_id
		FROM spaces s
		LEFT JOIN space_users su ON su.space_uuid = s.uuid
		WHERE s.author_id = ?
			 OR (su.user_id = ? AND su.joined = 1)
	`

	rows, err := db.ChatDB.Query(query, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userSpaces []DashDataSpace
	for rows.Next() {
		var space DashDataSpace
		if err := rows.Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID); err != nil {
			continue
		}
		userSpaces = append(userSpaces, space)
	}

	return userSpaces, nil
}
