package main

import (
	"database/sql"
	"fmt"
	"gochat/db"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	jwt "github.com/dgrijalva/jwt-go"
)

func generateJWT(userData UserData, expirationTime time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"userID":       userData.ID,
		"userEmail":    userData.Email,
		"userUsername": userData.Username,
		"exp":          time.Now().Add(expirationTime).Unix(), // Token expiration
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := os.Getenv("JWT_SECRET")
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func handleRegisterUser(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[RegisterUser](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid registration data"}})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password), bcrypt.DefaultCost)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Password hashing failed"}})
		return
	}

	// ensure email is lowercase
	lowerEmail := strings.ToLower(data.Email)

	var userData UserData

	query := `
	INSERT INTO users (username, email, password)
	VALUES (?, ?, ?)
	RETURNING id, username, email, password
`

	err = db.HostDB.QueryRow(query, data.Username, lowerEmail, hashedPassword).
		Scan(&userData.ID, &userData.Username, &userData.Email, &userData.Password)

	if err != nil {
		// Duplicate email case
		if err.Error() == "UNIQUE constraint failed: users.email" {
			SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
				Type: "error",
				Data: ChatError{
					Content: "Email is already taken",
				},
			})
			return
		}

		// Other DB errors
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{
				Content: "Database error inserting user data",
			},
		})
		return
	}

	token, err := generateJWT(userData, time.Hour*672) // 28 days
	if err != nil {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{
				Content: "Failed to generate JWT token",
			},
		})
		return
	}

	handleLoginApproved(client, conn, userData.ID, userData.Username, token)
}

func handleLogin(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[LoginUser](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid login data"}})
		return
	}

	var userData UserData
	query := `SELECT * FROM users WHERE email = ?`
	err = db.HostDB.QueryRow(query, data.Email).Scan(&userData.ID, &userData.Username, &userData.Email, &userData.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
				Type: "error",
				Data: ChatError{
					Content: "User not found by email",
				},
			})
			return
		} else {
			SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
				Type: "error",
				Data: ChatError{
					Content: "Database Error extracting user data",
				},
			})
			return
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(userData.Password), []byte(data.Password))
	if err != nil {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{
				Content: "Incorrect password",
			},
		})
		return
	}

	token, err := generateJWT(userData, time.Hour*672) // 28 days
	if err != nil {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{
				Content: "Failed to generate JWT token",
			},
		})
		return
	}

	handleLoginApproved(client, conn, userData.ID, userData.Username, token)
}

func handleLoginByToken(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[LoginUserByToken](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid login data"}})
		return
	}

	jwtSecret := os.Getenv("JWT_SECRET")

	// Parse the JWT token
	token, err := jwt.Parse(data.Token, func(token *jwt.Token) (interface{}, error) {
		// Ensure the token's signing method is correct
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{
				Content: "Failed to parse token",
			},
		})
		return
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userIDFloat, ok := claims["userID"].(float64)
		if !ok {
			SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
				Type: "error",
				Data: ChatError{
					Content: "Invalid User ID in token claims",
				},
			})
			return
		}
		userID := int(userIDFloat)
		username := claims["userUsername"].(string)
		// email := claims["userEmail"].(string)

		handleLoginApproved(client, conn, userID, username, data.Token)
	}
}

func handleLoginApproved(client *Client, conn *websocket.Conn, userID int, username string, token string) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	clientConn, ok := host.ClientConnsByUUID[client.ClientUUID]
	if !ok {
		log.Printf("SendToClient: client not connected to host %s\n", client.HostUUID)
		host.mu.Unlock()
		return
	}
	host.mu.Unlock()

	// login user
	host.ClientsByConn[clientConn].UserID = userID
	host.ClientsByConn[clientConn].Username = username
	host.ClientsByUserID[userID] = host.ClientsByConn[clientConn]

	SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
		Type: "login_user_success",
		Data: LoginUserToken{
			Token: token,
		},
	})
}

