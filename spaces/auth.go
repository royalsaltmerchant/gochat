package spaces

import (
	"database/sql"
	"gochat/db"

	"github.com/gin-gonic/gin"
)

// SpaceAuthMiddleware checks if the user has access to a space (either as author or member)
func SpaceAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := c.Param("uuid")
		userID, _ := c.Get("userID")

		// Get space data
		var space Space
		query := `SELECT * FROM spaces WHERE uuid = ?`
		err := db.DB.QueryRow(query, uuid).Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(404, gin.H{"error": "Space not found"})
			} else {
				c.JSON(500, gin.H{"error": "Database error finding space"})
			}
			c.Abort()
			return
		}

		// If user is the author, they have access
		if space.AuthorID == userID {
			c.Set("space", space)
			c.Set("isAuthor", true)
			c.Next()
			return
		}

		// Check if user is a member of the space
		var spaceUser SpaceUser
		query = `SELECT * FROM space_users WHERE space_uuid = ? AND user_id = ? AND joined = 1`
		err = db.DB.QueryRow(query, uuid, userID).Scan(&spaceUser.ID, &spaceUser.SpaceUUID, &spaceUser.UserID, &spaceUser.Joined)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(403, gin.H{"error": "Not authorized to access this space"})
			} else {
				c.JSON(500, gin.H{"error": "Database error checking space membership"})
			}
			c.Abort()
			return
		}

		c.Set("space", space)
		c.Set("isAuthor", false)
		c.Next()
	}
}

// ChannelAuthMiddleware checks if the user has access to a channel (by checking parent space access)
func ChannelAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := c.Param("uuid")
		userID, _ := c.Get("userID")

		// Get channel data
		var channel Channel
		query := `SELECT * FROM channels WHERE uuid = ?`
		err := db.DB.QueryRow(query, uuid).Scan(&channel.ID, &channel.UUID, &channel.Name, &channel.SpaceUUID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(404, gin.H{"error": "Channel not found"})
			} else {
				c.JSON(500, gin.H{"error": "Database error finding channel"})
			}
			c.Abort()
			return
		}

		// Get space data
		var space Space
		query = `SELECT * FROM spaces WHERE uuid = ?`
		err = db.DB.QueryRow(query, channel.SpaceUUID).Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID)
		if err != nil {
			c.JSON(500, gin.H{"error": "Database error finding parent space"})
			c.Abort()
			return
		}

		// If user is the author, they have access
		if space.AuthorID == userID {
			c.Set("channel", channel)
			c.Set("space", space)
			c.Set("isAuthor", true)
			c.Next()
			return
		}

		// Check if user is a member of the space
		var spaceUser SpaceUser
		query = `SELECT * FROM space_users WHERE space_uuid = ? AND user_id = ? AND joined = 1`
		err = db.DB.QueryRow(query, channel.SpaceUUID, userID).Scan(&spaceUser.ID, &spaceUser.SpaceUUID, &spaceUser.UserID, &spaceUser.Joined)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(403, gin.H{"error": "Not authorized to access this channel"})
			} else {
				c.JSON(500, gin.H{"error": "Database error checking space membership"})
			}
			c.Abort()
			return
		}

		c.Set("channel", channel)
		c.Set("space", space)
		c.Set("isAuthor", false)
		c.Next()
	}
}
