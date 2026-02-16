package main

import (
	"fmt"
	"os"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

type SFUTokenClaims struct {
	UserID      int    `json:"user_id"`
	Username    string `json:"username"`
	ChannelUUID string `json:"channel_uuid"`
	Exp         int64  `json:"exp"`
}

type VoiceCredentials struct {
	TurnURL        string `json:"turn_url"`
	TurnUsername   string `json:"turn_username"`
	TurnCredential string `json:"turn_credential"`
	SFUToken       string `json:"sfu_token"`
	ChannelUUID    string `json:"channel_uuid"`
}

// GenerateSFUToken creates a signed token for SFU access.
func GenerateSFUToken(userID int, username string, channelUUID string, secret string, ttl time.Duration) (string, error) {
	claims := SFUTokenClaims{
		UserID:      userID,
		Username:    username,
		ChannelUUID: channelUUID,
		Exp:         time.Now().Add(ttl).Unix(),
	}
	return generateSFUJWT(claims, secret)
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

func ValidateSFUToken(tokenString string, secret string) (*SFUTokenClaims, error) {
	return parseSFUJWT(tokenString, secret)
}

// HandleValidateSFUToken validates an SFU access token for Caddy forward_auth.
func HandleValidateSFUToken(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		token = c.GetHeader("X-SFU-Token")
	}
	if token == "" {
		c.JSON(400, gin.H{
			"authorized": false,
			"error":      "missing token parameter or X-SFU-Token header",
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

// HandleValidateIP provides explicit fallback behavior for legacy IP-based SFU auth.
func HandleValidateIP(c *gin.Context) {
	c.JSON(403, gin.H{
		"authorized": false,
		"error":      "IP-based SFU auth is disabled; provide token",
	})
}
