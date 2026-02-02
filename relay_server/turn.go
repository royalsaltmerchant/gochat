package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func generateTurnCredentials(secret string, ttlSeconds int64) (username string, password string) {
	unixTime := time.Now().Unix() + ttlSeconds
	username = fmt.Sprintf("%d", unixTime)

	h := hmac.New(sha1.New, []byte(secret))
	h.Write([]byte(username))
	password = base64.StdEncoding.EncodeToString(h.Sum(nil))

	return username, password
}

func HandleGetTurnCredentials(c *gin.Context) {
	// Check authentication - either API key or authenticated IP session
	apiKey := c.GetHeader("X-API-Key")
	expectedAPIKey := os.Getenv("TURN_API_KEY")
	clientIP := c.ClientIP()

	isAuthorized := false

	// Method 1: API key authentication (for third-party services)
	if apiKey != "" && expectedAPIKey != "" && apiKey == expectedAPIKey {
		isAuthorized = true
	}

	// Method 2: IP session authentication (for existing clients via relay WebSocket)
	if !isAuthorized && IsIPAuthenticated(clientIP) {
		isAuthorized = true
	}

	// If TURN_API_KEY is not set, allow unauthenticated access (backwards compatibility)
	// Remove this block once all clients are updated
	if !isAuthorized && expectedAPIKey == "" {
		isAuthorized = true
	}

	if !isAuthorized {
		c.JSON(401, gin.H{
			"error": "Unauthorized: valid API key or authenticated session required",
		})
		return
	}

	url := os.Getenv("TURN_URL")
	secret := os.Getenv("TURN_SECRET") // must match coturn config
	ttl := int64(8 * 3600)             // 8 hours

	if url == "" || secret == "" {
		c.JSON(500, gin.H{
			"error": "Missing TURN_URL or TURN_SECRET in environment",
		})
		return
	}

	username, credential := generateTurnCredentials(secret, ttl)

	c.JSON(200, gin.H{
		"url":        url,
		"username":   username,
		"credential": credential,
	})
}
