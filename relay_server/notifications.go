package main

import (
	"context"
	"database/sql"
	"fmt"
	"gochat/db"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	emailTypeInvite   = "invite"
	emailTypeActivity = "activity"
	emailTypeWeekly   = "weekly"
	emailTypeAll      = "all"
)

type emailPreference struct {
	UserID          int
	InviteEmails    bool
	ActivityEmails  bool
	WeeklyEmails    bool
	UnsubscribedAll bool
	Token           string
}

func getIntEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getPublicBaseURL() string {
	baseURL := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL"))
	if baseURL != "" {
		return strings.TrimRight(baseURL, "/")
	}
	return "https://parchchat.com"
}

func buildUnsubscribeLinks(pref *emailPreference, notificationType string) (string, string) {
	base := getPublicBaseURL()
	specific := fmt.Sprintf(
		"%s/unsubscribe?token=%s&type=%s",
		base,
		url.QueryEscape(pref.Token),
		url.QueryEscape(notificationType),
	)
	all := fmt.Sprintf(
		"%s/unsubscribe?token=%s&type=%s",
		base,
		url.QueryEscape(pref.Token),
		url.QueryEscape(emailTypeAll),
	)
	return specific, all
}

func ensureEmailPreference(userID int) (*emailPreference, error) {
	token := strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err := db.HostDB.Exec(
		`INSERT OR IGNORE INTO email_preferences (user_id, unsubscribe_token) VALUES (?, ?)`,
		userID, token,
	)
	if err != nil {
		return nil, err
	}

	var pref emailPreference
	var inviteInt, activityInt, weeklyInt, unsubscribedAllInt int
	err = db.HostDB.QueryRow(
		`SELECT user_id, invite_emails, activity_emails, weekly_emails, unsubscribed_all, unsubscribe_token
		 FROM email_preferences
		 WHERE user_id = ?`,
		userID,
	).Scan(
		&pref.UserID,
		&inviteInt,
		&activityInt,
		&weeklyInt,
		&unsubscribedAllInt,
		&pref.Token,
	)
	if err != nil {
		return nil, err
	}

	pref.InviteEmails = inviteInt == 1
	pref.ActivityEmails = activityInt == 1
	pref.WeeklyEmails = weeklyInt == 1
	pref.UnsubscribedAll = unsubscribedAllInt == 1
	return &pref, nil
}

func isNotificationEnabled(userID int, notificationType string) (bool, *emailPreference, error) {
	pref, err := ensureEmailPreference(userID)
	if err != nil {
		return false, nil, err
	}

	if pref.UnsubscribedAll {
		return false, pref, nil
	}

	switch notificationType {
	case emailTypeInvite:
		return pref.InviteEmails, pref, nil
	case emailTypeActivity:
		return pref.ActivityEmails, pref, nil
	case emailTypeWeekly:
		return pref.WeeklyEmails, pref, nil
	default:
		return false, pref, nil
	}
}

func getHostNameByUUID(hostUUID string) (string, error) {
	var hostName string
	err := db.HostDB.QueryRow(`SELECT name FROM hosts WHERE uuid = ?`, hostUUID).Scan(&hostName)
	if err == sql.ErrNoRows {
		return "your host", nil
	}
	return hostName, err
}

func sendSpaceInviteEmail(userID int, recipientEmail string, hostUUID string, spaceName string, inviterUsername string) error {
	enabled, pref, err := isNotificationEnabled(userID, emailTypeInvite)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	hostName, err := getHostNameByUUID(hostUUID)
	if err != nil {
		hostName = "your host"
	}

	if strings.TrimSpace(spaceName) == "" {
		spaceName = "a new space"
	}
	if strings.TrimSpace(inviterUsername) == "" {
		inviterUsername = "A host admin"
	}

	unsubInvite, unsubAll := buildUnsubscribeLinks(pref, emailTypeInvite)
	subject := fmt.Sprintf("Parch: You were invited to %q", spaceName)
	body := fmt.Sprintf(
		"Hi,\n\n%s invited you to join %q on %s.\n\n"+
			"Open Parch Web Client: %s/client\n\n"+
			"If you no longer want invite emails:\n%s\n\n"+
			"Unsubscribe from all Parch emails:\n%s\n",
		inviterUsername,
		spaceName,
		hostName,
		getPublicBaseURL(),
		unsubInvite,
		unsubAll,
	)

	return SendEmail(recipientEmail, subject, body)
}

