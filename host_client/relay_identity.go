package main

import (
	"fmt"
	"gochat/db"
	"strings"
)

func normalizeUsername(username string, publicKey string) string {
	name := strings.TrimSpace(username)
	if name == "" {
		suffix := publicKey
		if len(suffix) > 8 {
			suffix = suffix[:8]
		}
		name = "user-" + suffix
	}
	if len(name) > 40 {
		name = name[:40]
	}
	return name
}

func upsertHostUserByPublicKey(publicKey string, encPublicKey string, username string) (DashDataUser, error) {
	key := strings.TrimSpace(publicKey)
	encKey := strings.TrimSpace(encPublicKey)
	if key == "" {
		return DashDataUser{}, fmt.Errorf("missing public key")
	}
	if encKey == "" {
		return DashDataUser{}, fmt.Errorf("missing encryption public key")
	}

	trimmedUsername := strings.TrimSpace(username)
	resolvedUsername := normalizeUsername(trimmedUsername, key)

	_, err := db.ChatDB.Exec(
		`INSERT INTO chat_users (public_key, enc_public_key, username)
		 VALUES (?, ?, ?)
		 ON CONFLICT(public_key) DO UPDATE SET
		   enc_public_key = CASE
		     WHEN ? <> '' THEN ?
		     ELSE chat_users.enc_public_key
		   END,
		   username = CASE
		     WHEN ? <> '' THEN ?
		     ELSE chat_users.username
		   END,
		   updated_at = CURRENT_TIMESTAMP`,
		key,
		encKey,
		resolvedUsername,
		encKey,
		encKey,
		trimmedUsername,
		resolvedUsername,
	)
	if err != nil {
		return DashDataUser{}, err
	}

	var user DashDataUser
	err = db.ChatDB.QueryRow(
		`SELECT id, username, public_key, enc_public_key FROM chat_users WHERE public_key = ?`,
		key,
	).Scan(&user.ID, &user.Username, &user.PublicKey, &user.EncPublicKey)
	if err != nil {
		return DashDataUser{}, err
	}
	return user, nil
}

func lookupHostUserByPublicKey(publicKey string) (DashDataUser, error) {
	key := strings.TrimSpace(publicKey)
	if key == "" {
		return DashDataUser{}, fmt.Errorf("missing public key")
	}

	var user DashDataUser
	err := db.ChatDB.QueryRow(
		`SELECT id, username, public_key, enc_public_key FROM chat_users WHERE public_key = ?`,
		key,
	).Scan(&user.ID, &user.Username, &user.PublicKey, &user.EncPublicKey)
	if err != nil {
		return DashDataUser{}, err
	}
	return user, nil
}

func lookupHostUserByID(userID int) (DashDataUser, error) {
	if userID <= 0 {
		return DashDataUser{}, fmt.Errorf("invalid user id")
	}

	var user DashDataUser
	err := db.ChatDB.QueryRow(
		`SELECT id, username, public_key, enc_public_key FROM chat_users WHERE id = ?`,
		userID,
	).Scan(&user.ID, &user.Username, &user.PublicKey, &user.EncPublicKey)
	if err != nil {
		return DashDataUser{}, err
	}
	return user, nil
}

func lookupHostUsersByIDs(userIDs []int) ([]DashDataUser, error) {
	if len(userIDs) == 0 {
		return []DashDataUser{}, nil
	}

	filtered := make([]int, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID > 0 {
			filtered = append(filtered, userID)
		}
	}
	if len(filtered) == 0 {
		return []DashDataUser{}, nil
	}

	placeholders := make([]string, len(filtered))
	args := make([]interface{}, len(filtered))
	for i, userID := range filtered {
		placeholders[i] = "?"
		args[i] = userID
	}

	query := fmt.Sprintf(
		`SELECT id, username, public_key, enc_public_key FROM chat_users WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := db.ChatDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []DashDataUser
	for rows.Next() {
		var user DashDataUser
		if err := rows.Scan(&user.ID, &user.Username, &user.PublicKey, &user.EncPublicKey); err != nil {
			continue
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func resolveHostUserIdentity(userID int, userPublicKey string, userEncPublicKey string, fallbackUsername string) (DashDataUser, error) {
	trimmedPublicKey := strings.TrimSpace(userPublicKey)
	trimmedEncPublicKey := strings.TrimSpace(userEncPublicKey)

	if trimmedPublicKey != "" {
		if trimmedEncPublicKey != "" {
			return upsertHostUserByPublicKey(trimmedPublicKey, trimmedEncPublicKey, fallbackUsername)
		}
		lookedUp, err := lookupHostUserByPublicKey(trimmedPublicKey)
		if err == nil {
			return lookedUp, nil
		}
	}

	if userID > 0 {
		return lookupHostUserByID(userID)
	}

	return DashDataUser{}, fmt.Errorf("missing user identity")
}

func resolveHostUserIdentityStrict(userID int, userPublicKey string, userEncPublicKey string) (DashDataUser, error) {
	return resolveHostUserIdentity(userID, userPublicKey, userEncPublicKey, "")
}

func hydrateInvitePublicKeys(invites []DashDataInvite) []DashDataInvite {
	if len(invites) == 0 {
		return invites
	}

	ids := make([]int, 0, len(invites))
	seen := make(map[int]struct{})
	for _, invite := range invites {
		if invite.UserPublicKey != "" || invite.UserID <= 0 {
			continue
		}
		if _, ok := seen[invite.UserID]; ok {
			continue
		}
		seen[invite.UserID] = struct{}{}
		ids = append(ids, invite.UserID)
	}

	if len(ids) == 0 {
		return invites
	}

	users, err := lookupHostUsersByIDs(ids)
	if err != nil {
		return invites
	}

	keyByID := make(map[int]string, len(users))
	for _, user := range users {
		if user.ID > 0 && user.PublicKey != "" {
			keyByID[user.ID] = user.PublicKey
		}
	}

	for i := range invites {
		if invites[i].UserPublicKey == "" {
			invites[i].UserPublicKey = keyByID[invites[i].UserID]
		}
	}
	return invites
}
