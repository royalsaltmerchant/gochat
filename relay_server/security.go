package main

import (
	"net/url"
	"strings"
	"sync"
)

var (
	websocketOriginsMu      sync.RWMutex
	allowedWebSocketOrigins = map[string]struct{}{}
)

func defaultAllowedOrigins() []string {
	return []string{
		"https://parchchat.com",
		"https://www.parchchat.com",
		"https://chat.parchchat.com",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:8000",
		"http://127.0.0.1:8000",
	}
}

func parseAllowedOriginsFromEnv(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return defaultAllowedOrigins()
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		out = append(out, origin)
	}
	if len(out) == 0 {
		return defaultAllowedOrigins()
	}
	return out
}

func normalizeOrigin(origin string) string {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Scheme + "://" + u.Host)
}

func setAllowedWebSocketOrigins(origins []string) {
	next := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		normalized := normalizeOrigin(origin)
		if normalized == "" {
			continue
		}
		next[normalized] = struct{}{}
	}
	websocketOriginsMu.Lock()
	allowedWebSocketOrigins = next
	websocketOriginsMu.Unlock()
}

func isWebSocketOriginAllowed(origin string) bool {
	normalized := normalizeOrigin(origin)
	if normalized == "" {
		return false
	}
	websocketOriginsMu.RLock()
	_, ok := allowedWebSocketOrigins[normalized]
	websocketOriginsMu.RUnlock()
	return ok
}
