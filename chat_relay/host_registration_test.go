package main

import (
	"bytes"
	"encoding/json"
	"gochat/db"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func setupHostRegistrationDB(t *testing.T) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "host_registration.sqlite")
	hostDB, err := db.InitSQLite(dbPath)
	if err != nil {
		t.Fatalf("init sqlite: %v", err)
	}

	prevHostDB := db.HostDB
	db.HostDB = hostDB

	if err := ensureChatRelaySchema(); err != nil {
		t.Fatalf("ensure relay schema: %v", err)
	}

	t.Cleanup(func() {
		db.HostDB = prevHostDB
		_ = hostDB.Close()
	})
}

func registerHostRequest(t *testing.T, body map[string]string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/api/register_host", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	HandleRegisterHost(c)

	var resp map[string]interface{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	}
	return rec, resp
}

func TestHandleRegisterHostCreatesNewHost(t *testing.T) {
	setupHostRegistrationDB(t)

	signingKey := "test-signing-key"
	rec, resp := registerHostRequest(t, map[string]string{
		"name":               "Host One",
		"signing_public_key": signingKey,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	hostUUID, _ := resp["uuid"].(string)
	if hostUUID == "" {
		t.Fatalf("expected uuid in response")
	}
	if _, err := uuid.Parse(hostUUID); err != nil {
		t.Fatalf("response uuid is invalid: %v", err)
	}

	if got, _ := resp["signing_public_key"].(string); got != signingKey {
		t.Fatalf("expected signing_public_key %q, got %q", signingKey, got)
	}
}

func TestHandleRegisterHostBackfillsLegacySigningKey(t *testing.T) {
	setupHostRegistrationDB(t)

	hostUUID := uuid.NewString()
	if _, err := db.HostDB.Exec(
		`INSERT INTO hosts (uuid, name, signing_public_key, online) VALUES (?, ?, '', 0)`,
		hostUUID, "Legacy Host",
	); err != nil {
		t.Fatalf("insert legacy host: %v", err)
	}

	signingKey := "legacy-upgrade-key"
	rec, resp := registerHostRequest(t, map[string]string{
		"uuid":               hostUUID,
		"name":               "Legacy Host Upgraded",
		"signing_public_key": signingKey,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got, _ := resp["uuid"].(string); got != hostUUID {
		t.Fatalf("expected uuid %q, got %q", hostUUID, got)
	}

	var storedSigningKey string
	if err := db.HostDB.QueryRow(`SELECT signing_public_key FROM hosts WHERE uuid = ?`, hostUUID).Scan(&storedSigningKey); err != nil {
		t.Fatalf("query signing key: %v", err)
	}
	if storedSigningKey != signingKey {
		t.Fatalf("expected signing key %q, got %q", signingKey, storedSigningKey)
	}
}

func TestHandleRegisterHostRejectsMismatchedSigningKey(t *testing.T) {
	setupHostRegistrationDB(t)

	hostUUID := uuid.NewString()
	originalSigningKey := "original-signing-key"
	if _, err := db.HostDB.Exec(
		`INSERT INTO hosts (uuid, name, signing_public_key, online) VALUES (?, ?, ?, 0)`,
		hostUUID, "Existing Host", originalSigningKey,
	); err != nil {
		t.Fatalf("insert existing host: %v", err)
	}

	rec, _ := registerHostRequest(t, map[string]string{
		"uuid":               hostUUID,
		"name":               "Existing Host",
		"signing_public_key": "different-signing-key",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected %d, got %d body=%s", http.StatusConflict, rec.Code, rec.Body.String())
	}

	var storedSigningKey string
	if err := db.HostDB.QueryRow(`SELECT signing_public_key FROM hosts WHERE uuid = ?`, hostUUID).Scan(&storedSigningKey); err != nil {
		t.Fatalf("query signing key: %v", err)
	}
	if storedSigningKey != originalSigningKey {
		t.Fatalf("expected signing key to stay %q, got %q", originalSigningKey, storedSigningKey)
	}
}
