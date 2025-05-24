package main

import (
	"net/url"
)

var relayHost = "parch.julianranieri.com"

var relayBaseURL = url.URL{Scheme: "https", Host: relayHost, Path: ""}

var wsRelayURL = url.URL{Scheme: "wss", Host: relayHost, Path: "/ws"}

var dbName = "chat.db"

var configFileName = "host_config.json"
