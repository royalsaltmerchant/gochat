package spaces

import (
	"database/sql"
	"gochat/db"

	"github.com/gin-gonic/gin"
)

func SpaceAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := c.Param("uuid")
		userID, _ := c.Get("userID")

		var authorID int
		err := db.DB.QueryRow("SELECT author_id FROM spaces WHERE uuid = ?", uuid).Scan(&authorID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(404, gin.H{"error": "Space not found"})
				return
			}
			c.JSON(500, gin.H{"error": "Database error finding space"})
			return
		}

		if authorID == userID {
			c.Set("isSpaceAuthor", true)
		} else {
			c.Set("isSpaceAuthor", false)
			c.JSON(403, gin.H{"error": "Not authorized to manage this space"})
			return
		}

		c.Next()
	}
}

func ChannelAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		channelUUID := c.Param("uuid")
		userID, _ := c.Get("userID")

		query := `
			SELECT s.uuid AS space_uuid,
			       CASE WHEN s.author_id = ? THEN 1 ELSE 0 END AS is_author
			FROM channels c
			JOIN spaces s ON c.space_uuid = s.uuid
			LEFT JOIN space_users su ON su.space_uuid = s.uuid
			WHERE c.uuid = ? AND (s.author_id = ? OR (su.user_id = ? AND su.joined = 1))
			LIMIT 1
		`

		var spaceUUID string
		var isAuthor int
		err := db.DB.QueryRow(query, userID, channelUUID, userID, userID).Scan(&spaceUUID, &isAuthor)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(403, gin.H{"error": "You don't have permission to access this channel"})
			} else {
				c.JSON(500, gin.H{"error": "Database error checking channel access"})
			}
			c.Abort()
			return
		}

		c.Set("spaceUUID", spaceUUID)
		c.Set("isSpaceAuthor", isAuthor == 1)

		c.Next()
	}
}
