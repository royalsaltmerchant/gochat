package main

import (
	"context"
	"embed"
	"gochat/db"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ratelimit "github.com/JGLTechnologies/gin-rate-limit"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/pion/webrtc/v3"
)

//go:embed relay-migrations/*.sql
var MigrationFiles embed.FS

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
	db.HostDB, err = db.InitDB(dbName, MigrationFiles, "relay-migrations")
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

	// CORS
	r.Use(cors.Default())

	m := &webrtc.MediaEngine{}
	_ = m.RegisterDefaultCodecs()

	// WebSocket route
	r.GET("/ws", func(c *gin.Context) {
		HandleSocket(c)
	})
	// API
	r.GET("/api/host/:uuid", HandleGetHost)
	r.POST("/api/hosts_by_uuids", HandleGetHostsByUUIDs)
	r.POST("/api/host_offline/:uuid", HandleUpdateHostOffline)
	r.POST("/api/register_host", HandleRegisterHost)
	r.POST("/api/user_by_id", HandleGetUserByID)
	r.POST("/api/user_by_email", HandleGetUserByEmail)
	r.POST("/api/users_by_ids", HandleGetUsersByIDs)
	r.GET("/api/turn_credentials", HandleGetTurnCredentials)
	r.POST("/api/request_reset_email", HandlePasswordResetRequest)
	r.POST("/api/reset_password", HandlePasswordReset)
	// Internal validation endpoints (for Caddy/proxy auth)
	r.GET("/internal/validate-ip", HandleValidateIP)
	r.GET("/internal/validate-sfu-token", HandleValidateSFUToken)
	// Static
	r.GET("/", func(c *gin.Context) {
		c.File("/root/relay_server/static/index.html")
	})
	r.GET("/forgot_password", func(c *gin.Context) {
		c.File("/root/relay_server/static/forgot_password.html")
	})
	r.GET("/reset_password", func(c *gin.Context) {
		c.File("/root/relay_server/static/reset_password.html")
	})
	r.Static("/static", "/root/relay_server/static")
	// SEO files
	r.GET("/robots.txt", func(c *gin.Context) {
		c.File("/root/relay_server/static/robots.txt")
	})
	r.GET("/sitemap.xml", func(c *gin.Context) {
		c.File("/root/relay_server/static/sitemap.xml")
	})
	// Call landing page
	r.GET("/call", func(c *gin.Context) {
		c.File("/root/relay_server/static/call_landing.html")
	})
	// React call room app
	r.GET("/call/room", func(c *gin.Context) {
		c.File("/root/relay_server/static/call/index.html")
	})
	r.Static("/call/assets", "/root/relay_server/static/call/assets")

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
