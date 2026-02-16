package main

import "testing"

func TestAllowPreAuthMessage(t *testing.T) {
	regularClient := &Client{IsHostAuthor: false}
	authorClient := &Client{IsHostAuthor: true}

	if !allowPreAuthMessage(regularClient, "auth_pubkey") {
		t.Fatalf("expected auth_pubkey to be allowed pre-auth")
	}
	if allowPreAuthMessage(regularClient, "get_dash_data") {
		t.Fatalf("expected regular pre-auth client message to be blocked")
	}
	if !allowPreAuthMessage(authorClient, "get_dash_data_response") {
		t.Fatalf("expected host author passthrough message to be allowed")
	}
	if !allowPreAuthMessage(authorClient, "relay_health_check_ack") {
		t.Fatalf("expected relay_health_check_ack passthrough message to be allowed")
	}
	if allowPreAuthMessage(authorClient, "chat") {
		t.Fatalf("expected chat to require full client authentication")
	}
}
