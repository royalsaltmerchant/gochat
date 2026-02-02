package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

// AuthenticatedSessions tracks IPs with active authenticated WebSocket sessions
// Key: IP address, Value: count of authenticated sessions from that IP
var AuthenticatedSessions = make(map[string]int)
var authenticatedSessionsMu sync.RWMutex

// RegisterAuthenticatedIP increments the session count for an IP
func RegisterAuthenticatedIP(ip string) {
	authenticatedSessionsMu.Lock()
	defer authenticatedSessionsMu.Unlock()
	AuthenticatedSessions[ip]++
}

// UnregisterAuthenticatedIP decrements the session count for an IP
// Removes the entry if count reaches zero
func UnregisterAuthenticatedIP(ip string) {
	authenticatedSessionsMu.Lock()
	defer authenticatedSessionsMu.Unlock()
	if count, exists := AuthenticatedSessions[ip]; exists {
		if count <= 1 {
			delete(AuthenticatedSessions, ip)
		} else {
			AuthenticatedSessions[ip]--
		}
	}
}

// IsIPAuthenticated checks if an IP has at least one authenticated session
func IsIPAuthenticated(ip string) bool {
	authenticatedSessionsMu.RLock()
	defer authenticatedSessionsMu.RUnlock()
	return AuthenticatedSessions[ip] > 0
}

// GetAuthenticatedSessionCount returns the number of authenticated sessions for an IP
func GetAuthenticatedSessionCount(ip string) int {
	authenticatedSessionsMu.RLock()
	defer authenticatedSessionsMu.RUnlock()
	return AuthenticatedSessions[ip]
}

// SFU Token types and functions

type SFUTokenClaims struct {
	UserID      int    `json:"user_id"`
	Username    string `json:"username"`
	ChannelUUID string `json:"channel_uuid"`
	Exp         int64  `json:"exp"`
}

// VoiceCredentials is sent to clients when they join a voice channel
type VoiceCredentials struct {
	TurnURL        string `json:"turn_url"`
	TurnUsername   string `json:"turn_username"`
	TurnCredential string `json:"turn_credential"`
	SFUToken       string `json:"sfu_token"`
	ChannelUUID    string `json:"channel_uuid"`
}

// GenerateSFUToken creates a signed token for SFU access
func GenerateSFUToken(userID int, username string, channelUUID string, secret string, ttl time.Duration) (string, error) {
	claims := SFUTokenClaims{
		UserID:      userID,
		Username:    username,
		ChannelUUID: channelUUID,
		Exp:         time.Now().Add(ttl).Unix(),
	}

	return generateSFUJWT(claims, secret)
}

// ValidateSFUToken verifies an SFU token and returns the claims
func ValidateSFUToken(tokenString string, secret string) (*SFUTokenClaims, error) {
	return parseSFUJWT(tokenString, secret)
}

func generateSFUJWT(claims SFUTokenClaims, secret string) (string, error) {
	jwtClaims := jwt.MapClaims{
		"user_id":      claims.UserID,
		"username":     claims.Username,
		"channel_uuid": claims.ChannelUUID,
		"exp":          claims.Exp,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	return token.SignedString([]byte(secret))
}

func parseSFUJWT(tokenString string, secret string) (*SFUTokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	// Check expiration
	exp, ok := claims["exp"].(float64)
	if !ok || time.Now().Unix() > int64(exp) {
		return nil, fmt.Errorf("token expired")
	}

	userID, _ := claims["user_id"].(float64)
	username, _ := claims["username"].(string)
	channelUUID, _ := claims["channel_uuid"].(string)

	return &SFUTokenClaims{
		UserID:      int(userID),
		Username:    username,
		ChannelUUID: channelUUID,
		Exp:         int64(exp),
	}, nil
}

// HTTP Handlers for internal validation endpoints (used by Caddy)

// HandleValidateIP checks if an IP has an active authenticated session
// Used by Caddy to validate requests before proxying to SFU
func HandleValidateIP(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		ip = c.ClientIP()
	}

	if IsIPAuthenticated(ip) {
		c.JSON(200, gin.H{
			"authorized":    true,
			"ip":            ip,
			"session_count": GetAuthenticatedSessionCount(ip),
		})
		return
	}

	c.JSON(403, gin.H{
		"authorized": false,
		"ip":         ip,
		"error":      "No authenticated session for this IP",
	})
}

// HandleValidateSFUToken validates an SFU access token
// Used by Caddy to validate tokens before proxying to SFU
func HandleValidateSFUToken(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		token = c.GetHeader("X-SFU-Token")
	}

	if token == "" {
		c.JSON(400, gin.H{
			"authorized": false,
			"error":      "Missing token parameter or X-SFU-Token header",
		})
		return
	}

	sfuSecret := os.Getenv("SFU_SECRET")
	if sfuSecret == "" {
		c.JSON(500, gin.H{
			"authorized": false,
			"error":      "SFU_SECRET not configured",
		})
		return
	}

	claims, err := ValidateSFUToken(token, sfuSecret)
	if err != nil {
		c.JSON(403, gin.H{
			"authorized": false,
			"error":      err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"authorized":   true,
		"user_id":      claims.UserID,
		"username":     claims.Username,
		"channel_uuid": claims.ChannelUUID,
		"exp":          claims.Exp,
	})
}
