package main

import (
	"database/sql"
	"fmt"
	"gochat/db"
	"log"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

func generateJWT(userData UserData, jwtSecret string, expirationTime time.Duration) (string, error) {
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

func handleLoginUser(conn *websocket.Conn, wsMsg *WSMessage, jwtSecret string) {
	data, err := decodeData[ApproveLoginUser](wsMsg.Data)
	if err != nil {
		log.Println("error decoding login_user_request:", err)
		return
	}

	var userData UserData
	query := `SELECT * FROM users WHERE email = ?`
	err = db.ChatDB.QueryRow(query, data.Email).Scan(&userData.ID, &userData.Username, &userData.Email, &userData.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{
					Content:    "User not found by email",
					ClientUUID: data.ClientUUID,
				},
			})
			return
		} else {
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{
					Content:    "Database Error extracting user data",
					ClientUUID: data.ClientUUID,
				},
			})
			return
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(userData.Password), []byte(data.Password))
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Incorrect password",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	token, err := generateJWT(userData, jwtSecret, time.Hour*672) // 28 days
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Failed to generate JWT token",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	sendToConn(conn, WSMessage{
		Type: "login_approved",
		Data: ApprovedLoginUser{
			UserID:     userData.ID,
			Username:   userData.Username,
			Token:      token,
			ClientUUID: data.ClientUUID,
		},
	})

}

func handleLoginUserByToken(conn *websocket.Conn, wsMsg *WSMessage, jwtSecret string) {
	data, err := decodeData[ApproveLoginUserByToken](wsMsg.Data)
	if err != nil {
		log.Println("error decoding login_user_request:", err)
		return
	}

	// Parse the JWT token
	token, err := jwt.Parse(data.Token, func(token *jwt.Token) (interface{}, error) {
		// Ensure the token's signing method is correct
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Failed to parse token",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userIDFloat, ok := claims["userID"].(float64)
		if !ok {
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{
					Content:    "Invalid User ID in token claims",
					ClientUUID: data.ClientUUID,
				},
			})
			return
		}
		userID := int(userIDFloat)
		username := claims["userUsername"].(string)
		// email := claims["userEmail"].(string)

		sendToConn(conn, WSMessage{
			Type: "login_approved",
			Data: ApprovedLoginUser{
				UserID:     userID,
				Username:   username,
				ClientUUID: data.ClientUUID,
				Token:      data.Token,
			},
		})
	}
}

func handleRegisterUser(conn *websocket.Conn, wsMsg *WSMessage, jwtSecret string) {
	data, err := decodeData[ApproveRegisterUser](wsMsg.Data)
	if err != nil {
		log.Println("error decoding login_user_request:", err)
		return
	}

	var userData UserData

	query := `
	INSERT INTO users (username, email, password)
	VALUES (?, ?, ?)
	RETURNING id, username, email, password
`

	err = db.ChatDB.QueryRow(query, data.Username, data.Email, data.Password).
		Scan(&userData.ID, &userData.Username, &userData.Email, &userData.Password)

	if err != nil {
		// Duplicate email case
		if err.Error() == "UNIQUE constraint failed: users.email" {
			sendToConn(conn, WSMessage{
				Type: "error",
				Data: ChatError{
					Content:    "Email is already taken",
					ClientUUID: data.ClientUUID,
				},
			})
			return
		}

		// Other DB errors
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error inserting user data",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	token, err := generateJWT(userData, jwtSecret, time.Hour*672) // 28 days
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Failed to generate JWT token",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	sendToConn(conn, WSMessage{
		Type: "login_approved",
		Data: ApprovedLoginUser{
			UserID:     userData.ID,
			Username:   userData.Username,
			ClientUUID: data.ClientUUID,
			Token:      token,
		},
	})
}

func handleUpdateUsername(conn *websocket.Conn, wsMsg *WSMessage, jwtSecret string) {
	data, err := decodeData[UpdateUsernameRequest](wsMsg.Data)
	if err != nil {
		log.Println("error decoding update_username_request:", err)
		return
	}

	var userData UserData
	query := `UPDATE users SET username = ? WHERE id = ? RETURNING id, username, email`
	err = db.ChatDB.QueryRow(query, data.Username, data.UserID).Scan(
		&userData.ID,
		&userData.Username,
		&userData.Email,
	)
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Database error updating username",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	token, err := generateJWT(userData, jwtSecret, time.Hour*672) // 28 days
	if err != nil {
		sendToConn(conn, WSMessage{
			Type: "error",
			Data: ChatError{
				Content:    "Failed to generate JWT token",
				ClientUUID: data.ClientUUID,
			},
		})
		return
	}

	sendToConn(conn, WSMessage{
		Type: "update_username_approved",
		Data: UpdateUsernameResponse{
			UserID:     userData.ID,
			Username:   userData.Username,
			Token:      token,
			ClientUUID: data.ClientUUID,
		},
	})

}

// func sendPasswordResetEmail(userEmail string, resetToken string) error {
// 	// Construct the password reset link (ensure the reset endpoint is properly set up)
// 	resetLink := fmt.Sprintf("https://yourdomain.com/reset-password?token=%s", resetToken)

// 	// Sender data
// 	from := "your-email@example.com"
// 	password := "your-email-password"

// 	// Recipient data
// 	to := []string{userEmail}

// 	// Set up the message
// 	subject := "Password Reset Request"
// 	body := fmt.Sprintf("To reset your password, please click the following link: %s", resetLink)

// 	message := []byte("Subject: " + subject + "\r\n" +
// 		"To: " + userEmail + "\r\n" +
// 		"Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n" +
// 		body)

// 	// Set up authentication information
// 	auth := smtp.PlainAuth("", from, password, "smtp.example.com")

// 	// Send email
// 	err := smtp.SendMail("smtp.example.com:587", auth, from, to, message)
// 	return err
// }

// func HandlePasswordResetRequest(c *gin.Context) {
// 	var json struct {
// 		Email string `json:"email"`
// 	}

// 	// Bind incoming JSON
// 	if err := c.BindJSON(&json); err != nil {
// 		c.JSON(400, gin.H{"error": "Invalid request data"})
// 		return
// 	}

// 	// Check if the email exists in the database
// 	var userData types.UserData
// 	query := `SELECT email FROM users WHERE email = ?`
// 	err := db.ChatDB.QueryRow(query, json.Email).Scan(&userData.Email)
// 	if err != nil {
// 		if err == sql.ErrNoRows {
// 			c.JSON(400, gin.H{"error": "User not found by email"})
// 		} else {
// 			c.JSON(500, gin.H{"error": "Error querying the database"})
// 		}
// 		return
// 	}

// 	// Generate a password reset token
// 	resetToken, err := generateJWT(userData, time.Hour*2) // 2 hours
// 	if err != nil {
// 		c.JSON(500, gin.H{"error": "Failed to generate reset token"})
// 		return
// 	}

// 	// Send the reset link via email
// 	err = sendPasswordResetEmail(userData.Email, resetToken)
// 	if err != nil {
// 		log.Println("Error sending email:", err)
// 		c.JSON(500, gin.H{"error": "Failed to send password reset email"})
// 		return
// 	}

// 	c.JSON(200, gin.H{"message": "Password reset email sent"})
// }
