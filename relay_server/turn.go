package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
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

func isJWTAuthorized(c *gin.Context) bool {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return false
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return false
	}

	tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
	if tokenString == "" {
		return false
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return false
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false
	}

	expFloat, ok := claims["exp"].(float64)
	if !ok {
		return false
	}

	return time.Now().Unix() <= int64(expFloat)
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

	// Method 3: JWT bearer auth (for browser/mobile clients)
	if !isAuthorized && isJWTAuthorized(c) {
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
