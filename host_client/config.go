package main

import (
	"net/url"
)

var relayHost = "parchchat.com"

// var relayHost = "99.36.161.96:8000"

var relayBaseURL = url.URL{Scheme: "https", Host: relayHost, Path: ""}

var wsRelayURL = url.URL{Scheme: "wss", Host: relayHost, Path: "/ws"}

var dbName = "chat.db"

var configFileName = "host_config.json"