func UpsertUsersInSpace(hostUUID string, spaceUUID string, userIDs []int) error {
	if strings.TrimSpace(hostUUID) == "" || strings.TrimSpace(spaceUUID) == "" || len(userIDs) == 0 {
		return nil
	}

	uniqueUserIDs := make(map[int]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID > 0 {
			uniqueUserIDs[userID] = struct{}{}
		}
	}
	if len(uniqueUserIDs) == 0 {
		return nil
	}

	tx, err := db.HostDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for userID := range uniqueUserIDs {
		_, err := tx.Exec(
			`INSERT INTO user_space_memberships (user_id, host_uuid, space_uuid, is_active, first_seen_at, last_seen_at)
			 VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			 ON CONFLICT(user_id, host_uuid, space_uuid)
			 DO UPDATE SET
			   is_active = 1,
			   last_seen_at = CURRENT_TIMESTAMP`,
			userID,
			hostUUID,
			spaceUUID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func SyncUserSpaceMemberships(hostUUID string, currentUserID int, spaces []DashDataSpace) error {
	if strings.TrimSpace(hostUUID) == "" || currentUserID <= 0 {
		return nil
	}

	currentSpaceSet := make(map[string]struct{}, len(spaces))
	for _, space := range spaces {
		if strings.TrimSpace(space.UUID) == "" {
			continue
		}
		currentSpaceSet[space.UUID] = struct{}{}

		userIDs := make([]int, 0, len(space.Users))
		for _, user := range space.Users {
			if user.ID > 0 {
				userIDs = append(userIDs, user.ID)
			}
		}
		if err := UpsertUsersInSpace(hostUUID, space.UUID, userIDs); err != nil {
			return err
		}
	}

	args := []interface{}{currentUserID, hostUUID}
	if len(currentSpaceSet) == 0 {
		if _, err := db.HostDB.Exec(
			`UPDATE user_space_memberships
			 SET is_active = 0, last_seen_at = CURRENT_TIMESTAMP
			 WHERE user_id = ? AND host_uuid = ?`,
			args...,
		); err != nil {
			return err
		}
		if _, err := db.HostDB.Exec(
			`DELETE FROM user_space_message_counters
			 WHERE user_id = ? AND host_uuid = ?`,
			args...,
		); err != nil {
			return err
		}
		return nil
	}

	spaceUUIDs := make([]string, 0, len(currentSpaceSet))
	for spaceUUID := range currentSpaceSet {
		spaceUUIDs = append(spaceUUIDs, spaceUUID)
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(spaceUUIDs)), ",")
	updateQuery := fmt.Sprintf(
		`UPDATE user_space_memberships
		 SET is_active = 0, last_seen_at = CURRENT_TIMESTAMP
		 WHERE user_id = ? AND host_uuid = ? AND space_uuid NOT IN (%s)`,
		placeholders,
	)

	updateArgs := make([]interface{}, 0, len(args)+len(spaceUUIDs))
	updateArgs = append(updateArgs, args...)
	for _, spaceUUID := range spaceUUIDs {
		updateArgs = append(updateArgs, spaceUUID)
	}

	if _, err := db.HostDB.Exec(updateQuery, updateArgs...); err != nil {
		return err
	}

	deleteQuery := fmt.Sprintf(
		`DELETE FROM user_space_message_counters
		 WHERE user_id = ? AND host_uuid = ? AND space_uuid NOT IN (%s)`,
		placeholders,
	)
	if _, err := db.HostDB.Exec(deleteQuery, updateArgs...); err != nil {
		return err
	}

	return nil
}

func MarkUserLeftSpace(hostUUID string, spaceUUID string, userID int) error {
	if strings.TrimSpace(hostUUID) == "" || strings.TrimSpace(spaceUUID) == "" || userID <= 0 {
		return nil
	}

	tx, err := db.HostDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE user_space_memberships
		 SET is_active = 0, last_seen_at = CURRENT_TIMESTAMP
		 WHERE user_id = ? AND host_uuid = ? AND space_uuid = ?`,
		userID, hostUUID, spaceUUID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`DELETE FROM user_space_message_counters
		 WHERE user_id = ? AND host_uuid = ? AND space_uuid = ?`,
		userID, hostUUID, spaceUUID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func TrackOfflineMessageActivity(hostUUID string, spaceUUID string, senderUserID int, connectedUserIDs map[int]struct{}) error {
	if strings.TrimSpace(hostUUID) == "" || strings.TrimSpace(spaceUUID) == "" {
		return nil
	}

	rows, err := db.HostDB.Query(
		`SELECT user_id
		 FROM user_space_memberships
		 WHERE host_uuid = ?
		   AND space_uuid = ?
		   AND is_active = 1`,
		hostUUID,
		spaceUUID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var userID int
		if err := rows.Scan(&userID); err != nil {
			continue
		}
		if userID == senderUserID {
			continue
		}
		if _, isConnected := connectedUserIDs[userID]; isConnected {
			continue
		}

		_, err := db.HostDB.Exec(
			`INSERT INTO user_space_message_counters (user_id, host_uuid, space_uuid, pending_count, last_message_at)
			 VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP)
			 ON CONFLICT(user_id, host_uuid, space_uuid)
			 DO UPDATE SET
			   pending_count = pending_count + 1,
			   last_message_at = CURRENT_TIMESTAMP`,
			userID,
			hostUUID,
			spaceUUID,
		)
		if err != nil {
			log.Printf("message counter update failed for user %d host %s space %s: %v", userID, hostUUID, spaceUUID, err)
		}
	}

	return rows.Err()
}

func processActivityEmailBatch() error {
	threshold := getIntEnv("EMAIL_ACTIVITY_THRESHOLD", 20)
	cooldownMinutes := getIntEnv("EMAIL_ACTIVITY_COOLDOWN_MINUTES", 720)
	cooldownExpr := fmt.Sprintf("-%d minutes", cooldownMinutes)

	rows, err := db.HostDB.Query(
		`SELECT
		    c.user_id,
		    c.host_uuid,
		    SUM(c.pending_count) AS total_pending,
		    COUNT(CASE WHEN c.pending_count > 0 THEN 1 END) AS active_spaces,
		    u.email,
		    COALESCE(h.name, 'your host')
		 FROM user_space_message_counters c
		 JOIN users u ON u.id = c.user_id
		 JOIN email_preferences p ON p.user_id = c.user_id
		 LEFT JOIN hosts h ON h.uuid = c.host_uuid
		 WHERE c.pending_count > 0
		   AND p.unsubscribed_all = 0
		   AND p.activity_emails = 1
		   AND (c.last_emailed_at IS NULL OR datetime(c.last_emailed_at) <= datetime('now', ?))
		 GROUP BY c.user_id, c.host_uuid, u.email, h.name
		 HAVING SUM(c.pending_count) >= ?
		 ORDER BY total_pending DESC
		 LIMIT 500`,
		cooldownExpr,
		threshold,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var userID int
		var hostUUID, email, hostName string
		var pendingCount int
		var activeSpaces int
		if err := rows.Scan(&userID, &hostUUID, &pendingCount, &activeSpaces, &email, &hostName); err != nil {
			continue
		}

		pref, err := ensureEmailPreference(userID)
		if err != nil {
			continue
		}
		unsubActivity, unsubAll := buildUnsubscribeLinks(pref, emailTypeActivity)

		subject := fmt.Sprintf("Parch: %d new messages on %s", pendingCount, hostName)
		body := fmt.Sprintf(
			"Hi,\n\nThere has been a lot of recent activity in %d of your spaces on %s.\n"+
				"You have about %d new messages waiting.\n\n"+
				"Open Parch Web Client: %s/client\n\n"+
				"Stop activity emails:\n%s\n\n"+
				"Unsubscribe from all Parch emails:\n%s\n",
			activeSpaces,
			hostName,
			pendingCount,
			getPublicBaseURL(),
			unsubActivity,
			unsubAll,
		)

		if err := SendEmail(email, subject, body); err != nil {
			log.Printf("activity email send failed user %d: %v", userID, err)
			continue
		}

		_, err = db.HostDB.Exec(
			`UPDATE user_space_message_counters
			 SET pending_count = 0, last_emailed_at = CURRENT_TIMESTAMP
			 WHERE user_id = ? AND host_uuid = ?`,
			userID,
			hostUUID,
		)
		if err != nil {
			log.Printf("activity counter reset failed user %d host %s: %v", userID, hostUUID, err)
		}
	}

	return rows.Err()
}

func processWeeklyEmailBatch() error {
	intervalDays := getIntEnv("EMAIL_WEEKLY_INTERVAL_DAYS", 7)
	intervalExpr := fmt.Sprintf("-%d days", intervalDays)
	feedbackEmail := strings.TrimSpace(os.Getenv("FEEDBACK_EMAIL"))
	if feedbackEmail == "" {
		feedbackEmail = strings.TrimSpace(os.Getenv("EMAIL"))
	}

	rows, err := db.HostDB.Query(
		`SELECT
		    u.id,
		    u.email,
		    u.username
		 FROM users u
		 JOIN email_preferences p ON p.user_id = u.id
		 WHERE p.unsubscribed_all = 0
		   AND p.weekly_emails = 1
		   AND (p.last_weekly_sent_at IS NULL OR datetime(p.last_weekly_sent_at) <= datetime('now', ?))
		 LIMIT 1000`,
		intervalExpr,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var userID int
		var email, username string
		if err := rows.Scan(&userID, &email, &username); err != nil {
			continue
		}

		pref, err := ensureEmailPreference(userID)
		if err != nil {
			continue
		}
		unsubWeekly, unsubAll := buildUnsubscribeLinks(pref, emailTypeWeekly)

		subject := "Parch Weekly: updates, tips, and feedback"
		body := fmt.Sprintf(
			"Hi %s,\n\n"+
				"Thanks for using Parch.\n\n"+
				"What's new:\n"+
				"- Web Client is available: %s/client\n"+
				"- On-demand voice/video is available: %s/call\n\n"+
				"We'd love your feedback: %s\n\n"+
				"Stop weekly emails:\n%s\n\n"+
				"Unsubscribe from all Parch emails:\n%s\n",
			username,
			getPublicBaseURL(),
			getPublicBaseURL(),
			feedbackEmail,
			unsubWeekly,
			unsubAll,
		)

		if err := SendEmail(email, subject, body); err != nil {
			log.Printf("weekly email send failed user %d: %v", userID, err)
			continue
		}

		_, err = db.HostDB.Exec(
			`UPDATE email_preferences
			 SET last_weekly_sent_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			 WHERE user_id = ?`,
			userID,
		)
		if err != nil {
			log.Printf("weekly email update failed user %d: %v", userID, err)
		}
	}

	return rows.Err()
}

func StartEmailNotificationScheduler(ctx context.Context) {
	activityInterval := time.Duration(getIntEnv("EMAIL_ACTIVITY_CHECK_MINUTES", 10)) * time.Minute
	weeklyInterval := time.Duration(getIntEnv("EMAIL_WEEKLY_CHECK_HOURS", 24)) * time.Hour

	activityTicker := time.NewTicker(activityInterval)
	weeklyTicker := time.NewTicker(weeklyInterval)
	defer activityTicker.Stop()
	defer weeklyTicker.Stop()

	log.Printf("Email scheduler started (activity every %s, weekly check every %s)", activityInterval, weeklyInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Email scheduler stopped")
			return
		case <-activityTicker.C:
			if err := processActivityEmailBatch(); err != nil {
				log.Printf("activity email batch failed: %v", err)
			}
		case <-weeklyTicker.C:
			if err := processWeeklyEmailBatch(); err != nil {
				log.Printf("weekly email batch failed: %v", err)
			}
		}
	}
}

func unsubscribeHTML(title string, body string) string {
	return fmt.Sprintf(
		`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title></head><body style="font-family:Arial,sans-serif;max-width:680px;margin:40px auto;padding:0 16px;line-height:1.5;"><h1>%s</h1><p>%s</p><p><a href="%s">Return to Parch</a></p></body></html>`,
		title,
		title,
		body,
		getPublicBaseURL(),
	)
}

func HandleUnsubscribe(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	notificationType := strings.TrimSpace(strings.ToLower(c.Query("type")))
	if notificationType == "" {
		notificationType = emailTypeAll
	}

	if token == "" {
		c.Data(400, "text/html; charset=utf-8", []byte(unsubscribeHTML("Invalid unsubscribe link", "This unsubscribe link is missing a token.")))
		return
	}

	var query string
	switch notificationType {
	case emailTypeAll:
		query = `UPDATE email_preferences
		         SET invite_emails = 0,
		             activity_emails = 0,
		             weekly_emails = 0,
		             unsubscribed_all = 1,
		             updated_at = CURRENT_TIMESTAMP
		         WHERE unsubscribe_token = ?`
	case emailTypeInvite:
		query = `UPDATE email_preferences
		         SET invite_emails = 0,
		             updated_at = CURRENT_TIMESTAMP
		         WHERE unsubscribe_token = ?`
	case emailTypeActivity:
		query = `UPDATE email_preferences
		         SET activity_emails = 0,
		             updated_at = CURRENT_TIMESTAMP
		         WHERE unsubscribe_token = ?`
	case emailTypeWeekly:
		query = `UPDATE email_preferences
		         SET weekly_emails = 0,
		             updated_at = CURRENT_TIMESTAMP
		         WHERE unsubscribe_token = ?`
	default:
		c.Data(400, "text/html; charset=utf-8", []byte(unsubscribeHTML("Invalid unsubscribe type", "The unsubscribe type in this link is not supported.")))
		return
	}

	res, err := db.HostDB.Exec(query, token)
	if err != nil {
		c.Data(500, "text/html; charset=utf-8", []byte(unsubscribeHTML("Unsubscribe failed", "We couldn't update your preferences right now.")))
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.Data(404, "text/html; charset=utf-8", []byte(unsubscribeHTML("Link not found", "This unsubscribe link is invalid or has already been used.")))
		return
	}

	message := "You have been unsubscribed successfully."
	switch notificationType {
	case emailTypeInvite:
		message = "You will no longer receive invite emails."
	case emailTypeActivity:
		message = "You will no longer receive activity emails."
	case emailTypeWeekly:
		message = "You will no longer receive weekly update emails."
	case emailTypeAll:
		message = "You have been unsubscribed from all Parch emails."
	}

	c.Data(200, "text/html; charset=utf-8", []byte(unsubscribeHTML("Unsubscribed", message)))
}
