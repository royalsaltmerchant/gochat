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

func setHostOnlineState(uuid string, online int) {
	query := `UPDATE hosts SET online = ? WHERE uuid = ?`
	res, err := db.HostDB.Exec(query, online, uuid)
	if err != nil {
		log.Println("Database error updating host online state")
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		log.Println("Database error updating host online state")
		return
	}
}

func HandleUpdateHostOnline(uuid string) { // internal
	setHostOnlineState(uuid, 1)
}

func HandleUpdateHostOffline(uuid string) { // internal
	setHostOnlineState(uuid, 0)
}

func HandleRegisterHost(c *gin.Context) {
	var req struct {
		Name             string `json:"name" binding:"required"`
		SigningPublicKey string `json:"signing_public_key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	trimmedSigningKey := strings.TrimSpace(req.SigningPublicKey)
	if trimmedSigningKey == "" {
		c.JSON(400, gin.H{"error": "signing_public_key is required"})
		return
	}

	hostUUID := uuid.New().String()

	var host ClientHost
	query := `
		INSERT INTO hosts (uuid, name, signing_public_key)
		VALUES (?, ?, ?)
		RETURNING id, uuid, name, signing_public_key
	`
	err := db.HostDB.QueryRow(query, hostUUID, req.Name, trimmedSigningKey).
		Scan(&host.ID, &host.UUID, &host.Name, &host.SigningPublicKey)

	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to register host"})
		return
	}

	c.JSON(201, host)
}
