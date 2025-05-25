package main

import (
	"net/url"
)

var relayHost = "parchchat.com"

var relayBaseURL = url.URL{Scheme: "https", Host: relayHost, Path: ""}
