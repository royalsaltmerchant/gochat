package spaces

import (
	ch "gochat/chatroom"
	"gochat/db"
	"gochat/types"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func HandleInsertChannel(c *gin.Context) {
	// TODO: Needs auth to ensure user can create new channel on this space

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

	var channel types.Channel

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

	// Broadcast
	ch.BroadcastToSpace(json.SpaceUUID, ch.WSMessage{
		Type: "new-channel",
		Data: ch.NewChannelPayload{
			ID:        channel.ID,
			UUID:      channel.UUID,
			Name:      channel.Name,
			SpaceUUID: channel.SpaceUUID,
		},
	})

	c.JSON(201, gin.H{
		"Channel": channel,
	})
}

func HandleDeleteChannel(c *gin.Context) {
	uuid := c.Param("uuid")
	isAuthor, _ := c.Get("isSpaceAuthor")

	if !isAuthor.(bool) {
		c.JSON(403, gin.H{"error": "You don't have permission to delete this channel"})
		return
	}

	// Get channel info before deleting
	var channelID int
	var spaceUUID string
	err := db.DB.QueryRow(`SELECT id, space_uuid FROM channels WHERE uuid = ?`, uuid).Scan(&channelID, &spaceUUID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Channel not found"})
		return
	}

	// Delete the channel
	res, err := db.DB.Exec(`DELETE FROM channels WHERE uuid = ?`, uuid)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete channel"})
		return
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		c.JSON(404, gin.H{"error": "Channel not found"})
		return
	}

	c.JSON(200, gin.H{"message": "Channel successfully deleted"})

	// Broadcast channel deletion
	ch.BroadcastToSpace(spaceUUID, ch.WSMessage{
		Type: "delete-channel",
		Data: ch.NewChannelPayload{
			ID:        channelID,
			SpaceUUID: spaceUUID,
		},
	})
}
