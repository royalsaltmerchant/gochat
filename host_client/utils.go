package main

import (
	"gochat/db"
	"log"
)

func AppendspaceChannelsAndUsers(space *DashDataSpace) {
	// Fetch channels
	channelsQuery := `SELECT id, uuid, name, space_uuid, allow_voice FROM channels WHERE space_uuid = ?`
	channelRows, err := db.ChatDB.Query(channelsQuery, space.UUID)
	if err == nil {
		defer channelRows.Close()
		var channels []DashDataChannel
		for channelRows.Next() {
			var channel DashDataChannel
			if err := channelRows.Scan(&channel.ID, &channel.UUID, &channel.Name, &channel.SpaceUUID, &channel.AllowVoice); err == nil {
				channels = append(channels, channel)
			}
		}
		space.Channels = channels
	}

	// Fetch user IDs in space
	usersQuery := `SELECT user_id FROM space_users WHERE space_uuid = ? AND joined = 1`
	userRows, err := db.ChatDB.Query(usersQuery, space.UUID)
	if err != nil {
		log.Println("Error fetching space_users:", err)
		return
	}
	defer userRows.Close()

	userIDSet := make(map[int]struct{})
	for userRows.Next() {
		var uid int
		if err := userRows.Scan(&uid); err == nil {
			userIDSet[uid] = struct{}{}
		}
	}

	// Add author if not already included
	if _, exists := userIDSet[space.AuthorID]; !exists {
		userIDSet[space.AuthorID] = struct{}{}
	}

	// Build user ID slice
	var userIDs []int
	for id := range userIDSet {
		if id <= 0 {
			continue
		}
		userIDs = append(userIDs, id)
	}

	if len(userIDs) == 0 {
		space.Users = []DashDataUser{}
		return
	}

	users, err := lookupHostUsersByIDs(userIDs)
	if err != nil {
		log.Println("Error fetching users from host DB:", err)
		return
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
