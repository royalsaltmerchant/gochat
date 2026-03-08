package gm

import (
	"database/sql"
	"errors"
	"strings"
)

var (
	ErrNotFound                 = errors.New("not found")
	ErrForbidden                = errors.New("forbidden")
	errRepositoryNotInitialized = errors.New("gm repository not initialized")
	gmDB                        *sql.DB
)

func initRepository(db *sql.DB) {
	gmDB = db
}

func currentDB() (*sql.DB, error) {
	if gmDB == nil {
		return nil, errRepositoryNotInitialized
	}
	return gmDB, nil
}

func repoGetProfile(userID int) (Profile, error) {
	db, err := currentDB()
	if err != nil {
		return Profile{}, err
	}

	var p Profile
	query := `
SELECT
    u.id,
    COALESCE(gp.display_name, u.username),
    COALESCE(gp.bio, ''),
    COALESCE(gp.timezone, 'UTC'),
    COALESCE(gp.stripe_connect_account_id, ''),
    COALESCE(gp.onboarding_status, 'not_started'),
    COALESCE(gp.created_at, datetime('now')),
    COALESCE(gp.updated_at, datetime('now'))
FROM users u
LEFT JOIN gm_profiles gp ON gp.user_id = u.id
WHERE u.id = ?`
	err = db.QueryRow(query, userID).Scan(
		&p.UserID,
		&p.DisplayName,
		&p.Bio,
		&p.Timezone,
		&p.StripeConnectAccountID,
		&p.OnboardingStatus,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Profile{}, ErrNotFound
		}
		return Profile{}, err
	}
	return p, nil
}

func repoUpsertProfile(userID int, req UpsertProfileRequest) (Profile, error) {
	db, err := currentDB()
	if err != nil {
		return Profile{}, err
	}

	query := `
INSERT INTO gm_profiles (user_id, display_name, bio, timezone)
VALUES (?, ?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
    display_name = excluded.display_name,
    bio = excluded.bio,
    timezone = excluded.timezone,
    updated_at = datetime('now')`
	if _, err := db.Exec(query, userID, req.DisplayName, req.Bio, req.Timezone); err != nil {
		return Profile{}, err
	}
	return repoGetProfile(userID)
}

func repoCreateListing(userID int, req CreateListingRequest) (Listing, error) {
	db, err := currentDB()
	if err != nil {
		return Listing{}, err
	}

	var listing Listing
	var isPublished int
	query := `
INSERT INTO gm_listings
    (gm_user_id, title, system_key, description, price_cents, currency, seat_capacity, duration_minutes)
VALUES
    (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, gm_user_id, title, system_key, description, price_cents, currency, seat_capacity, duration_minutes, is_published, created_at, updated_at`
	err = db.QueryRow(
		query,
		userID,
		req.Title,
		req.SystemKey,
		req.Description,
		req.PriceCents,
		req.Currency,
		req.SeatCapacity,
		req.DurationMinutes,
	).Scan(
		&listing.ID,
		&listing.GMUserID,
		&listing.Title,
		&listing.SystemKey,
		&listing.Description,
		&listing.PriceCents,
		&listing.Currency,
		&listing.SeatCapacity,
		&listing.DurationMinutes,
		&isPublished,
		&listing.CreatedAt,
		&listing.UpdatedAt,
	)
	if err != nil {
		return Listing{}, err
	}
	listing.IsPublished = isPublished == 1
	return listing, nil
}

func repoGetListingForOwner(userID, listingID int) (Listing, error) {
	db, err := currentDB()
	if err != nil {
		return Listing{}, err
	}

	var listing Listing
	var isPublished int
	query := `
SELECT id, gm_user_id, title, system_key, description, price_cents, currency, seat_capacity, duration_minutes, is_published, created_at, updated_at
FROM gm_listings
WHERE id = ?`
	err = db.QueryRow(query, listingID).Scan(
		&listing.ID,
		&listing.GMUserID,
		&listing.Title,
		&listing.SystemKey,
		&listing.Description,
		&listing.PriceCents,
		&listing.Currency,
		&listing.SeatCapacity,
		&listing.DurationMinutes,
		&isPublished,
		&listing.CreatedAt,
		&listing.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, err
	}
	if listing.GMUserID != userID {
		return Listing{}, ErrForbidden
	}
	listing.IsPublished = isPublished == 1
	return listing, nil
}

func repoUpdateListing(userID, listingID int, req UpdateListingRequest) (Listing, error) {
	db, err := currentDB()
	if err != nil {
		return Listing{}, err
	}

	if _, err := repoGetListingForOwner(userID, listingID); err != nil {
		return Listing{}, err
	}

	var listing Listing
	var isPublished int
	query := `
UPDATE gm_listings
SET
    title = ?,
    system_key = ?,
    description = ?,
    price_cents = ?,
    currency = ?,
    seat_capacity = ?,
    duration_minutes = ?,
    updated_at = datetime('now')
WHERE id = ? AND gm_user_id = ?
RETURNING id, gm_user_id, title, system_key, description, price_cents, currency, seat_capacity, duration_minutes, is_published, created_at, updated_at`
	err = db.QueryRow(
		query,
		req.Title,
		req.SystemKey,
		req.Description,
		req.PriceCents,
		req.Currency,
		req.SeatCapacity,
		req.DurationMinutes,
		listingID,
		userID,
	).Scan(
		&listing.ID,
		&listing.GMUserID,
		&listing.Title,
		&listing.SystemKey,
		&listing.Description,
		&listing.PriceCents,
		&listing.Currency,
		&listing.SeatCapacity,
		&listing.DurationMinutes,
		&isPublished,
		&listing.CreatedAt,
		&listing.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, err
	}
	listing.IsPublished = isPublished == 1
	return listing, nil
}

