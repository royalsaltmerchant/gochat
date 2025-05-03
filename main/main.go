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

		var channel spaces.Channel
		query := `SELECT * FROM channels WHERE uuid = ?`
		err := db.DB.QueryRow(query, uuid).Scan(&channel.ID, &channel.UUID, &channel.Name, &channel.SpaceID)
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
			"Title": channel.Name,
		})
	})

	r.GET("/dashboard", auth.AuthMiddleware(), func(c *gin.Context) {
		authorID, _ := c.Get("userID") // From middleware

		rows, err := db.DB.Query(`SELECT * FROM spaces WHERE author_id = ? LIMIT 10`, authorID)
		if err != nil {
			fmt.Println("Error querying spaces:", err)
			c.Status(500)
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

		c.HTML(200, "dashboard.html", gin.H{
			"Title":      "Dashboard",
			"userSpaces": userSpaces,
		})
	})

	// API
	r.POST("/api/register", auth.HandleRegister)
	r.POST("/api/login", auth.HandleLogin)
	r.POST("/api/new_space", auth.AuthMiddleware(), spaces.HandleInsertSpace)

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
