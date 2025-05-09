package routes

import (
	"gochat/db"
	"gochat/spaces"
	auth "gochat/users"

	"github.com/gin-gonic/gin"
)

func SetupAPIRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		// User
		api.POST("/register", auth.HandleRegister)
		api.POST("/login", auth.ValidateCSRFMiddleware(), auth.HandleLogin)
		api.POST("/logout", auth.HandleLogout)
		api.PUT("/update_username", auth.AuthMiddleware(), auth.HandleUpdateUsername)
		// Space
		api.POST("/new_space", auth.AuthMiddleware(), spaces.HandleInsertSpace)
		api.DELETE("/space/:uuid", auth.AuthMiddleware(), spaces.SpaceAuthMiddleware(), spaces.HandleDeleteSpace)
		// Channel
		api.POST("/new_channel", auth.AuthMiddleware(), spaces.HandleInsertChannel)
		api.DELETE("/channel/:uuid", auth.AuthMiddleware(), spaces.ChannelAuthMiddleware(), spaces.HandleDeleteChannel)
		// Space User
		api.POST("/new_space_user/:uuid", auth.AuthMiddleware(), spaces.SpaceAuthMiddleware(), spaces.HandleInsertSpaceUser)
		api.DELETE("/space_user/:uuid", auth.AuthMiddleware(), spaces.SpaceAuthMiddleware(), spaces.HandleDeleteSpaceUser)
		api.POST("/accept_invite", auth.AuthMiddleware(), spaces.HandleAcceptInvite)
		api.POST("/decline_invite", auth.AuthMiddleware(), spaces.HandleDeclineInvite)
		// Message
		api.GET("/get_messages/:uuid", auth.AuthMiddleware(), spaces.ChannelAuthMiddleware(), spaces.HandleGetMessages)
		// Invites
		api.GET("/get_invites", auth.AuthMiddleware(), func(c *gin.Context) {
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

			var invites []spaces.SpaceUser
			for rows.Next() {
				var invite spaces.SpaceUser
				err := rows.Scan(&invite.ID, &invite.SpaceUUID, &invite.UserID, &invite.Joined, &invite.Name)
				if err != nil {
					continue
				}
				invites = append(invites, invite)
			}

			c.JSON(200, gin.H{
				"invites": invites,
			})
		})

		// Full Dashboard
		api.GET("/dashboard_data", auth.AuthMiddleware(), func(c *gin.Context) {
			userID, _ := c.Get("userID")
			username, _ := c.Get("userUsername")

			// 1. Collect user spaces (as author OR accepted invite)
			query := `
				SELECT DISTINCT s.id, s.uuid, s.name, s.author_id
				FROM spaces s
				LEFT JOIN space_users su ON su.space_uuid = s.uuid
				WHERE s.author_id = ?
					 OR (su.user_id = ? AND su.joined = 1)
			`

			rows, err := db.DB.Query(query, userID, userID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Database error fetching user spaces"})
				return
			}
			defer rows.Close()

			var userSpaces []spaces.Space
			for rows.Next() {
				var space spaces.Space
				err := rows.Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID)
				if err != nil {
					continue
				}
				userSpaces = append(userSpaces, space)
			}

			// Get channels and space users for each space
			for i := range userSpaces {
				spaces.AppendspaceChannelsAndUsers(&userSpaces[i])
			}

			c.JSON(200, gin.H{
				"user":   gin.H{"ID": userID, "Username": username},
				"spaces": userSpaces,
			})
		})
	}
}
