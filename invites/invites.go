package invites

import (
	"database/sql"
	ch "gochat/chatroom"
	"gochat/db"

	"github.com/gin-gonic/gin"
)

func HandleAcceptInvite(c *gin.Context) {
	usernameRaw, _ := c.Get("userUsername")
	username := usernameRaw.(string)

	userIDRaw, _ := c.Get("userID")
	userID, _ := userIDRaw.(int)

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

	// Broadcast to users joined on space
	ch.BroadcastToSpace(spaceUUID, ch.WSMessage{
		Type: "new-user",
		Data: ch.NewUserPayload{
			ID:        userID,
			Username:  username,
			SpaceUUID: spaceUUID,
		},
	})

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

func HandleGetInvites(c *gin.Context) {
	userID, _ := c.Get("userID")

	// Collect invites (space_users.joined = 0) + space.name
	query := `
				SELECT su.id, su.space_uuid, su.user_id, su.joined, s.name
				FROM space_users su
				JOIN spaces s ON su.space_uuid = s.uuid
				WHERE su.user_id = ? AND su.joined = 0
			`

	rows, err := db.DB.Query(query, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error fetching invites"})
		return
	}
	defer rows.Close()

	var spaceInvites []SpaceUser
	for rows.Next() {
		var spaceInvite SpaceUser
		err := rows.Scan(&spaceInvite.ID, &spaceInvite.SpaceUUID, &spaceInvite.UserID, &spaceInvite.Joined, &spaceInvite.Name)
		if err != nil {
			continue
		}
		spaceInvites = append(spaceInvites, spaceInvite)
	}

	c.JSON(200, gin.H{
		"invites": spaceInvites,
	})
}
