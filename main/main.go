package main

import (
	"database/sql"
	"fmt"
	"gochat/auth"
	cr "gochat/chatroom"
	"gochat/db"
	"gochat/spaces"
	"log"
	"os"
	"time"

	ratelimit "github.com/JGLTechnologies/gin-rate-limit"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

func keyFunc(c *gin.Context) string {
	return c.ClientIP()
}

func rateLimiterrorHandler(c *gin.Context, info ratelimit.Info) {
	c.String(429, "Too many requests. Try again in "+time.Until(info.ResetTime).String())
}

// Initialize the HTTP server
func main() {
	// Load .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Env Vars
	port := os.Getenv("PORT")
	dbName := os.Getenv("DB_FILE")

	// Init DB
	if err = db.InitDB(dbName); err != nil {
		log.Fatal("Error opening database:", err)
		return
	}

	// Setup Gin
	r := gin.Default()

	// Rate Limit
	store := ratelimit.InMemoryStore(&ratelimit.InMemoryOptions{
		Rate:  time.Second,
		Limit: 100, // This makes it so each ip can only make 100 requests per second
	})

	mw := ratelimit.RateLimiter(store, &ratelimit.Options{
		ErrorHandler: rateLimiterrorHandler,
		KeyFunc:      keyFunc,
	})

	r.Use(mw)

	// Serve static files using Gin's static route
	r.Static("/static", "./static")
	r.Static("/js", "./js")
	// Load Template files
	r.LoadHTMLGlob("templates/*")

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

	r.GET("/channel/:uuid", func(c *gin.Context) {
		uuid := c.Param("uuid")
		username, _ := c.Get("userUsername")

		// TODO: Ensure user is authorized to view this channel

		// Get channel data
		var channel spaces.Channel
		query := `SELECT * FROM channels WHERE uuid = ?`
		err := db.DB.QueryRow(query, uuid).Scan(&channel.ID, &channel.UUID, &channel.Name, &channel.SpaceUUID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(400, gin.H{"error": "Channel not found by UUID"})
			} else {
				c.JSON(500, gin.H{"error": "Database Error extracting channel data"})
			}
			return
		}

		cr.ChatRooms[uuid] = &cr.ChatRoom{Users: make(map[*websocket.Conn]string)}

		c.HTML(200, "channel.html", gin.H{
			"Username": username,
			"Title":    channel.Name,
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

	r.GET("/space/:uuid", auth.AuthMiddleware(), func(c *gin.Context) {
		uuid := c.Param("uuid")
		userID, _ := c.Get("userID")
		username, _ := c.Get("userUsername")

		// First get author ID
		var space spaces.Space

		query := `SELECT * FROM spaces WHERE uuid = ?`
		err := db.DB.QueryRow(query, uuid).Scan(&space.ID, &space.UUID, &space.Name, &space.AuthorID)

		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to query space by UUID"})
			return
		}

		if space.AuthorID != userID {
			var spaceUser spaces.SpaceUser
			query := `SELECT * FROM space_users WHERE user_id = ? AND joined = 1`
			err = db.DB.QueryRow(query, userID).Scan(&spaceUser.ID, &spaceUser.SpaceUUID, &spaceUser.UserID, &spaceUser.Joined)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(401, gin.H{"error": "User not authorized to visit this page"})
				} else {
					c.JSON(500, gin.H{"error": "Database Error extracting user data"})
				}
				return
			}
		}
		// Then get space users user ID

		rows, err := db.DB.Query(`SELECT * FROM channels WHERE space_uuid = ? LIMIT 10`, uuid)
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
			"Title":    space.Name,
			"channels": channels,
		})
	})

	// API
	r.POST("/api/register", auth.HandleRegister)
	r.POST("/api/login", auth.HandleLogin)
	r.POST("/api/new_space", auth.AuthMiddleware(), spaces.HandleInsertSpace)
	r.POST("/api/new_space_user", auth.AuthMiddleware(), spaces.HandleInsertSpaceUser)
	r.POST("/api/accept_invite", auth.AuthMiddleware(), spaces.HandleAcceptInvite)
	r.POST("/api/decline_invite", auth.AuthMiddleware(), spaces.HandleDeclineInvite)
	r.POST("/api/new_channel", auth.AuthMiddleware(), spaces.HandleInsertChannel)
	r.POST("/api/get_messages", auth.AuthMiddleware(), spaces.HandleGetMessages)

	// Join chat room endpoint
	r.GET("/ws/:uuid", auth.AuthMiddleware(), cr.JoinChatRoom)

	// Start the server
	go func() {
		if err := r.Run(port); err != nil {
			log.Fatal(err)
		}
	}()

	// Gracefully close the database on shutdown
	defer db.CloseDB()

	// Keep the server running (block forever)
	select {}
}
