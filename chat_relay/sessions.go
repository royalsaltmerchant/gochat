package main

import "sync"

var authenticatedSessions = make(map[string]int)
var authenticatedSessionsMu sync.RWMutex

func RegisterAuthenticatedIP(ip string) {
	authenticatedSessionsMu.Lock()
	defer authenticatedSessionsMu.Unlock()
	authenticatedSessions[ip]++
}

func UnregisterAuthenticatedIP(ip string) {
	authenticatedSessionsMu.Lock()
	defer authenticatedSessionsMu.Unlock()
	if count, exists := authenticatedSessions[ip]; exists {
		if count <= 1 {
			delete(authenticatedSessions, ip)
		} else {
			authenticatedSessions[ip]--
		}
	}
}