func repoPublishListing(userID, listingID int) (Listing, error) {
	db, err := currentDB()
	if err != nil {
		return Listing{}, err
	}

	if _, err := repoGetListingForOwner(userID, listingID); err != nil {
		return Listing{}, err
	}

	var listing Listing
	var isPublished int
	query := `
UPDATE gm_listings
SET is_published = 1, updated_at = datetime('now')
WHERE id = ? AND gm_user_id = ?
RETURNING id, gm_user_id, title, system_key, description, price_cents, currency, seat_capacity, duration_minutes, is_published, created_at, updated_at`
	err = db.QueryRow(query, listingID, userID).Scan(
		&listing.ID,
		&listing.GMUserID,
		&listing.Title,
		&listing.SystemKey,
		&listing.Description,
		&listing.PriceCents,
		&listing.Currency,
		&listing.SeatCapacity,
		&listing.DurationMinutes,
		&isPublished,
		&listing.CreatedAt,
		&listing.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, err
	}
	listing.IsPublished = isPublished == 1
	return listing, nil
}

func repoListPublishedListings(limit, offset int) ([]Listing, error) {
	db, err := currentDB()
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
SELECT
    l.id,
    l.gm_user_id,
    COALESCE(gp.display_name, u.username) AS gm_display_name,
    l.title,
    l.system_key,
    l.description,
    l.price_cents,
    l.currency,
    l.seat_capacity,
    l.duration_minutes,
    l.is_published,
    l.created_at,
    l.updated_at
FROM gm_listings l
JOIN users u ON u.id = l.gm_user_id
LEFT JOIN gm_profiles gp ON gp.user_id = l.gm_user_id
WHERE l.is_published = 1
ORDER BY l.created_at DESC
LIMIT ? OFFSET ?`

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Listing{}
	for rows.Next() {
		var listing Listing
		var isPublished int
		if err := rows.Scan(
			&listing.ID,
			&listing.GMUserID,
			&listing.GMDisplayName,
			&listing.Title,
			&listing.SystemKey,
			&listing.Description,
			&listing.PriceCents,
			&listing.Currency,
			&listing.SeatCapacity,
			&listing.DurationMinutes,
			&isPublished,
			&listing.CreatedAt,
			&listing.UpdatedAt,
		); err != nil {
			return nil, err
		}
		listing.IsPublished = isPublished == 1
		out = append(out, listing)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func repoGetPublishedListingDetail(listingID int) (ListingDetail, error) {
	db, err := currentDB()
	if err != nil {
		return ListingDetail{}, err
	}

	var listing Listing
	var isPublished int
	listingQuery := `
SELECT
    l.id,
    l.gm_user_id,
    COALESCE(gp.display_name, u.username) AS gm_display_name,
    l.title,
    l.system_key,
    l.description,
    l.price_cents,
    l.currency,
    l.seat_capacity,
    l.duration_minutes,
    l.is_published,
    l.created_at,
    l.updated_at
FROM gm_listings l
JOIN users u ON u.id = l.gm_user_id
LEFT JOIN gm_profiles gp ON gp.user_id = l.gm_user_id
WHERE l.id = ? AND l.is_published = 1`
	err = db.QueryRow(listingQuery, listingID).Scan(
		&listing.ID,
		&listing.GMUserID,
		&listing.GMDisplayName,
		&listing.Title,
		&listing.SystemKey,
		&listing.Description,
		&listing.PriceCents,
		&listing.Currency,
		&listing.SeatCapacity,
		&listing.DurationMinutes,
		&isPublished,
		&listing.CreatedAt,
		&listing.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ListingDetail{}, ErrNotFound
		}
		return ListingDetail{}, err
	}
	listing.IsPublished = isPublished == 1

	sessionsQuery := `
SELECT id, listing_id, gm_user_id, starts_at, ends_at, room_id, status, seat_capacity, seats_booked, cancel_reason, created_at, updated_at
FROM gm_sessions
WHERE listing_id = ? AND status IN ('scheduled', 'live')
ORDER BY starts_at ASC`
	rows, err := db.Query(sessionsQuery, listingID)
	if err != nil {
		return ListingDetail{}, err
	}
	defer rows.Close()

	sessions := []Session{}
	for rows.Next() {
		var session Session
		if err := rows.Scan(
			&session.ID,
			&session.ListingID,
			&session.GMUserID,
			&session.StartsAt,
			&session.EndsAt,
			&session.RoomID,
			&session.Status,
			&session.SeatCapacity,
			&session.SeatsBooked,
			&session.CancelReason,
			&session.CreatedAt,
			&session.UpdatedAt,
		); err != nil {
			return ListingDetail{}, err
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return ListingDetail{}, err
	}

	return ListingDetail{Listing: listing, Sessions: sessions}, nil
}

func repoCreateSession(session Session) (Session, error) {
	db, err := currentDB()
	if err != nil {
		return Session{}, err
	}

	query := `
INSERT INTO gm_sessions
    (listing_id, gm_user_id, starts_at, ends_at, room_id, status, seat_capacity, seats_booked, cancel_reason)
VALUES
    (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, listing_id, gm_user_id, starts_at, ends_at, room_id, status, seat_capacity, seats_booked, cancel_reason, created_at, updated_at`

	var out Session
	err = db.QueryRow(
		query,
		session.ListingID,
		session.GMUserID,
		session.StartsAt,
		session.EndsAt,
		session.RoomID,
		session.Status,
		session.SeatCapacity,
		session.SeatsBooked,
		session.CancelReason,
	).Scan(
		&out.ID,
		&out.ListingID,
		&out.GMUserID,
		&out.StartsAt,
		&out.EndsAt,
		&out.RoomID,
		&out.Status,
		&out.SeatCapacity,
		&out.SeatsBooked,
		&out.CancelReason,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return Session{}, err
	}
	return out, nil
}

func repoGetSessionForOwner(userID, sessionID int) (Session, error) {
	db, err := currentDB()
	if err != nil {
		return Session{}, err
	}

	query := `
SELECT id, listing_id, gm_user_id, starts_at, ends_at, room_id, status, seat_capacity, seats_booked, cancel_reason, created_at, updated_at
FROM gm_sessions
WHERE id = ?`
	var session Session
	err = db.QueryRow(query, sessionID).Scan(
		&session.ID,
		&session.ListingID,
		&session.GMUserID,
		&session.StartsAt,
		&session.EndsAt,
		&session.RoomID,
		&session.Status,
		&session.SeatCapacity,
		&session.SeatsBooked,
		&session.CancelReason,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	if session.GMUserID != userID {
		return Session{}, ErrForbidden
	}
	return session, nil
}

func repoListSessionsForOwner(userID int) ([]Session, error) {
	db, err := currentDB()
	if err != nil {
		return nil, err
	}

	query := `
SELECT id, listing_id, gm_user_id, starts_at, ends_at, room_id, status, seat_capacity, seats_booked, cancel_reason, created_at, updated_at
FROM gm_sessions
WHERE gm_user_id = ?
ORDER BY starts_at ASC`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Session{}
	for rows.Next() {
		var session Session
		if err := rows.Scan(
			&session.ID,
			&session.ListingID,
			&session.GMUserID,
			&session.StartsAt,
			&session.EndsAt,
			&session.RoomID,
			&session.Status,
			&session.SeatCapacity,
			&session.SeatsBooked,
			&session.CancelReason,
			&session.CreatedAt,
			&session.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func repoUpdateSession(userID, sessionID int, startsAt, endsAt string, seatCapacity int) (Session, error) {
	db, err := currentDB()
	if err != nil {
		return Session{}, err
	}

	if _, err := repoGetSessionForOwner(userID, sessionID); err != nil {
		return Session{}, err
	}

	query := `
UPDATE gm_sessions
SET
    starts_at = ?,
    ends_at = ?,
    seat_capacity = ?,
    updated_at = datetime('now')
WHERE id = ? AND gm_user_id = ?
RETURNING id, listing_id, gm_user_id, starts_at, ends_at, room_id, status, seat_capacity, seats_booked, cancel_reason, created_at, updated_at`
	var session Session
	err = db.QueryRow(query, startsAt, endsAt, seatCapacity, sessionID, userID).Scan(
		&session.ID,
		&session.ListingID,
		&session.GMUserID,
		&session.StartsAt,
		&session.EndsAt,
		&session.RoomID,
		&session.Status,
		&session.SeatCapacity,
		&session.SeatsBooked,
		&session.CancelReason,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	return session, nil
}

func repoCancelSession(userID, sessionID int, reason string) (Session, error) {
	db, err := currentDB()
	if err != nil {
		return Session{}, err
	}

	if _, err := repoGetSessionForOwner(userID, sessionID); err != nil {
		return Session{}, err
	}

	reason = strings.TrimSpace(reason)
	query := `
UPDATE gm_sessions
SET
    status = 'canceled',
    cancel_reason = ?,
    updated_at = datetime('now')
WHERE id = ? AND gm_user_id = ?
RETURNING id, listing_id, gm_user_id, starts_at, ends_at, room_id, status, seat_capacity, seats_booked, cancel_reason, created_at, updated_at`
	var session Session
	err = db.QueryRow(query, reason, sessionID, userID).Scan(
		&session.ID,
		&session.ListingID,
		&session.GMUserID,
		&session.StartsAt,
		&session.EndsAt,
		&session.RoomID,
		&session.Status,
		&session.SeatCapacity,
		&session.SeatsBooked,
		&session.CancelReason,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	return session, nil
}
