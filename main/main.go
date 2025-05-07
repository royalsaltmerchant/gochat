package main

import (
	"gochat/db"
	"gochat/main/routes"
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
		Limit: 100, // This makes it so each ip can only make 100 requests per second
	})

	mw := ratelimit.RateLimiter(store, &ratelimit.Options{
		ErrorHandler: rateLimiterrorHandler,
		KeyFunc:      keyFunc,
	})

	r.Use(mw)

	// Setup routes
	routes.SetupRegularRoutes(r)
	routes.SetupAPIRoutes(r)

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
