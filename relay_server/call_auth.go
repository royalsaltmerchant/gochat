package main

import (
	"database/sql"
	"fmt"
	"gochat/db"
	"os"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// HandleCallRegister handles POST /call/register
func HandleCallRegister(c *gin.Context) {
	var json struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Username string `json:"username"`
	}

	if err := c.BindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}

	if json.Email == "" || json.Password == "" || json.Username == "" {
		c.JSON(400, gin.H{"error": "Email, password, and username are required"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(json.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "Password hashing failed"})
		return
	}

	lowerEmail := strings.ToLower(json.Email)

	var userData UserData
	query := `INSERT INTO users (username, email, password) VALUES (?, ?, ?) RETURNING id, username, email, password`
	err = db.HostDB.QueryRow(query, json.Username, lowerEmail, hashedPassword).
		Scan(&userData.ID, &userData.Username, &userData.Email, &userData.Password)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
			c.JSON(409, gin.H{"error": "Email is already taken"})
			return
		}
		c.JSON(500, gin.H{"error": "Database error inserting user data"})
		return
	}

	token, err := generateJWT(userData, time.Hour*672)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate JWT token"})
		return
	}

	c.JSON(200, gin.H{
		"token":    token,
		"user_id":  userData.ID,
		"username": userData.Username,
		"email":    userData.Email,
	})
}

// HandleCallLogin handles POST /call/login
func HandleCallLogin(c *gin.Context) {
	var json struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}

	if json.Email == "" || json.Password == "" {
		c.JSON(400, gin.H{"error": "Email and password are required"})
		return
	}

	lowerEmail := strings.ToLower(json.Email)

	var userData UserData
	query := `SELECT id, username, email, password FROM users WHERE email = ?`
	err := db.HostDB.QueryRow(query, lowerEmail).
		Scan(&userData.ID, &userData.Username, &userData.Email, &userData.Password)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(401, gin.H{"error": "Invalid email or password"})
			return
		}
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(userData.Password), []byte(json.Password))
	if err != nil {
		c.JSON(401, gin.H{"error": "Invalid email or password"})
		return
	}

	token, err := generateJWT(userData, time.Hour*672)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate JWT token"})
		return
	}

	c.JSON(200, gin.H{
		"token":    token,
		"user_id":  userData.ID,
		"username": userData.Username,
		"email":    userData.Email,
	})
}

// HandleCallLoginByToken handles POST /call/login-by-token
func HandleCallLoginByToken(c *gin.Context) {
	var json struct {
		Token string `json:"token"`
	}

	if err := c.BindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}

	if json.Token == "" {
		c.JSON(400, gin.H{"error": "Token is required"})
		return
	}

	jwtSecret := os.Getenv("JWT_SECRET")

	token, err := jwt.Parse(json.Token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		c.JSON(401, gin.H{"error": "Invalid or expired token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(401, gin.H{"error": "Invalid token claims"})
		return
	}

	userIDFloat, ok := claims["userID"].(float64)
	if !ok {
		c.JSON(401, gin.H{"error": "Invalid user ID in token"})
		return
	}
	userID := int(userIDFloat)

	username, _ := claims["userUsername"].(string)
	email, _ := claims["userEmail"].(string)

	c.JSON(200, gin.H{
		"user_id":  userID,
		"username": username,
		"email":    email,
	})
}

// HandleCallAccount handles GET /call/api/account
func HandleCallAccount(c *gin.Context) {
	userID, err := extractUserIDFromAuth(c)
	if err != nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	var subscriptionStatus string
	var creditMinutes int
	query := `SELECT COALESCE(subscription_status, 'none'), COALESCE(credit_minutes, 0) FROM users WHERE id = ?`
	err = db.HostDB.QueryRow(query, userID).Scan(&subscriptionStatus, &creditMinutes)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	c.JSON(200, gin.H{
		"subscription_status": subscriptionStatus,
		"credit_minutes":      creditMinutes,
	})
}

// extractUserIDFromAuth parses the Bearer token from the Authorization header and returns the userID.
func extractUserIDFromAuth(c *gin.Context) (int, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return 0, fmt.Errorf("missing Authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return 0, fmt.Errorf("invalid Authorization header format")
	}

	tokenString := parts[1]
	jwtSecret := os.Getenv("JWT_SECRET")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return 0, fmt.Errorf("invalid or expired token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("invalid token claims")
	}

	userIDFloat, ok := claims["userID"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid user ID in token")
	}

	return int(userIDFloat), nil
}
