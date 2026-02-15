package main

import (
	"fmt"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
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
