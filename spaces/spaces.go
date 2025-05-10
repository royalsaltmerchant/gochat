package spaces

import (
	"gochat/db"
	auth "gochat/users"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Space struct {
	ID       int
	UUID     string
	Name     string
	AuthorID int
	Channels []Channel
	Users    []auth.UserData
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

	space.Channels = append(space.Channels, channel)
	space.Users = append(space.Users, auth.UserData{})

	c.JSON(201, gin.H{
		"Space": space,
	})
}

func HandleDeleteSpace(c *gin.Context) {
	uuid := c.Param("uuid")

	// Delete the space (cascades to channels, messages, space_users)
	res, err := db.DB.Exec("DELETE FROM spaces WHERE uuid = ?", uuid)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error deleting space"})
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		c.JSON(404, gin.H{"error": "Space not found"})
		return
	}

	c.JSON(200, gin.H{"message": "Space deleted successfully"})
}

func HandleGetDashData(c *gin.Context) {
	userID, _ := c.Get("userID")
	username, _ := c.Get("userUsername")

	// 1. Use helper
	userSpaces, err := GetUserSpaces(userID.(int))
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error fetching user spaces"})
		return
	}

	// 2. Enrich with channels/users
	for i := range userSpaces {
		AppendspaceChannelsAndUsers(&userSpaces[i])
	}

	// 3. Respond
	c.JSON(200, gin.H{
		"user":   gin.H{"ID": userID, "Username": username},
		"spaces": userSpaces,
	})
}
