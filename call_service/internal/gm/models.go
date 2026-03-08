package gm

type Profile struct {
	UserID                 int    `json:"user_id"`
	DisplayName            string `json:"display_name"`
	Bio                    string `json:"bio"`
	Timezone               string `json:"timezone"`
	StripeConnectAccountID string `json:"stripe_connect_account_id"`
	OnboardingStatus       string `json:"onboarding_status"`
	CreatedAt              string `json:"created_at"`
	UpdatedAt              string `json:"updated_at"`
}

type Listing struct {
	ID              int    `json:"id"`
	GMUserID        int    `json:"gm_user_id"`
	GMDisplayName   string `json:"gm_display_name,omitempty"`
	Title           string `json:"title"`
	SystemKey       string `json:"system_key"`
	Description     string `json:"description"`
	PriceCents      int    `json:"price_cents"`
	Currency        string `json:"currency"`
	SeatCapacity    int    `json:"seat_capacity"`
	DurationMinutes int    `json:"duration_minutes"`
	IsPublished     bool   `json:"is_published"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type Session struct {
	ID           int    `json:"id"`
	ListingID    int    `json:"listing_id"`
	GMUserID     int    `json:"gm_user_id"`
	StartsAt     string `json:"starts_at"`
	EndsAt       string `json:"ends_at"`
	RoomID       string `json:"room_id"`
	Status       string `json:"status"`
	SeatCapacity int    `json:"seat_capacity"`
	SeatsBooked  int    `json:"seats_booked"`
	CancelReason string `json:"cancel_reason"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type ListingDetail struct {
	Listing  Listing   `json:"listing"`
	Sessions []Session `json:"sessions"`
}

type UpsertProfileRequest struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	Timezone    string `json:"timezone"`
}

type CreateListingRequest struct {
	Title           string `json:"title"`
	SystemKey       string `json:"system_key"`
	Description     string `json:"description"`
	PriceCents      int    `json:"price_cents"`
	Currency        string `json:"currency"`
	SeatCapacity    int    `json:"seat_capacity"`
	DurationMinutes int    `json:"duration_minutes"`
}

type UpdateListingRequest struct {
	Title           string `json:"title"`
	SystemKey       string `json:"system_key"`
	Description     string `json:"description"`
	PriceCents      int    `json:"price_cents"`
	Currency        string `json:"currency"`
	SeatCapacity    int    `json:"seat_capacity"`
	DurationMinutes int    `json:"duration_minutes"`
}

type CreateSessionRequest struct {
	ListingID    int    `json:"listing_id"`
	StartsAt     string `json:"starts_at"`
	EndsAt       string `json:"ends_at,omitempty"`
	SeatCapacity int    `json:"seat_capacity,omitempty"`
}

type UpdateSessionRequest struct {
	StartsAt     *string `json:"starts_at,omitempty"`
	EndsAt       *string `json:"ends_at,omitempty"`
	SeatCapacity *int    `json:"seat_capacity,omitempty"`
}

type CancelSessionRequest struct {
	Reason string `json:"reason"`
}
