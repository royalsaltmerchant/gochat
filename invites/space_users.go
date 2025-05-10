package invites

import (
	"database/sql"
	ch "gochat/chatroom"
	"gochat/db"

	"github.com/gin-gonic/gin"
)

type SpaceUser struct {
	ID        int
	SpaceUUID string
	UserID    int
	Joined    int
	Name      string
}

func HandleInsertSpaceUser(c *gin.Context) { // Create invite
	spaceUUID := c.Param("uuid")

	var json struct {
		UserEmail string `json:"userEmail" binding:"required"`
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
			// FUTURE TODO: Send email invite?
		} else {
			c.JSON(500, gin.H{"error": "Database error finding user"})
		}
		return
	}

	var spaceUser SpaceUser

	query := `INSERT INTO space_users (space_uuid, user_id) VALUES (?, ?) RETURNING *`
	err = db.DB.QueryRow(query, spaceUUID, userID).Scan(&spaceUser.ID, &spaceUser.SpaceUUID, &spaceUser.UserID, &spaceUser.Joined)

	if err != nil {
		c.JSON(500, gin.H{"error": "Database error inserting space_user data"})
		return
	}

	// Send through socket to invited user

	c.JSON(201, gin.H{
		"message": "Successfully created new space user",
	})
}

func HandleDeleteSpaceUser(c *gin.Context) {
	var json struct {
		SpaceUUID string `json:"spaceUUID" binding:"required"`
		UserID    int    `json:"userID" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	res, err := db.DB.Exec("DELETE FROM space_users WHERE space_uuid = ? AND user_id = ?", json.SpaceUUID, json.UserID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error deleting space user"})
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		c.JSON(404, gin.H{"error": "Space user not found"})
		return
	}

	// Broadcast to connected users in space
	ch.BroadcastToSpace(json.SpaceUUID, ch.WSMessage{
		Type: "remove-user",
		Data: ch.NewUserPayload{
			ID:        json.UserID,
			SpaceUUID: json.SpaceUUID,
		},
	})

	c.JSON(200, gin.H{"message": "Space user deleted successfully"})
}

func HandleDeleteSpaceUserSelf(c *gin.Context) {
	userIDRaw, _ := c.Get("userID")
	userID, _ := userIDRaw.(int)

	var json struct {
		SpaceUUID string `json:"spaceUUID" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	res, err := db.DB.Exec("DELETE FROM space_users WHERE space_uuid = ? AND user_id = ?", json.SpaceUUID, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error deleting space user"})
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		c.JSON(404, gin.H{"error": "Space user not found"})
		return
	}

	// Broadcast to connected users in space
	ch.BroadcastToSpace(json.SpaceUUID, ch.WSMessage{
		Type: "remove-user",
		Data: ch.NewUserPayload{
			ID:        userID,
			SpaceUUID: json.SpaceUUID,
		},
	})

	c.JSON(200, gin.H{"message": "Space user deleted successfully"})
}
