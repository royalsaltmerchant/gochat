package routes

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"gochat/auth"
	cr "gochat/chatroom"
	"gochat/db"
	"gochat/spaces"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func GenerateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

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
		// Set CSRF token as cookie
		csrfToken := GenerateCSRFToken()
		isSecure := os.Getenv("ENV") == "production"
		host := strings.Split(c.Request.Host, ":")[0]
		c.SetCookie(
			"csrf_token",
			csrfToken,
			3600,
			"/",
			host,
			isSecure,
			false, // Must be accessible via JS
		)

		c.HTML(200, "login.html", gin.H{
			"Title":      "Login",
			"csrf_token": csrfToken,
		})
	})

	r.GET("/channel/:uuid/content", auth.AuthMiddleware(), spaces.ChannelAuthMiddleware(), func(c *gin.Context) {
		username, _ := c.Get("userUsername")
		channel, _ := c.Get("channel")

		cr.ChatRooms[channel.(spaces.Channel).UUID] = &cr.ChatRoom{Users: make(map[*websocket.Conn]string)}

		// Return just the chat app div
		c.HTML(200, "channel_content.html", gin.H{
			"Title":    channel.(spaces.Channel).Name,
			"Username": username,
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
		username, _ := c.Get("userUsername")
		c.HTML(200, "dashboard.html", gin.H{
			"Title":    "Dashboard",
			"Username": username,
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
