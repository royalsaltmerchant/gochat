package main

import (
	"gochat/auth"
	cr "gochat/chatroom"
	"gochat/db"
	"log"
	"os"
	"time"

	ratelimit "github.com/JGLTechnologies/gin-rate-limit"
	"github.com/gin-gonic/gin"
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
		Limit: 5, // This makes it so each ip can only make 5 requests per second
	})

	mw := ratelimit.RateLimiter(store, &ratelimit.Options{
		ErrorHandler: rateLimiterrorHandler,
		KeyFunc:      keyFunc,
	})

	r.Use(mw)

	// Serve static files using Gin's static route
	r.Static("/static", "./static")
	// Load Template files
	r.LoadHTMLGlob("templates/*")

	r.GET("/register", func(c *gin.Context) {
		c.HTML(200, "register.html", gin.H{
			"Title": "Register",
		})
	})

	// API
	r.POST("/api/register", auth.HandleRegister)
	r.POST("/api/login", auth.HandleLogin)
	r.GET("/api/rooms", auth.JwtMiddleware(), cr.ListChatRooms)

	// Join chat room endpoint
	r.GET("/ws", cr.JoinChatRoom)

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
