package spaces

import (
	"database/sql"
	"fmt"
	"gochat/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Space struct {
	ID       int
	UUID     string
	Name     string
	AuthorID int
}

type SpaceUser struct {
	ID        int
	SpaceUUID string
	UserID    int
	Joined    int
	Name      string
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

func HandleInsertSpace(c *gin.Context) {
	var json struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get UUID and Author ID
	spaceUUID := uuid.New()

	authorIDRaw, exists := c.Get("userID")
	if !exists {
		c.JSON(500, gin.H{"error": "Failed to retrieve author ID"})
	}

	authorID, ok := authorIDRaw.(int)
	if !ok {
		c.JSON(500, gin.H{"error": "Failed to convert author ID to int"})
	}

	var space Space

	query := `INSERT INTO spaces (uuid, name, author_id) VALUES (?, ?, ?) RETURNING *`
	err := db.DB.QueryRow(query, spaceUUID, json.Name, authorID).Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID)

	if err != nil {
		// Check if the error message contains "UNIQUE constraint failed"
		if err.Error() == "UNIQUE constraint failed: spaces.uuid" {
			c.JSON(500, gin.H{"error": "uuid is already taken"})
			return
		}

		// For other database errors
		c.JSON(500, gin.H{"error": "Database error inserting space data"})
		return
	}

	channelUUID := uuid.New()
	initalChannelName := "Initial Channel"

	var channel Channel

	query = `INSERT INTO channels (uuid, name, space_uuid) VALUES (?, ?, ?) RETURNING *`
	err = db.DB.QueryRow(query, channelUUID, initalChannelName, space.UUID).Scan(&channel.ID, &channel.UUID, &channel.Name, &channel.SpaceUUID)

	if err != nil {
		// Check if the error message contains "UNIQUE constraint failed"
		if err.Error() == "UNIQUE constraint failed: channels.uuid" {
			c.JSON(500, gin.H{"error": "uuid is already taken"})
			return
		}

		// For other database errors
		c.JSON(500, gin.H{"error": "Database error inserting channel data"})
		return
	}

	c.JSON(201, gin.H{
		"message": "Successfully created new space with initial channel",
		"space": gin.H{
			"ID":       space.ID,
			"UUID":     space.UUID,
			"Name":     space.Name,
			"AuthorID": space.AuthorID,
		},
		"channel": gin.H{
			"ID":        channel.ID,
			"UUID":      channel.UUID,
			"Name":      channel.Name,
			"SpaceUUID": channel.SpaceUUID,
		},
	})
}

func HandleInsertSpaceUser(c *gin.Context) {
	var json struct {
		UserEmail string `json:"userEmail" binding:"required"`
		SpaceUUID string `json:"spaceUUID" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var userID int
	err := db.DB.QueryRow(`SELECT id FROM users WHERE email = ?`, json.UserEmail).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(400, gin.H{"error": "User not found"})
		} else {
			c.JSON(500, gin.H{"error": "Database error finding user"})
		}
		return
	}

	var spaceUser SpaceUser

	query := `INSERT INTO space_users (space_uuid, user_id) VALUES (?, ?) RETURNING *`
	err = db.DB.QueryRow(query, json.SpaceUUID, userID).Scan(&spaceUser.ID, &spaceUser.SpaceUUID, &spaceUser.UserID, &spaceUser.Joined)

	if err != nil {
		c.JSON(500, gin.H{"error": "Database error inserting channel data"})
		return
	}

	c.JSON(201, gin.H{
		"message": "Successfully created new space user",
	})
}

func HandleAcceptInvite(c *gin.Context) {
	var json struct {
		SpaceUserID string `json:"spaceUserID" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get space user
	var spaceUUID string
	query := `UPDATE space_users SET joined = 1 WHERE id = ? RETURNING space_uuid`
	err := db.DB.QueryRow(query, json.SpaceUserID).Scan(&spaceUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"error": "space user not found by id"})
		} else {
			c.JSON(500, gin.H{"error": "Database error finding space user"})
		}
		return
	}
	// Get and return space
	var space Space
	err = db.DB.QueryRow(`SELECT * FROM spaces WHERE uuid = ?`, spaceUUID).Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"error": "Space not found by uuid"})
		} else {
			c.JSON(500, gin.H{"error": "Database error finding space"})
		}
		return
	}

	c.JSON(200, gin.H{
		"space": space,
	})
}

func HandleDeclineInvite(c *gin.Context) {
	var json struct {
		SpaceUserID string `json:"spaceUserID" binding:"required"`
	}
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	res, err := db.DB.Exec(`DELETE FROM space_users WHERE id = ?`, json.SpaceUserID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete invite"})
		return
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		c.JSON(400, gin.H{"error": "Invite not found"})
		return
	}

	c.JSON(200, gin.H{"message": "Invite declined"})
}

func HandleInsertChannel(c *gin.Context) {
	var json struct {
		Name      string `json:"name" binding:"required"`
		SpaceUUID string `json:"spaceUUID" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get UUID and Author ID
	channelUUID := uuid.New()

	var channel Channel

	query := `INSERT INTO channels (uuid, name, space_uuid) VALUES (?, ?, ?) RETURNING *`
	err := db.DB.QueryRow(query, channelUUID, json.Name, json.SpaceUUID).Scan(&channel.ID, &channel.UUID, &channel.Name, &channel.SpaceUUID)

	if err != nil {
		// Check if the error message contains "UNIQUE constraint failed"
		if err.Error() == "UNIQUE constraint failed: channels.uuid" {
			c.JSON(500, gin.H{"error": "uuid is already taken"})
			return
		}

		// For other database errors
		c.JSON(500, gin.H{"error": "Database error inserting channel data"})
		return
	}

	c.JSON(201, gin.H{
		"message": "Successfully created new channel",
		"channel": gin.H{
			"ID":        channel.ID,
			"UUID":      channel.UUID,
			"Name":      channel.Name,
			"SpaceUUID": channel.SpaceUUID,
		},
	})
}

func InsertMessage(channelUUID string, content string, username string, userID sql.NullInt64, timestamp string) {
	query := `INSERT INTO messages (channel_uuid, content, username, user_id, timestamp) VALUES (?, ?, ?, ?, ?)`
	_, err := db.DB.Exec(query, channelUUID, content, username, userID, timestamp)
	if err != nil {
		fmt.Println("Error: Database failed to insert message")
	}
}

func HandleGetMessages(c *gin.Context) {
	var json struct {
		ChannelUUID string `json:"channelUUID" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get channel messages limit by 100 for now
	rows, err := db.DB.Query(`SELECT * FROM messages WHERE channel_uuid = ? LIMIT 100`, json.ChannelUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(400, gin.H{"error": "Channel not found by UUID"})
		} else {
			c.JSON(500, gin.H{"error": "Database Error extracting messages data"})
		}
		return
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var message Message
		err := rows.Scan(&message.ID, &message.ChannelUUID, &message.Content, &message.Username, &message.UserID, &message.Timestamp)
		if err != nil {
			fmt.Println("Error scanning message:", err)
			continue
		}
		messages = append(messages, message)
	}

	c.JSON(200, gin.H{
		"messages": messages,
	})
}
