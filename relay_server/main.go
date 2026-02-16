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

	// Prefer static dir adjacent to deployed binary.
	if exePath, err := os.Executable(); err == nil {
		exeStatic := filepath.Join(filepath.Dir(exePath), "static")
		if _, err := os.Stat(exeStatic); err == nil {
			return exeStatic
		}
	}

	candidates := []string{
		"./static",
		"./relay_server/static",
		"./relay_dist/static",
		"/root/relay_dist/static",
		"/root/relay_server/static",
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
	return "/root/relay_dist/static"
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

	// WebSocket route for call room signaling
	r.GET("/ws", func(c *gin.Context) {
		HandleSocket(c)
	})

	// Call app auth endpoints
	r.POST("/call/register", HandleCallRegister)
	r.POST("/call/login", HandleCallLogin)
	r.POST("/call/login-by-token", HandleCallLoginByToken)
	r.GET("/call/api/account", HandleCallAccount)
	// Call app Stripe endpoints
	r.POST("/call/create-checkout-session", HandleCreateCheckoutSession)
	r.POST("/call/stripe-webhook", HandleStripeWebhook)
	r.POST("/call/create-portal-session", HandleCreatePortalSession)
	// Internal SFU auth endpoints (used by Caddy forward_auth)
	r.GET("/internal/validate-sfu-token", HandleValidateSFUToken)
	r.GET("/internal/validate-ip", HandleValidateIP)

	// Static
	r.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "index.html"))
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
	// Redirect old how-it-works URL to landing page (content merged into index)
	r.GET("/chat/how-it-works", func(c *gin.Context) {
		c.Redirect(301, "/")
	})
	// React call room app
	r.GET("/call/room", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "call", "index.html"))
	})
	r.Static("/call/assets", filepath.Join(staticDir, "call", "assets"))

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
