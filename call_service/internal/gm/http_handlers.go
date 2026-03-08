package gm

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"gochat/call_service/internal/platform"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r gin.IRouter, db *sql.DB) {
	initRepository(db)

	r.GET("/call/api/gm/profile", getProfile)
	r.PUT("/call/api/gm/profile", upsertProfile)

	r.POST("/call/api/gm/listings", createListing)
	r.PUT("/call/api/gm/listings/:id", updateListing)
	r.POST("/call/api/gm/listings/:id/publish", publishListing)

	r.POST("/call/api/gm/sessions", createSession)
	r.GET("/call/api/gm/sessions", listMySessions)
	r.PUT("/call/api/gm/sessions/:id", updateSession)
	r.POST("/call/api/gm/sessions/:id/cancel", cancelSession)

	r.GET("/call/api/listings", listPublicListings)
	r.GET("/call/api/listings/:id", getPublicListing)
}

func getProfile(c *gin.Context) {
	userID, err := platform.ExtractUserIDFromGin(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	profile, err := GetProfile(userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func upsertProfile(c *gin.Context) {
	userID, err := platform.ExtractUserIDFromGin(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req UpsertProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	profile, err := UpsertProfile(userID, req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func createListing(c *gin.Context) {
	userID, err := platform.ExtractUserIDFromGin(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req CreateListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	listing, err := CreateListing(userID, req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, listing)
}

func updateListing(c *gin.Context) {
	userID, err := platform.ExtractUserIDFromGin(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	listingID, ok := parsePathID(c, "id")
	if !ok {
		return
	}

	var req UpdateListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	listing, err := UpdateListing(userID, listingID, req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, listing)
}

func publishListing(c *gin.Context) {
	userID, err := platform.ExtractUserIDFromGin(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	listingID, ok := parsePathID(c, "id")
	if !ok {
		return
	}

	listing, err := PublishListing(userID, listingID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, listing)
}

func createSession(c *gin.Context) {
	userID, err := platform.ExtractUserIDFromGin(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	session, err := CreateSession(userID, req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, session)
}

func listMySessions(c *gin.Context) {
	userID, err := platform.ExtractUserIDFromGin(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessions, err := ListGMSessions(userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func updateSession(c *gin.Context) {
	userID, err := platform.ExtractUserIDFromGin(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID, ok := parsePathID(c, "id")
	if !ok {
		return
	}

	var req UpdateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	session, err := UpdateSession(userID, sessionID, req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, session)
}

func cancelSession(c *gin.Context) {
	userID, err := platform.ExtractUserIDFromGin(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID, ok := parsePathID(c, "id")
	if !ok {
		return
	}

	var req CancelSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	session, err := CancelSession(userID, sessionID, req.Reason)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, session)
}

func listPublicListings(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	listings, err := ListPublishedListings(limit, offset)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"listings": listings})
}

func getPublicListing(c *gin.Context) {
	listingID, ok := parsePathID(c, "id")
	if !ok {
		return
	}

	detail, err := GetPublishedListingDetail(listingID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, platform.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	case errors.Is(err, ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
}

func parsePathID(c *gin.Context, key string) (int, bool) {
	id, err := strconv.Atoi(c.Param(key))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return 0, false
	}
	return id, true
}
