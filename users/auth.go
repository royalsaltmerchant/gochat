package auth

import (
	"database/sql"
	"fmt"
	"gochat/db"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type UserData struct {
	ID       int
	Username string
	Email    string
	Password string
}

func ValidateCSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("csrf_token")
		fmt.Println(cookie)
		if err != nil {
			c.AbortWithStatusJSON(403, gin.H{"error": "Missing CSRF token cookie"})
			return
		}

		header := c.GetHeader("X-CSRF-Token")
		if header == "" || header != cookie {
			c.AbortWithStatusJSON(403, gin.H{"error": "Invalid CSRF token"})
			return
		}

		c.Next()
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		jwtSecret := os.Getenv("JWT_SECRET")

		tokenString, _ := c.Cookie("auth_token")

		// If no token is found in the cookie, redirect to login
		if tokenString == "" {
			c.Redirect(302, "/login")
			c.Abort()
			return
		}

		tokenString = strings.TrimPrefix(tokenString, "Bearer ")

		// Parse the JWT token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Ensure the token's signing method is correct
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.Redirect(302, "/login")
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			// Set user data on the context directly from claims
			c.Set("userID", int(claims["userID"].(float64)))
			c.Set("userEmail", claims["userEmail"].(string))
			c.Set("userUsername", claims["userUsername"].(string))
		}

		c.Next()
	}
}

func generateJWT(userData UserData, expirationTime time.Duration) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	claims := jwt.MapClaims{
		"userID":       userData.ID,
		"userEmail":    userData.Email,
		"userUsername": userData.Username,
		"exp":          time.Now().Add(expirationTime).Unix(), // Token expiration
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func setCookie(token string, c *gin.Context) {
	isSecure := os.Getenv("ENV") == "production"
	host := strings.Split(c.Request.Host, ":")[0]
	c.SetCookie("auth_token", token, 28*24*3600, "/", host, isSecure, true) // 28 days like the JWT
}

func hashPassword(password string) (string, error) {
	// Hash the password using bcrypt with a cost factor of 14
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func HandleLogin(c *gin.Context) {
	var json struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.BindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}

	var userData UserData
	query := `SELECT * FROM users WHERE email = ?`
	err := db.DB.QueryRow(query, json.Email).Scan(&userData.ID, &userData.Username, &userData.Email, &userData.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(400, gin.H{"error": "User not found by email"})
		} else {
			c.JSON(500, gin.H{"error": "Database Error extracting user data"})
		}
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(userData.Password), []byte(json.Password))
	if err != nil {
		c.JSON(400, gin.H{"error": "Incorrect password"})
		return
	}

	token, err := generateJWT(userData, time.Hour*672) // 28 days
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate JWT token"})
		return
	}

	setCookie(token, c)

	c.JSON(200, gin.H{"token": token})
}

func HandleRegister(c *gin.Context) {
	var json struct {
		Username string `json:"username" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Hash the password
	hashedPassword, err := hashPassword(json.Password)
	if err != nil {
		log.Fatal("Error hashing password:", err)
		c.JSON(500, gin.H{"error": "Failed to hash password"})
		return
	}

	query := `INSERT INTO users (username, email, password) VALUES (?, ?, ?)`
	_, err = db.DB.Exec(query, json.Username, json.Email, hashedPassword)

	if err != nil {
		// Check if the error message contains "UNIQUE constraint failed"
		if err.Error() == "UNIQUE constraint failed: users.email" {
			c.JSON(400, gin.H{"error": "Email is already taken"})
			return
		}

		// For other database errors
		c.JSON(500, gin.H{"error": "Database error inserting data"})
		return
	}

	c.JSON(201, gin.H{"message": "Successfully registered"})
}

func sendPasswordResetEmail(userEmail string, resetToken string) error {
	// Construct the password reset link (ensure the reset endpoint is properly set up)
	resetLink := fmt.Sprintf("https://yourdomain.com/reset-password?token=%s", resetToken)

	// Sender data
	from := "your-email@example.com"
	password := "your-email-password"

	// Recipient data
	to := []string{userEmail}

	// Set up the message
	subject := "Password Reset Request"
	body := fmt.Sprintf("To reset your password, please click the following link: %s", resetLink)

	message := []byte("Subject: " + subject + "\r\n" +
		"To: " + userEmail + "\r\n" +
		"Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n" +
		body)

	// Set up authentication information
	auth := smtp.PlainAuth("", from, password, "smtp.example.com")

	// Send email
	err := smtp.SendMail("smtp.example.com:587", auth, from, to, message)
	return err
}

func HandlePasswordResetRequest(c *gin.Context) {
	var json struct {
		Email string `json:"email"`
	}

	// Bind incoming JSON
	if err := c.BindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}

	// Check if the email exists in the database
	var userData UserData
	query := `SELECT email FROM users WHERE email = ?`
	err := db.DB.QueryRow(query, json.Email).Scan(&userData.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(400, gin.H{"error": "User not found by email"})
		} else {
			c.JSON(500, gin.H{"error": "Error querying the database"})
		}
		return
	}

	// Generate a password reset token
	resetToken, err := generateJWT(userData, time.Hour*2) // 2 hours
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate reset token"})
		return
	}

	// Send the reset link via email
	err = sendPasswordResetEmail(userData.Email, resetToken)
	if err != nil {
		log.Println("Error sending email:", err)
		c.JSON(500, gin.H{"error": "Failed to send password reset email"})
		return
	}

	c.JSON(200, gin.H{"message": "Password reset email sent"})
}

func HandleLogout(c *gin.Context) {
	host := strings.Split(c.Request.Host, ":")[0]
	isSecure := os.Getenv("ENV") == "production"
	// Overwrite the cookie with an expired one
	c.SetCookie(
		"auth_token", // name
		"",           // value
		-1,           // maxAge (negative = delete immediately)
		"/",          // path
		host,         // domain ("" = current domain)
		isSecure,     // secure
		true,         // httpOnly
	)

	c.JSON(200, gin.H{"message": "Logged out successfully"})
}

func HandleUpdateUsername(c *gin.Context) {
	var json struct {
		Username string `json:"username" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("userID")
	userEmail, _ := c.Get("userEmail")

	// Update the username in the database
	query := `UPDATE users SET username = ? WHERE id = ?`
	res, err := db.DB.Exec(query, json.Username, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error updating username"})
		return
	}

	// Check if any rows were affected
	if rows, _ := res.RowsAffected(); rows == 0 {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}

	userData := UserData{
		ID:       userID.(int),
		Username: json.Username,
		Email:    userEmail.(string),
	}

	token, err := generateJWT(userData, time.Hour*672) // 28 days
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate new JWT token"})
		return
	}

	setCookie(token, c)

	c.JSON(200, gin.H{
		"message": "Username updated successfully",
		"token":   token,
	})
}
