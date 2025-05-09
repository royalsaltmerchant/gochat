package spaces

import (
	"database/sql"
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

	c.JSON(200, gin.H{"message": "Space user deleted successfully"})
}

func HandleAcceptInvite(c *gin.Context) {
	userID, _ := c.Get("userID")

	var json struct {
		SpaceUserID string `json:"spaceUserID" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get space user
	var spaceUUID string
	query := `UPDATE space_users SET joined = 1 WHERE id = ? AND user_id = ? RETURNING space_uuid` // Checking by user_id also ensures they are authorized
	err := db.DB.QueryRow(query, json.SpaceUserID, userID).Scan(&spaceUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"error": "space user not found by id"})
		} else {
			c.JSON(500, gin.H{"error": "Database error finding space user"})
		}
		return
	}

	c.JSON(200, gin.H{
		"message": "Accepted Invite",
	})
}

func HandleDeclineInvite(c *gin.Context) {
	userID, _ := c.Get("userID")

	var json struct {
		SpaceUserID string `json:"spaceUserID" binding:"required"`
	}
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	res, err := db.DB.Exec(`DELETE FROM space_users WHERE id = ? AND user_id = ?`, json.SpaceUserID, userID) // Checking by user_id also ensures they are authorized
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
