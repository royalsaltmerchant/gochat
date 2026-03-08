package gm

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidInput = errors.New("invalid input")

func GetProfile(userID int) (Profile, error) {
	return repoGetProfile(userID)
}

func UpsertProfile(userID int, req UpsertProfileRequest) (Profile, error) {
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Bio = strings.TrimSpace(req.Bio)
	req.Timezone = strings.TrimSpace(req.Timezone)

	if req.DisplayName == "" {
		return Profile{}, wrapInvalid("display_name is required")
	}
	if len(req.DisplayName) > 120 {
		return Profile{}, wrapInvalid("display_name is too long")
	}
	if len(req.Bio) > 5000 {
		return Profile{}, wrapInvalid("bio is too long")
	}
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}

	return repoUpsertProfile(userID, req)
}

func CreateListing(userID int, req CreateListingRequest) (Listing, error) {
	normalized, err := normalizeListingRequest(req)
	if err != nil {
		return Listing{}, err
	}
	return repoCreateListing(userID, normalized)
}

func UpdateListing(userID, listingID int, req UpdateListingRequest) (Listing, error) {
	normalized, err := normalizeListingRequest(CreateListingRequest{
		Title:           req.Title,
		SystemKey:       req.SystemKey,
		Description:     req.Description,
		PriceCents:      req.PriceCents,
		Currency:        req.Currency,
		SeatCapacity:    req.SeatCapacity,
		DurationMinutes: req.DurationMinutes,
	})
	if err != nil {
		return Listing{}, err
	}

	return repoUpdateListing(userID, listingID, UpdateListingRequest{
		Title:           normalized.Title,
		SystemKey:       normalized.SystemKey,
		Description:     normalized.Description,
		PriceCents:      normalized.PriceCents,
		Currency:        normalized.Currency,
		SeatCapacity:    normalized.SeatCapacity,
		DurationMinutes: normalized.DurationMinutes,
	})
}

func PublishListing(userID, listingID int) (Listing, error) {
	return repoPublishListing(userID, listingID)
}

func ListPublishedListings(limit, offset int) ([]Listing, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return repoListPublishedListings(limit, offset)
}

func GetPublishedListingDetail(listingID int) (ListingDetail, error) {
	return repoGetPublishedListingDetail(listingID)
}

func CreateSession(userID int, req CreateSessionRequest) (Session, error) {
	if req.ListingID <= 0 {
		return Session{}, wrapInvalid("listing_id is required")
	}

	listing, err := repoGetListingForOwner(userID, req.ListingID)
	if err != nil {
		return Session{}, err
	}

	startsAt, err := parseRFC3339(req.StartsAt)
	if err != nil {
		return Session{}, wrapInvalid("starts_at must be RFC3339")
	}

	var endsAt time.Time
	if strings.TrimSpace(req.EndsAt) == "" {
		endsAt = startsAt.Add(time.Duration(listing.DurationMinutes) * time.Minute)
	} else {
		endsAt, err = parseRFC3339(req.EndsAt)
		if err != nil {
			return Session{}, wrapInvalid("ends_at must be RFC3339")
		}
	}
	if !endsAt.After(startsAt) {
		return Session{}, wrapInvalid("ends_at must be after starts_at")
	}

	seatCapacity := req.SeatCapacity
	if seatCapacity <= 0 {
		seatCapacity = listing.SeatCapacity
	}
	if seatCapacity <= 0 {
		return Session{}, wrapInvalid("seat_capacity must be greater than 0")
	}

	session := Session{
		ListingID:    listing.ID,
		GMUserID:     userID,
		StartsAt:     startsAt.UTC().Format(time.RFC3339),
		EndsAt:       endsAt.UTC().Format(time.RFC3339),
		RoomID:       "gm-session-" + uuid.NewString(),
		Status:       "scheduled",
		SeatCapacity: seatCapacity,
		SeatsBooked:  0,
		CancelReason: "",
	}

	return repoCreateSession(session)
}

