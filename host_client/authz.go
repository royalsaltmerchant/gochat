package main

import (
	"database/sql"
	"fmt"
	"gochat/db"
)

func ensureSpaceAuthor(spaceUUID string, requesterID int) error {
	if spaceUUID == "" {
		return fmt.Errorf("missing space uuid")
	}
	if requesterID <= 0 {
		return fmt.Errorf("invalid requester id")
	}

	var authorID int
	err := db.ChatDB.QueryRow(`SELECT author_id FROM spaces WHERE uuid = ?`, spaceUUID).Scan(&authorID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("space not found")
		}
		return fmt.Errorf("failed to load space author: %w", err)
	}
	if authorID != requesterID {
		return fmt.Errorf("forbidden")
	}
	return nil
}

func loadChannelAuthz(channelUUID string) (spaceUUID string, authorID int, err error) {
	if channelUUID == "" {
		return "", 0, fmt.Errorf("missing channel uuid")
	}
	err = db.ChatDB.QueryRow(
		`SELECT c.space_uuid, s.author_id
		   FROM channels c
		   JOIN spaces s ON s.uuid = c.space_uuid
		  WHERE c.uuid = ?`,
		channelUUID,
	).Scan(&spaceUUID, &authorID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", 0, fmt.Errorf("channel not found")
		}
		return "", 0, err
	}
	return spaceUUID, authorID, nil
}
