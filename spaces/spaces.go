package spaces

import (
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

type Channel struct {
	ID        int
	UUID      string
	Name      string
	SpaceUUID string // space UUID
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
			"name":     space.Name,
			"authorID": space.AuthorID,
		},
		"channel": gin.H{
			"ID":       channel.ID,
			"UUID":     channel.UUID,
			"name":     channel.Name,
			"space_id": channel.SpaceUUID,
		},
	})
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
			"ID":         channel.ID,
			"UUID":       channel.UUID,
			"name":       channel.Name,
			"space_uuid": channel.SpaceUUID,
		},
	})
}
