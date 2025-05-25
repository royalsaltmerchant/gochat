package main

import (
	"net/url"
)

var relayHost = "parch.julianranieri.com"

var relayBaseURL = url.URL{Scheme: "https", Host: relayHost, Path: ""}
