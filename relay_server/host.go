package main

import (
	"database/sql"
	"gochat/db"

	"github.com/gin-gonic/gin"
)

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
