package main

import (
	"database/sql"
	"gochat/db"
	"log"

	"github.com/gin-gonic/gin"
)

func HandleGetUsersByIDs(c *gin.Context) {
	var req struct {
		UserIDs []int `json:"user_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	if len(req.UserIDs) == 0 {
		c.JSON(400, gin.H{"error": "user_ids list is empty"})
		return
	}

	users, err := lookupIdentitiesByIDs(req.UserIDs)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database query failed"})
		return
	}

	c.JSON(200, users)
}

func HandleGetUserByID(c *gin.Context) {
	var req struct {
		UserID int `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	user, err := lookupIdentityByID(req.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(400, gin.H{"error": "User not found by user id"})
			return
		}
		c.JSON(500, gin.H{"error": "Database failed to find user by id"})
		return
	}

	c.JSON(200, user)
}

func HandleGetUserByPubKey(c *gin.Context) {
	var req struct {
		PublicKey string `json:"public_key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	user, err := lookupIdentityByPublicKey(req.PublicKey)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("User not found by public key")
			c.JSON(400, gin.H{"error": "User not found"})
			return
		}
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	c.JSON(200, user)
}

func ensureIdentityForConnectedUser(userID int, publicKey string, username string) {
	if userID <= 0 || publicKey == "" {
		return
	}
	_, err := db.HostDB.Exec(
		`INSERT INTO chat_identities (id, public_key, username)
		 VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   public_key = excluded.public_key,
		   username = excluded.username,
		   updated_at = CURRENT_TIMESTAMP`,
		userID,
		publicKey,
		normalizeUsername(username, publicKey),
	)
	if err != nil {
		log.Printf("ensureIdentityForConnectedUser failed user=%d: %v", userID, err)
	}
}
