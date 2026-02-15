package main

import (
	"database/sql"
	"gochat/db"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func HandleGetHostsByUUIDs(c *gin.Context) {
	var req UUIDListRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.UUIDs) == 0 {
		c.JSON(400, gin.H{"error": "Invalid or missing UUID list"})
		return
	}

	placeholders := make([]string, len(req.UUIDs))
	args := make([]interface{}, len(req.UUIDs))
	for i, uuid := range req.UUIDs {
		placeholders[i] = "?"
		args[i] = uuid
	}

	query := `SELECT uuid, name, online FROM hosts WHERE uuid IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := db.HostDB.Query(query, args...)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database query error"})
		return
	}
	defer rows.Close()

	var results []gin.H
	for rows.Next() {
		var uuid, name string
		var online int
		if err := rows.Scan(&uuid, &name, &online); err != nil {
			continue
		}
		results = append(results, gin.H{
			"uuid":   uuid,
			"name":   name,
			"online": online,
		})
	}

	c.JSON(200, gin.H{"hosts": results})
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
		"uuid": uuid,
	})
}

func HandleUpdateHostOffline(c *gin.Context) { // external
	uuid := c.Param("uuid")

	var req struct {
		AuthorID string `json:"author_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	query := `UPDATE hosts SET online = 0 WHERE uuid = ? AND author_id = ?`
	res, err := db.HostDB.Exec(query, uuid, req.AuthorID)
	if err != nil {
		log.Println("Database error updating host offline")
		c.JSON(500, gin.H{"error": "Database error updating host offline"})
		return
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		log.Println("Database error updating host offline")
		c.JSON(500, gin.H{"error": "Database error updating host offline"})
		return
	}
}

func HandleUpdateHostOnline(uuid string) { // internal
	query := `UPDATE hosts SET online = 1 WHERE uuid = ?`
	res, err := db.HostDB.Exec(query, uuid)
	if err != nil {
		log.Println("Database error updating host online")
		return
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		log.Println("Database error updating host online")
		return
	}
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