func UpdateSession(userID, sessionID int, req UpdateSessionRequest) (Session, error) {
	session, err := repoGetSessionForOwner(userID, sessionID)
	if err != nil {
		return Session{}, err
	}

	if session.Status != "scheduled" {
		return Session{}, wrapInvalid("only scheduled sessions can be updated")
	}

	startsAt, err := parseRFC3339(session.StartsAt)
	if err != nil {
		return Session{}, fmt.Errorf("existing starts_at invalid: %w", err)
	}
	endsAt, err := parseRFC3339(session.EndsAt)
	if err != nil {
		return Session{}, fmt.Errorf("existing ends_at invalid: %w", err)
	}

	if req.StartsAt != nil {
		startsAt, err = parseRFC3339(*req.StartsAt)
		if err != nil {
			return Session{}, wrapInvalid("starts_at must be RFC3339")
		}
	}
	if req.EndsAt != nil {
		endsAt, err = parseRFC3339(*req.EndsAt)
		if err != nil {
			return Session{}, wrapInvalid("ends_at must be RFC3339")
		}
	}
	if !endsAt.After(startsAt) {
		return Session{}, wrapInvalid("ends_at must be after starts_at")
	}

	seatCapacity := session.SeatCapacity
	if req.SeatCapacity != nil {
		if *req.SeatCapacity <= 0 {
			return Session{}, wrapInvalid("seat_capacity must be greater than 0")
		}
		seatCapacity = *req.SeatCapacity
	}
	if seatCapacity < session.SeatsBooked {
		return Session{}, wrapInvalid("seat_capacity cannot be lower than seats_booked")
	}

	return repoUpdateSession(
		userID,
		sessionID,
		startsAt.UTC().Format(time.RFC3339),
		endsAt.UTC().Format(time.RFC3339),
		seatCapacity,
	)
}

func CancelSession(userID, sessionID int, reason string) (Session, error) {
	session, err := repoGetSessionForOwner(userID, sessionID)
	if err != nil {
		return Session{}, err
	}
	if session.Status == "canceled" {
		return Session{}, wrapInvalid("session is already canceled")
	}
	if session.Status == "completed" {
		return Session{}, wrapInvalid("completed session cannot be canceled")
	}

	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "canceled_by_gm"
	}

	return repoCancelSession(userID, sessionID, reason)
}

func ListGMSessions(userID int) ([]Session, error) {
	return repoListSessionsForOwner(userID)
}

func normalizeListingRequest(req CreateListingRequest) (CreateListingRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.SystemKey = strings.ToLower(strings.TrimSpace(req.SystemKey))
	req.Description = strings.TrimSpace(req.Description)
	req.Currency = strings.ToLower(strings.TrimSpace(req.Currency))

	if req.Title == "" {
		return CreateListingRequest{}, wrapInvalid("title is required")
	}
	if len(req.Title) > 160 {
		return CreateListingRequest{}, wrapInvalid("title is too long")
	}
	if req.SystemKey == "" {
		return CreateListingRequest{}, wrapInvalid("system_key is required")
	}
	if req.PriceCents < 0 {
		return CreateListingRequest{}, wrapInvalid("price_cents must be >= 0")
	}
	if req.Currency == "" {
		req.Currency = "usd"
	}
	if len(req.Currency) != 3 {
		return CreateListingRequest{}, wrapInvalid("currency must be a 3-letter code")
	}
	if req.SeatCapacity <= 0 {
		return CreateListingRequest{}, wrapInvalid("seat_capacity must be > 0")
	}
	if req.DurationMinutes <= 0 {
		return CreateListingRequest{}, wrapInvalid("duration_minutes must be > 0")
	}
	if req.DurationMinutes > 24*60 {
		return CreateListingRequest{}, wrapInvalid("duration_minutes is too large")
	}
	if len(req.Description) > 20000 {
		return CreateListingRequest{}, wrapInvalid("description is too long")
	}
	return req, nil
}

func parseRFC3339(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, strings.TrimSpace(value))
}

func wrapInvalid(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
