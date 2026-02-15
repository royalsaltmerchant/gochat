package main

import (
	"os"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

func generateJWT(userData UserData, expirationTime time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"userID":       userData.ID,
		"userEmail":    userData.Email,
		"userUsername": userData.Username,
		"exp":          time.Now().Add(expirationTime).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := os.Getenv("JWT_SECRET")
	return token.SignedString([]byte(jwtSecret))
}
