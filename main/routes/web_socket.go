package routes

import (
	cr "gochat/chatroom"
	auth "gochat/users"

	"github.com/gin-gonic/gin"
)

func SetupWebSocketRoutes(r *gin.Engine) {
	// Join chat room endpoint

	r.GET("/ws", auth.AuthMiddleware(), cr.HandleSocket)
}
