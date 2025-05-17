package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gochat/db"

	"net/http"

	ratelimit "github.com/JGLTechnologies/gin-rate-limit"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pion/webrtc/v3"
)

func keyFunc(c *gin.Context) string {
	return c.ClientIP()
}

func rateLimiterrorHandler(c *gin.Context, info ratelimit.Info) {
	c.String(429, "Too many requests. Try again in "+time.Until(info.ResetTime).String())
}

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("RELAY_PORT")
	if port == "" {
		port = "8000"
	}
	dbName := os.Getenv("HOST_DB_FILE")

	// Init DB
	var err error
	db.HostDB, err = db.InitDB(dbName)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.CloseDB(db.HostDB)

	// Setup Gin
	r := gin.Default()

	// Rate Limiting
	store := ratelimit.InMemoryStore(&ratelimit.InMemoryOptions{
		Rate:  time.Second,
		Limit: 100,
	})
	r.Use(ratelimit.RateLimiter(store, &ratelimit.Options{
		ErrorHandler: rateLimiterrorHandler,
		KeyFunc:      keyFunc,
	}))

	m := &webrtc.MediaEngine{}
	_ = m.RegisterDefaultCodecs()
	rtcapi := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	// WebSocket route
	r.GET("/ws", func(c *gin.Context) {
		HandleSocket(c, rtcapi)
	})
	r.GET("/api/host/:uuid", HandleGetHost)
	r.POST("/api/register_host", HandleRegisterHost)

	// Create HTTP server manually so we can shut it down
	server := &http.Server{
		Addr:    port,
		Handler: r,
	}

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Starting server on port %s...", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	// Wait for SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Gracefully shut down HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly.")
}
