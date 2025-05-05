package routes

import (
	"fmt"
	"gochat/auth"
	cr "gochat/chatroom"
	"gochat/db"
	"gochat/spaces"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func SetupRegularRoutes(r *gin.Engine) {
	// Serve static files using Gin's static route
	r.Static("/static", "./static")
	r.Static("/js", "./js")
	// Load Template files
	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{
			"Title": "Index",
		})
	})

	r.GET("/account", auth.AuthMiddleware(), func(c *gin.Context) {
		username, _ := c.Get("userUsername")

		c.HTML(200, "account.html", gin.H{
			"Title":    "Account",
			"Username": username,
		})
	})

	r.GET("/register", func(c *gin.Context) {
		c.HTML(200, "register.html", gin.H{
			"Title": "Register",
		})
	})

	r.GET("/login", func(c *gin.Context) {
		c.HTML(200, "login.html", gin.H{
			"Title": "Login",
		})
	})

	r.GET("/channel/:uuid", auth.AuthMiddleware(), spaces.ChannelAuthMiddleware(), func(c *gin.Context) {
		username, _ := c.Get("userUsername")
		channel, _ := c.Get("channel")

		cr.ChatRooms[channel.(spaces.Channel).UUID] = &cr.ChatRoom{Users: make(map[*websocket.Conn]string)}

		c.HTML(200, "channel.html", gin.H{
			"Title":    channel.(spaces.Channel).Name,
			"Username": username,
		})
	})

	r.GET("/dashboard", auth.AuthMiddleware(), func(c *gin.Context) {
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
			fmt.Println("Error querying user spaces:", err)
			c.JSON(500, gin.H{"error": "Database error fetching user spaces"})
			return
		}
		defer rows.Close()

		var userSpaces []spaces.Space
		for rows.Next() {
			var space spaces.Space
			err := rows.Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID)
			if err != nil {
				fmt.Println("Error scanning space:", err)
				continue
			}
			userSpaces = append(userSpaces, space)
		}

		// 2. Collect invites (space_users.joined = 0) + space.name
		query = `
			SELECT su.id, su.space_uuid, su.user_id, su.joined, s.name
			FROM space_users su
			JOIN spaces s ON su.space_uuid = s.uuid
			WHERE su.user_id = ? AND su.joined = 0
		`

		rows, err = db.DB.Query(query, userID)
		if err != nil {
			fmt.Println("Error querying invites:", err)
			c.JSON(500, gin.H{"error": "Database error fetching invites"})
			return
		}
		defer rows.Close()

		var invites []spaces.SpaceUser
		for rows.Next() {
			var invite spaces.SpaceUser
			err := rows.Scan(&invite.ID, &invite.SpaceUUID, &invite.UserID, &invite.Joined, &invite.Name)
			if err != nil {
				fmt.Println("Error scanning invite:", err)
				continue
			}
			invites = append(invites, invite)
		}

		// 3. Render dashboard
		c.HTML(200, "dashboard.html", gin.H{
			"Title":      "Dashboard",
			"Username":   username,
			"userSpaces": userSpaces,
			"invites":    invites,
		})
	})

	r.GET("/space/:uuid", auth.AuthMiddleware(), spaces.SpaceAuthMiddleware(), func(c *gin.Context) {
		username, _ := c.Get("userUsername")
		space, _ := c.Get("space")
		isAuthor, _ := c.Get("isAuthor")

		// Get channels
		rows, err := db.DB.Query(`SELECT * FROM channels WHERE space_uuid = ? LIMIT 10`, space.(spaces.Space).UUID)
		if err != nil {
			fmt.Println("Error querying channels:", err)
			return
		}
		defer rows.Close()

		var channels []spaces.Channel
		for rows.Next() {
			var channel spaces.Channel
			err := rows.Scan(&channel.ID, &channel.UUID, &channel.Name, &channel.SpaceUUID)
			if err != nil {
				fmt.Println("Error scanning channel:", err)
				continue
			}
			channels = append(channels, channel)
		}

		c.HTML(200, "space.html", gin.H{
			"Username": username,
			"Title":    space.(spaces.Space).Name,
			"channels": channels,
			"IsAuthor": isAuthor,
		})
	})

	// Join chat room endpoint
	r.GET("/ws/:uuid", auth.AuthMiddleware(), spaces.ChannelAuthMiddleware(), cr.JoinChatRoom)
}
