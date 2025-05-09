package spaces

import (
	"database/sql"
	"fmt"
	"gochat/db"

	"github.com/gin-gonic/gin"
)

type Message struct {
	ID          int
	ChannelUUID string
	Content     string
	Username    string
	UserID      int
	Timestamp   string
}

func InsertMessage(channelUUID string, content string, username string, userID sql.NullInt64, timestamp string) {
	query := `INSERT INTO messages (channel_uuid, content, username, user_id, timestamp) VALUES (?, ?, ?, ?, ?)`
	_, err := db.DB.Exec(query, channelUUID, content, username, userID, timestamp)
	if err != nil {
		fmt.Println("Error: Database failed to insert message")
	}
}

func HandleGetMessages(c *gin.Context) {
	uuid := c.Param("uuid")

	// Get channel messages limit by 100 for now maybe paginate?
	rows, err := db.DB.Query(`SELECT * FROM messages WHERE channel_uuid = ? LIMIT 100`, uuid)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error extracting messages"})
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
		"Messages": messages,
	})
}
