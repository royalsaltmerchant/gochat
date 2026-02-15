package main

import (
	"net/url"
	"os"
)

// dev
// var relayHost = "99.36.161.96:8000"
// var relayBaseURL = url.URL{Scheme: "http", Host: relayHost, Path: ""}
// var wsRelayURL = url.URL{Scheme: "ws", Host: relayHost, Path: "/ws"}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

var relayHost = envOrDefault("CHAT_RELAY_HOST", "chat.parchchat.com")
var relayScheme = envOrDefault("CHAT_RELAY_SCHEME", "https")
var relayWSScheme = envOrDefault("CHAT_RELAY_WS_SCHEME", "wss")
var relayBaseURL = url.URL{Scheme: relayScheme, Host: relayHost, Path: ""}
var wsRelayURL = url.URL{Scheme: relayWSScheme, Host: relayHost, Path: "/ws"}

// Use a dedicated v2 host DB file so legacy chat DB data cannot conflict with
// the decentralized host schema bootstrap.
var dbName = "host_chat_v2.db"

var configFileName = "host_config.json"

// Official community host/space defaults.
// Override with env vars on the official host deployment.
var officialHostUUID = envOrDefault("OFFICIAL_HOST_UUID", "5837a5c3-5268-45e1-9ea4-ee87d959d067")
var officialSpaceUUID = envOrDefault("OFFICIAL_SPACE_UUID", "parch-community")
var officialSpaceName = envOrDefault("OFFICIAL_SPACE_NAME", "Parch Community")

var officialSpaceChannels = []string{"general", "feedback", "announcements"}

var currentHostUUID string

func isOfficialHostInstance() bool {
	if officialHostUUID == "" {
		return false
	}
	return currentHostUUID == officialHostUUID
}
