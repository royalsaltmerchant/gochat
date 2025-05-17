package main

import (
	"net/url"
)

var relayHost = "localhost:8000"

var relayBaseURL = url.URL{Scheme: "http", Host: relayHost, Path: ""}

var wsRelayURL = url.URL{Scheme: "ws", Host: relayHost, Path: "/ws"}
