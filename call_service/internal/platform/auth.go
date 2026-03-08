package platform

import (
	"errors"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
)

var ErrUnauthorized = errors.New("unauthorized")

func ExtractUserIDFromAuthHeader(authHeader string) (int, error) {
	if authHeader == "" {
		return 0, ErrUnauthorized
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return 0, ErrUnauthorized
	}

	tokenString := parts[1]
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return 0, ErrUnauthorized
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return 0, ErrUnauthorized
	}

	userIDFloat, ok := claims["userID"].(float64)
	if !ok {
		return 0, ErrUnauthorized
	}

	return int(userIDFloat), nil
}

func ExtractUserIDFromGin(c *gin.Context) (int, error) {
	return ExtractUserIDFromAuthHeader(c.GetHeader("Authorization"))
}
