package routes

import (
	"gochat/auth"
	"gochat/spaces"

	"github.com/gin-gonic/gin"
)

func SetupAPIRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.POST("/register", auth.HandleRegister)
		api.POST("/login", auth.HandleLogin)
		api.POST("/logout", auth.HandleLogout)
		api.POST("/new_space", auth.AuthMiddleware(), spaces.HandleInsertSpace)
		api.POST("/new_space_user", auth.AuthMiddleware(), spaces.HandleInsertSpaceUser)
		api.POST("/accept_invite", auth.AuthMiddleware(), spaces.HandleAcceptInvite)
		api.POST("/decline_invite", auth.AuthMiddleware(), spaces.HandleDeclineInvite)
		api.POST("/new_channel", auth.AuthMiddleware(), spaces.HandleInsertChannel)
		api.POST("/get_messages", auth.AuthMiddleware(), spaces.HandleGetMessages)
		api.DELETE("/space/:uuid", auth.AuthMiddleware(), spaces.HandleDeleteSpace)
	}
}
