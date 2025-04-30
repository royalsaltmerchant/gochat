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

func JwtMiddleware() gin.HandlerFunc {

	return func(c *gin.Context) {
		jwtSecret := os.Getenv("JWT_SECRET")

		tokenString := c.GetHeader("Authorization")

		if tokenString == "" {
			c.JSON(401, gin.H{"error": "Authorization header required"})
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
			c.JSON(401, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			c.Set("userEmail", claims["userEmail"])
		}

		c.Next()
	}
}

func generateJWT(userEmail string, expirationTime time.Duration) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	claims := jwt.MapClaims{
		"userEmail": userEmail,
		"exp":       time.Now().Add(expirationTime).Unix(), // Token expiration
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
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
		Email    string `json:"email"`
		Password string `json:"password"`
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
			c.JSON(500, gin.H{"error": "Error extracting data"})
		}
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(userData.Password), []byte(json.Password))
	if err != nil {
		c.JSON(400, gin.H{"error": "Incorrect password"})
		return
	}

	token, err := generateJWT(userData.Email, time.Hour*672) // 28 days
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate JWT token"})
		return
	}

	c.JSON(200, gin.H{"auth_token": token})
}

func HandleRegister(c *gin.Context) {
	var json struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request data"})
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
	resetToken, err := generateJWT(userData.Email, time.Hour*2) // 2 hours
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
