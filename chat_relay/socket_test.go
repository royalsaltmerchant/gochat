package main

import (
	"net/http/httptest"
	"testing"
)

func TestIsTrustedHostAuthorJoin(t *testing.T) {
	const authorID = "author-123"

	reqNoOrigin := httptest.NewRequest("GET", "http://example/ws", nil)
	if !isTrustedHostAuthorJoin(reqNoOrigin, authorID, authorID) {
		t.Fatalf("expected host-author join without Origin to be trusted")
	}

	reqBrowser := httptest.NewRequest("GET", "http://example/ws", nil)
	reqBrowser.Header.Set("Origin", "https://chat.parchchat.com")
	if isTrustedHostAuthorJoin(reqBrowser, authorID, authorID) {
		t.Fatalf("expected browser-origin join to be rejected as host-author")
	}

	if isTrustedHostAuthorJoin(reqNoOrigin, "", authorID) {
		t.Fatalf("expected empty author id to be rejected")
	}
	if isTrustedHostAuthorJoin(reqNoOrigin, "wrong", authorID) {
		t.Fatalf("expected mismatched author id to be rejected")
	}
}
