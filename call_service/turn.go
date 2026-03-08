package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"time"
)

func generateTurnCredentials(secret string, ttlSeconds int64) (username string, password string) {
	unixTime := time.Now().Unix() + ttlSeconds
	username = fmt.Sprintf("%d", unixTime)

	h := hmac.New(sha1.New, []byte(secret))
	h.Write([]byte(username))
	password = base64.StdEncoding.EncodeToString(h.Sum(nil))

	return username, password
}
