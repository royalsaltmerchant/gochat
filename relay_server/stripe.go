package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"gochat/db"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	stripe "github.com/stripe/stripe-go/v82"
	portalsession "github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"
)

func ensureStripeKey() {
	if stripe.Key == "" {
		stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	}
}

// HandleCreateCheckoutSession handles POST /call/create-checkout-session
func HandleCreateCheckoutSession(c *gin.Context) {
	ensureStripeKey()
	userID, err := extractUserIDFromAuth(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Type string `json:"type"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	if req.Type != "credits" && req.Type != "subscription" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'credits' or 'subscription'"})
		return
	}

	customerID, err := getOrCreateStripeCustomer(userID)
	if err != nil {
		log.Printf("Error getting/creating Stripe customer for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Stripe customer"})
		return
	}

	baseURL := os.Getenv("BASE_URL")

	var mode string
	var priceID string

	if req.Type == "credits" {
		mode = string(stripe.CheckoutSessionModePayment)
		priceID = os.Getenv("STRIPE_CREDIT_PRICE_ID")
	} else {
		mode = string(stripe.CheckoutSessionModeSubscription)
		priceID = os.Getenv("STRIPE_SUBSCRIPTION_PRICE_ID")
	}

	if priceID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Price ID not configured"})
		return
	}

	userIDStr := strconv.Itoa(userID)
	successURL := baseURL + "/call/account?session_id={CHECKOUT_SESSION_ID}"
	cancelURL := baseURL + "/call"

	params := &stripe.CheckoutSessionParams{
		Customer:          stripe.String(customerID),
		ClientReferenceID: stripe.String(userIDStr),
		Mode:              stripe.String(mode),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
	}

	sess, err := checkoutsession.New(params)
	if err != nil {
		log.Printf("Error creating checkout session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create checkout session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": sess.URL})
}

// HandleStripeWebhook handles POST /call/stripe-webhook
func HandleStripeWebhook(c *gin.Context) {
	ensureStripeKey()
	const maxBodyBytes = 65536
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to read body"})
		return
	}

	endpointSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	sigHeader := c.GetHeader("Stripe-Signature")

	event, err := webhook.ConstructEventWithOptions(body, sigHeader, endpointSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		log.Printf("Webhook signature verification failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid signature"})
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			log.Printf("Error parsing checkout session: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing webhook data"})
			return
		}
		handleCheckoutCompleted(&sess)

	case "invoice.paid":
		var invoice map[string]interface{}
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			log.Printf("Error parsing invoice: %v", err)
			break
		}
		if subID, ok := invoice["subscription"].(string); ok && subID != "" {
			updateSubscriptionStatus(subID, "active")
		}

	case "invoice.payment_failed":
		var invoice map[string]interface{}
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			log.Printf("Error parsing invoice: %v", err)
			break
		}
		if subID, ok := invoice["subscription"].(string); ok && subID != "" {
			updateSubscriptionStatus(subID, "past_due")
		}

	case "customer.subscription.deleted":
		var sub map[string]interface{}
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			log.Printf("Error parsing subscription: %v", err)
			break
		}
		if subID, ok := sub["id"].(string); ok && subID != "" {
			updateSubscriptionStatus(subID, "none")
		}
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// HandleCreatePortalSession handles POST /call/create-portal-session
func HandleCreatePortalSession(c *gin.Context) {
	ensureStripeKey()
	userID, err := extractUserIDFromAuth(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var stripeCustomerID string
	err = db.HostDB.QueryRow("SELECT stripe_customer_id FROM users WHERE id = ?", userID).Scan(&stripeCustomerID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No Stripe customer found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if stripeCustomerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No Stripe customer found"})
		return
	}

	baseURL := os.Getenv("BASE_URL")
	returnURL := baseURL + "/call/account"

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(stripeCustomerID),
		ReturnURL: stripe.String(returnURL),
	}

	sess, err := portalsession.New(params)
	if err != nil {
		log.Printf("Error creating portal session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create portal session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": sess.URL})
}

// getOrCreateStripeCustomer checks the DB for an existing stripe_customer_id
// for the given user. If none exists, it creates a new Stripe Customer and
// stores the ID in the database.
func getOrCreateStripeCustomer(userID int) (string, error) {
	var existingID sql.NullString
	err := db.HostDB.QueryRow("SELECT stripe_customer_id FROM users WHERE id = ?", userID).Scan(&existingID)
	if err != nil {
		return "", fmt.Errorf("failed to query user: %w", err)
	}

	if existingID.Valid && existingID.String != "" {
		return existingID.String, nil
	}

	// Look up user email for the Stripe customer
	var email string
	err = db.HostDB.QueryRow("SELECT email FROM users WHERE id = ?", userID).Scan(&email)
	if err != nil {
		return "", fmt.Errorf("failed to query user email: %w", err)
	}

	// Create Stripe customer
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"user_id": strconv.Itoa(userID),
		},
	}

	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create Stripe customer: %w", err)
	}

	// Store the Stripe customer ID
	_, err = db.HostDB.Exec("UPDATE users SET stripe_customer_id = ? WHERE id = ?", cust.ID, userID)
	if err != nil {
		return "", fmt.Errorf("failed to update stripe_customer_id: %w", err)
	}

	return cust.ID, nil
}

// handleCheckoutCompleted processes a checkout.session.completed event.
func handleCheckoutCompleted(sess *stripe.CheckoutSession) {
	userIDStr := sess.ClientReferenceID
	if userIDStr == "" {
		log.Printf("checkout.session.completed: missing client_reference_id")
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		log.Printf("checkout.session.completed: invalid client_reference_id %q: %v", userIDStr, err)
		return
	}

	switch sess.Mode {
	case stripe.CheckoutSessionModePayment:
		// Credit pack purchase
		minutesStr := os.Getenv("CREDIT_PACK_MINUTES")
		minutes := 120 // default
		if minutesStr != "" {
			if parsed, err := strconv.Atoi(minutesStr); err == nil {
				minutes = parsed
			}
		}

		_, err := db.HostDB.Exec("UPDATE users SET credit_minutes = credit_minutes + ? WHERE id = ?", minutes, userID)
		if err != nil {
			log.Printf("checkout.session.completed: failed to add credit minutes for user %d: %v", userID, err)
		} else {
			log.Printf("checkout.session.completed: added %d credit minutes for user %d", minutes, userID)
		}

	case stripe.CheckoutSessionModeSubscription:
		// Subscription purchase
		subID := ""
		if sess.Subscription != nil {
			subID = sess.Subscription.ID
		}

		_, err := db.HostDB.Exec(
			"UPDATE users SET subscription_status = 'active', subscription_stripe_id = ? WHERE id = ?",
			subID, userID,
		)
		if err != nil {
			log.Printf("checkout.session.completed: failed to activate subscription for user %d: %v", userID, err)
		} else {
			log.Printf("checkout.session.completed: activated subscription for user %d", userID)
		}
	}
}

// updateSubscriptionStatus updates the subscription_status for a user
// identified by their subscription_stripe_id.
func updateSubscriptionStatus(stripeSubID string, status string) {
	_, err := db.HostDB.Exec(
		"UPDATE users SET subscription_status = ? WHERE subscription_stripe_id = ?",
		status, stripeSubID,
	)
	if err != nil {
		log.Printf("updateSubscriptionStatus: failed to set status=%q for sub=%s: %v", status, stripeSubID, err)
	} else {
		log.Printf("updateSubscriptionStatus: set status=%q for sub=%s", status, stripeSubID)
	}
}
