package main

import (
	"database/sql"
	"fmt"
	"gochat/db"
	"log"
	"strings"

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

	// Build SQL query with dynamic number of placeholders (?, ?, ?)
	placeholders := make([]string, len(req.UserIDs))
	args := make([]interface{}, len(req.UserIDs))
	for i, id := range req.UserIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT id, username FROM users WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)

	rows, err := db.HostDB.Query(query, args...)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database query failed"})
		return
	}
	defer rows.Close()

	var users []DashDataUser
	for rows.Next() {
		var user DashDataUser
		if err := rows.Scan(&user.ID, &user.Username); err != nil {
			log.Println("Error scanning user row:", err)
			continue
		}
		users = append(users, user)
	}

	c.JSON(200, users)
}

func HandleGetUserByID(c *gin.Context) {
	var req struct {
		UserID int `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var user DashDataUser
	query := `SELECT id, username FROM users WHERE id = ?`
	err := db.HostDB.QueryRow(query, req.UserID).Scan(&user.ID, &user.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("User not found by user id")
			c.JSON(400, gin.H{"error": "User not found by user id"})
			return
		} else {
			c.JSON(500, gin.H{"error": "Database failed to find user by id"})
		}
	}

	c.JSON(200, user)
}

func HandleGetUserByEmail(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid email format"})
		return
	}

	var user DashDataUser
	query := `SELECT id, username FROM users WHERE email = ?`
	err := db.HostDB.QueryRow(query, req.Email).Scan(&user.ID, &user.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("User not found by email")
			c.JSON(400, gin.H{"error": "User not found"})
		} else {
			c.JSON(500, gin.H{"error": "Database error"})
		}
		return
	}

	c.JSON(200, user)
}
