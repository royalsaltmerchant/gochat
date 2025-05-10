package routes

import (
	"gochat/invites"
	"gochat/spaces"
	auth "gochat/users"

	"github.com/gin-gonic/gin"
)

func SetupAPIRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		// User
		api.POST("/register", auth.HandleRegister)
		api.POST("/login", auth.ValidateCSRFMiddleware(), auth.HandleLogin)
		api.POST("/logout", auth.HandleLogout)
		api.PUT("/update_username", auth.AuthMiddleware(), auth.HandleUpdateUsername)
		// Space
		api.POST("/new_space", auth.AuthMiddleware(), spaces.HandleInsertSpace)
		api.DELETE("/space/:uuid", auth.AuthMiddleware(), spaces.SpaceAuthMiddleware(), spaces.HandleDeleteSpace)
		// Channel
		api.POST("/new_channel", auth.AuthMiddleware(), spaces.HandleInsertChannel)
		api.DELETE("/channel/:uuid", auth.AuthMiddleware(), spaces.ChannelAuthMiddleware(), spaces.HandleDeleteChannel)
		// Space User
		api.POST("/new_space_user/:uuid", auth.AuthMiddleware(), spaces.SpaceAuthMiddleware(), invites.HandleInsertSpaceUser)
		api.DELETE("/space_user/:uuid", auth.AuthMiddleware(), spaces.SpaceAuthMiddleware(), invites.HandleDeleteSpaceUser)
		api.DELETE("/space_user_self/:uuid", auth.AuthMiddleware(), invites.HandleDeleteSpaceUserSelf)
		// Message
		api.GET("/get_messages/:uuid", auth.AuthMiddleware(), spaces.ChannelAuthMiddleware(), spaces.HandleGetMessages)
		// Invites
		api.POST("/accept_invite", auth.AuthMiddleware(), invites.HandleAcceptInvite)
		api.POST("/decline_invite", auth.AuthMiddleware(), invites.HandleDeclineInvite)
		api.GET("/get_invites", auth.AuthMiddleware(), invites.HandleGetInvites)

		// Full Dashboard
		api.GET("/dashboard_data", auth.AuthMiddleware(), spaces.HandleGetDashData)
	}
}
