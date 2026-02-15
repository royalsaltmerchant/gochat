package main

import (
	"context"
	"gochat/db"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	ratelimit "github.com/JGLTechnologies/gin-rate-limit"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func keyFunc(c *gin.Context) string {
	return c.ClientIP()
}

func rateLimiterrorHandler(c *gin.Context, info ratelimit.Info) {
	c.String(429, "Too many requests. Try again in "+time.Until(info.ResetTime).String())
}

func main() {
	_ = godotenv.Load()

	port := os.Getenv("CHAT_RELAY_PORT")
	if port == "" {
		port = "8001"
	}

	dbName := os.Getenv("CHAT_DB_FILE")
	if dbName == "" {
		dbName = "./chat_relay.db"
	}
	staticDir := os.Getenv("CHAT_STATIC_DIR")
	if staticDir == "" {
		staticDir = "./chat_relay/static"
	}

	var err error
	db.HostDB, err = db.InitSQLite(dbName)
	if err != nil {
		log.Fatal("Error opening chat relay database:", err)
	}
	defer db.CloseDB(db.HostDB)
	if err := ensureChatRelaySchema(); err != nil {
		log.Fatal("Error ensuring chat relay schema:", err)
	}

	r := gin.Default()

	store := ratelimit.InMemoryStore(&ratelimit.InMemoryOptions{Rate: time.Second, Limit: 150})
	r.Use(ratelimit.RateLimiter(store, &ratelimit.Options{ErrorHandler: rateLimiterrorHandler, KeyFunc: keyFunc}))
	r.Use(cors.Default())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/client")
	})

	r.GET("/ws", HandleSocket)

	r.GET("/api/host/:uuid", HandleGetHost)
	r.POST("/api/hosts_by_uuids", HandleGetHostsByUUIDs)
	r.POST("/api/host_offline/:uuid", HandleUpdateHostOffline)
	r.POST("/api/register_host", HandleRegisterHost)

	r.POST("/api/user_by_id", HandleGetUserByID)
	r.POST("/api/user_by_pubkey", HandleGetUserByPubKey)
	r.POST("/api/users_by_ids", HandleGetUsersByIDs)

	r.GET("/client", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "client", "index.html"))
	})
	r.GET("/client/", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "client", "index.html"))
	})
	r.Static("/client/assets", filepath.Join(staticDir, "client", "assets"))

	server := &http.Server{Addr: ":" + port, Handler: r}

	go func() {
		log.Printf("Starting chat relay on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down chat relay...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("chat relay forced shutdown: %v", err)
	}
}