func handleUpdateUsername(client *Client, conn *websocket.Conn, wsMsg *WSMessage) {
	data, err := decodeData[UpdateUsernameRequest](wsMsg.Data)
	if err != nil {
		safeSend(client, conn, WSMessage{Type: "error", Data: ChatError{Content: "Invalid update username data"}})
		return
	}

	var userData UserData
	query := `UPDATE users SET username = ? WHERE id = ? RETURNING id, username, email`
	err = db.HostDB.QueryRow(query, data.Username, data.UserID).Scan(
		&userData.ID,
		&userData.Username,
		&userData.Email,
	)
	if err != nil {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{
				Content: "Database error updating username",
			},
		})
		return
	}

	token, err := generateJWT(userData, time.Hour*672) // 28 days
	if err != nil {
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "error",
			Data: ChatError{
				Content: "Failed to generate JWT token",
			},
		})
		return
	}

	handleUpdateUsernameApproved(client, conn, data.UserID, data.Username, token)
}

func handleUpdateUsernameApproved(client *Client, conn *websocket.Conn, userID int, username string, token string) {
	host, exists := GetHost(client.HostUUID)
	if !exists {
		log.Printf("host %s not found\n", client.HostUUID)
		SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
			Type: "author_error",
			Data: ChatError{Content: "Failed to connect to the host"},
		})
		return
	}

	host.mu.Lock()
	clientConn, ok := host.ClientConnsByUUID[client.ClientUUID]
	if !ok {
		host.mu.Unlock()
		log.Printf("SendToClient: client not connected to host %s\n", client.HostUUID)
		return
	}
	host.mu.Unlock()

	// update user
	host.ClientsByConn[clientConn].Username = username

	SendToClient(client.HostUUID, client.ClientUUID, WSMessage{
		Type: "update_username_success",
		Data: UpdateUsername{
			UserID:   userID,
			Username: username,
			Token:    token,
		},
	})
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
	query := `SELECT id, username, email FROM users WHERE email = ?`
	err := db.HostDB.QueryRow(query, json.Email).Scan(&userData.ID, &userData.Username, &userData.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(400, gin.H{"error": "User not found by email"})
		} else {
			c.JSON(500, gin.H{"error": "Error querying the database for user"})
		}
		return
	}

	// Generate a password reset token
	resetToken, err := generateJWT(userData, time.Hour*2) // 2 hours
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate reset token"})
		return
	}

	recipient := userData.Email
	subject := "Parch Password Reset"
	var link = url.URL{Scheme: "https", Host: relayHost, Path: "/reset_password"}
	q := url.Values{}
	q.Set("token", resetToken)
	link.RawQuery = q.Encode()
	finalURL := link.String()

	body := fmt.Sprintf("Please follow this link to reset your password: %s", finalURL)

	// Send the reset link via email
	SendEmail(recipient, subject, body)

	c.JSON(200, gin.H{"message": "Password reset email sent to: " + userData.Email})
}

func HandlePasswordReset(c *gin.Context) {
	var json struct {
		Password string `json:"password"`
		Token    string `json:"token"`
	}

	// Bind incoming JSON
	if err := c.BindJSON(&json); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}

	// Validate token
	jwtSecret := os.Getenv("JWT_SECRET")

	// Parse the JWT token
	token, err := jwt.Parse(json.Token, func(token *jwt.Token) (interface{}, error) {
		// Ensure the token's signing method is correct
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		c.JSON(400, gin.H{"error": "Invalid link. Token invalid."})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	{
		if !ok {
			c.JSON(500, gin.H{"error": "Failed to extract email from token"})
			return
		}

	}
	email := claims["userEmail"].(string)

	// hash and save

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(json.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to hash password"})
		return
	}

	query := `UPDATE users SET password = ? WHERE email = ?`
	res, err := db.HostDB.Exec(query, hashedPassword, email)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to update the new password to this email address"})
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		c.JSON(500, gin.H{"error": "Failed to update the new password to this email address"})
	}

	c.JSON(200, gin.H{"message": "Successfully updated password for " + email})
}
