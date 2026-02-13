package main

import (
	"context"
	"embed"
	"gochat/db"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	ratelimit "github.com/JGLTechnologies/gin-rate-limit"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

//go:embed relay-migrations/*.sql
var MigrationFiles embed.FS

func keyFunc(c *gin.Context) string {
	return c.ClientIP()
}

func rateLimiterrorHandler(c *gin.Context, info ratelimit.Info) {
	c.String(429, "Too many requests. Try again in "+time.Until(info.ResetTime).String())
}

func resolveStaticDir() string {
	if staticDir := os.Getenv("STATIC_DIR"); staticDir != "" {
		return staticDir
	}

	// Common local/dev path when running from relay_server/
	if _, err := os.Stat("./static"); err == nil {
		return "./static"
	}

	// Common local/dev path when running from repo root
	if _, err := os.Stat("./relay_server/static"); err == nil {
		return "./relay_server/static"
	}

	// Fallback to source-relative directory when available
	if _, currentFile, _, ok := runtime.Caller(0); ok {
		return filepath.Join(filepath.Dir(currentFile), "static")
	}

	// Deployment fallback used previously
	return "/root/relay_server/static"
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
	staticDir := resolveStaticDir()
	log.Printf("Using static directory: %s", staticDir)

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
	// Email preference/unsubscribe
	r.GET("/unsubscribe", HandleUnsubscribe)
	// Static
	r.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "index.html"))
	})
	r.GET("/forgot_password", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "forgot_password.html"))
	})
	r.GET("/reset_password", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "reset_password.html"))
	})
	r.Static("/static", staticDir)
	// SEO files
	r.GET("/robots.txt", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "robots.txt"))
	})
	r.GET("/sitemap.xml", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "sitemap.xml"))
	})
	// Call landing page
	r.GET("/call", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "call_landing.html"))
	})
	// React call room app
	r.GET("/call/room", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "call", "index.html"))
	})
	r.Static("/call/assets", filepath.Join(staticDir, "call", "assets"))
	// Web chat client app
	r.GET("/client", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "client", "index.html"))
	})
	r.GET("/client/", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "client", "index.html"))
	})
	r.Static("/client/assets", filepath.Join(staticDir, "client", "assets"))

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

	notificationCtx, cancelNotifications := context.WithCancel(context.Background())
	defer cancelNotifications()
	go StartEmailNotificationScheduler(notificationCtx)

	// Wait for SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
	cancelNotifications()

	// Gracefully shut down HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly.")
}
