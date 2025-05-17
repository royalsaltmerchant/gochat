package main

import (
	"encoding/json"
	"gochat/db"
	"log"
)

func AppendspaceChannelsAndUsers(space *DashDataSpace) {
	// Fetch channels
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
		userIDs = append(userIDs, id)
	}

	// Batch request to /api/users_by_ids
	payload := map[string][]int{"user_ids": userIDs}
	resp, err := PostJSON(relayBaseURL.String()+"/api/users_by_ids", payload, nil)
	if err != nil {
		log.Println("Error fetching users:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 400 {
		log.Println("Failed to find users by IDs")
	}

	var users []DashDataUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		log.Println("Error decoding users_by_ids response:", err)
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
