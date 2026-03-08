package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"gochat/db"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	callPasswordResetTTL          = 2 * time.Hour
	callPasswordResetMinChars     = 8
	callPasswordResetTimeDBLayout = "2006-01-02 15:04:05"
)

func HandleCallPasswordResetRequest(c *gin.Context) {
	var json struct {
		Email string `json:"email"`
	}

	if err := c.BindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(json.Email))
	if email == "" {
		c.JSON(400, gin.H{"error": "Email is required"})
		return
	}

	var userID int
	var username string
	err := db.HostDB.QueryRow(`SELECT id, username FROM users WHERE email = ?`, email).Scan(&userID, &username)
	if err != nil {
		if err == sql.ErrNoRows {
			// Do not reveal whether an email exists.
			c.JSON(200, gin.H{"message": "If an account exists for this email, a reset link has been sent."})
			return
		}
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	token, tokenHash, err := generatePasswordResetToken()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate reset token"})
		return
	}

	expiresAt := time.Now().UTC().Add(callPasswordResetTTL).Format(callPasswordResetTimeDBLayout)

	tx, err := db.HostDB.Begin()
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM call_password_resets WHERE user_id = ?`, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	_, err = tx.Exec(
		`INSERT INTO call_password_resets (user_id, token_hash, expires_at, request_ip) VALUES (?, ?, ?, ?)`,
		userID,
		tokenHash,
		expiresAt,
		c.ClientIP(),
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	resetURL := buildCallResetURL(c, token)
	subject := "Parch Calls password reset"
	body := fmt.Sprintf(
		"Hi %s,\n\nUse this link to reset your password:\n%s\n\nThis link expires in 2 hours.\nIf you didn't request this, you can ignore this email.\n",
		username,
		resetURL,
	)

	if err := SendEmail(email, subject, body); err != nil {
		log.Printf("password reset email send failed for %s: %v", email, err)
	}

	c.JSON(200, gin.H{"message": "If an account exists for this email, a reset link has been sent."})
}

func HandleCallPasswordReset(c *gin.Context) {
	var json struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}

	if err := c.BindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}

	token := strings.TrimSpace(json.Token)
	password := json.Password

	if token == "" {
		c.JSON(400, gin.H{"error": "Reset token is required"})
		return
	}
	if len(password) < callPasswordResetMinChars {
		c.JSON(400, gin.H{"error": "Password must be at least 8 characters"})
		return
	}

	tokenHash := hashResetToken(token)
	now := time.Now().UTC().Format(callPasswordResetTimeDBLayout)

	tx, err := db.HostDB.Begin()
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback()

	var userID int
	err = tx.QueryRow(
		`SELECT user_id
		 FROM call_password_resets
		 WHERE token_hash = ?
		   AND used_at IS NULL
		   AND expires_at > ?
		 LIMIT 1`,
		tokenHash,
		now,
	).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(400, gin.H{"error": "Reset link is invalid or expired"})
			return
		}
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to hash password"})
		return
	}

	if _, err := tx.Exec(`UPDATE users SET password = ? WHERE id = ?`, hashedPassword, userID); err != nil {
		c.JSON(500, gin.H{"error": "Failed to update password"})
		return
	}

	if _, err := tx.Exec(`UPDATE call_password_resets SET used_at = ? WHERE token_hash = ?`, now, tokenHash); err != nil {
		c.JSON(500, gin.H{"error": "Failed to mark reset token used"})
		return
	}

	if _, err := tx.Exec(`DELETE FROM call_password_resets WHERE user_id = ? AND used_at IS NULL`, userID); err != nil {
		c.JSON(500, gin.H{"error": "Failed to clean reset tokens"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	c.JSON(200, gin.H{"message": "Password updated successfully"})
}

func generatePasswordResetToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	return token, hashResetToken(token), nil
}

func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func buildCallResetURL(c *gin.Context, token string) string {
	base := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL"))
	if base != "" {
		if u, err := url.Parse(base); err == nil && u.Host != "" {
			u.Path = "/call/reset-password"
			q := u.Query()
			q.Set("token", token)
			u.RawQuery = q.Encode()
			return u.String()
		}
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xfp := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xfp != "" {
		scheme = strings.TrimSpace(strings.Split(xfp, ",")[0])
	}

	u := url.URL{
		Scheme: scheme,
		Host:   c.Request.Host,
		Path:   "/call/reset-password",
	}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String()
}
