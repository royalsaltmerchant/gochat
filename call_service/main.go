package main

import (
	"context"
	"embed"
	"gochat/call_service/internal/gm"
	"gochat/db"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
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
	if staticDir := os.Getenv("CALL_STATIC_DIR"); staticDir != "" {
		return staticDir
	}

	// Prefer static dir adjacent to deployed binary.
	if exePath, err := os.Executable(); err == nil {
		exeStatic := filepath.Join(filepath.Dir(exePath), "static")
		if _, err := os.Stat(exeStatic); err == nil {
			return exeStatic
		}
	}

	candidates := []string{
		"./static",
		"./call_service/static",
		"./call_service_dist/static",
		"/root/call_service_dist/static",
		"/root/call_service/static",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Local source fallback.
	if _, currentFile, _, ok := runtime.Caller(0); ok {
		return filepath.Join(filepath.Dir(currentFile), "static")
	}

	// Last-resort deploy fallback.
	return "/root/call_service_dist/static"
}

func main() {
	_ = godotenv.Load()

	port := os.Getenv("CALL_SERVICE_PORT")
	if port == "" {
		port = "8000"
	}
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	dbName := strings.TrimSpace(os.Getenv("HOST_DB_FILE"))
	if dbName == "" {
		dbName = "./relay.db"
	}
	dbPathForLog := dbName
	if !filepath.IsAbs(dbPathForLog) {
		if abs, err := filepath.Abs(dbPathForLog); err == nil {
			dbPathForLog = abs
		}
	}
	allowedOrigins := parseAllowedOriginsFromEnv(os.Getenv("ALLOWED_ORIGINS"))
	setAllowedWebSocketOrigins(allowedOrigins)
	staticDir := resolveStaticDir()
	log.Printf("Using static directory: %s", staticDir)
	log.Printf("Using host database: %s", dbPathForLog)

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
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// WebSocket route for call room signaling
	r.GET("/ws", func(c *gin.Context) {
		HandleSocket(c)
	})

	// Call app auth endpoints
	r.POST("/call/register", HandleCallRegister)
	r.POST("/call/login", HandleCallLogin)
	r.POST("/call/login-by-token", HandleCallLoginByToken)
	r.POST("/call/request-reset-email", HandleCallPasswordResetRequest)
	r.POST("/call/reset-password", HandleCallPasswordReset)
	r.GET("/call/api/account", HandleCallAccount)
	// Call app Stripe endpoints
	r.POST("/call/create-checkout-session", HandleCreateCheckoutSession)
	r.POST("/call/stripe-webhook", HandleStripeWebhook)
	r.POST("/call/create-portal-session", HandleCreatePortalSession)
	// GM marketplace endpoints
	gm.RegisterRoutes(r, db.HostDB)
	// Internal SFU auth endpoints (used by Caddy forward_auth)
	r.GET("/internal/validate-sfu-token", HandleValidateSFUToken)
	r.GET("/internal/validate-ip", HandleValidateIP)

	// Call landing page
	r.GET("/call", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "call_landing.html"))
	})
	r.GET("/call/", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "call_landing.html"))
	})
	// Call account page
	r.GET("/call/account", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "call_account.html"))
	})
	// Call pricing page
	r.GET("/call/pricing", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "call_pricing.html"))
	})
	// Call reset password page
	r.GET("/call/reset-password", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "call_reset_password.html"))
	})
	r.GET("/call/reset-password/", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "call_reset_password.html"))
	})
	// React call room app
	r.GET("/call/room", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "call", "index.html"))
	})
	r.Static("/call/assets", filepath.Join(staticDir, "call", "assets"))
	r.Static("/call/static", staticDir)

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
