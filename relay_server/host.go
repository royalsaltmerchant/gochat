package main

import (
	"database/sql"
	"gochat/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ClientHost struct {
	ID       int    `json:"id"`
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	AuthorID string `json:"author_id"`
}

func HandleGetHost(c *gin.Context) {
	uuid := c.Param("uuid")

	var name string
	query := `SELECT name FROM hosts WHERE uuid = ?`
	err := db.HostDB.QueryRow(query, uuid).Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"error": "Host not found by uuid"})
		} else {
			c.JSON(500, gin.H{"error": "Database error finding host"})
		}
		return
	}

	c.JSON(200, gin.H{
		"name": name,
	})

}

func HandleRegisterHost(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	hostUUID := uuid.New().String()
	authorID := uuid.New().String()

	var host ClientHost
	query := `
		INSERT INTO hosts (uuid, name, author_id)
		VALUES (?, ?, ?)
		RETURNING id, uuid, name, author_id
	`
	err := db.HostDB.QueryRow(query, hostUUID, req.Name, authorID).
		Scan(&host.ID, &host.UUID, &host.Name, &host.AuthorID)

	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to register host"})
		return
	}

	c.JSON(201, host)
}
