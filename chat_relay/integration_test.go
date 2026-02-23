package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"gochat/db"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const testReadTimeout = 3 * time.Second

type relayIntegrationEnv struct {
	hostUUID          string
	server            *httptest.Server
	signingPrivateKey ed25519.PrivateKey
}

type authorPeer struct {
	t       *testing.T
	conn    *websocket.Conn
	inbox   chan WSMessage
	writeMu sync.Mutex
	stop    chan struct{}
	done    chan struct{}
	closeMu sync.Mutex
	closed  bool
}

func newRelayIntegrationEnv(t *testing.T) *relayIntegrationEnv {
	t.Helper()
	return newRelayIntegrationEnvWithSigningKey(t, true)
}

func newRelayIntegrationEnvWithSigningKey(t *testing.T, withSigningKey bool) *relayIntegrationEnv {
	t.Helper()

	gin.SetMode(gin.TestMode)
	tempDir, err := os.MkdirTemp("", "relay-integration-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(tempDir, "relay_integration.sqlite")
	hostDB, err := db.InitSQLite(dbPath)
	if err != nil {
		t.Fatalf("init sqlite: %v", err)
	}

	prevHostDB := db.HostDB
	db.HostDB = hostDB
	if err := ensureChatRelaySchema(); err != nil {
		t.Fatalf("ensure relay schema: %v", err)
	}

	hostUUID := uuid.NewString()
	signingPublicKey := ""
	var signingPrivateKey ed25519.PrivateKey
	if withSigningKey {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("generate host signing key: %v", err)
		}
		signingPublicKey = base64.RawStdEncoding.EncodeToString(pub)
		signingPrivateKey = priv
	}
	if _, err := db.HostDB.Exec(
		`INSERT INTO hosts (uuid, name, signing_public_key, online) VALUES (?, ?, ?, 1)`,
		hostUUID,
		"Integration Host",
		signingPublicKey,
	); err != nil {
		t.Fatalf("insert test host: %v", err)
	}

	hostsMu.Lock()
	prevHosts := Hosts
	Hosts = map[string]*Host{}
	hostsMu.Unlock()

	hostHealthChecksMu.Lock()
	prevHealthChecks := hostHealthChecks
	hostHealthChecks = make(map[string]chan struct{})
	hostHealthChecksMu.Unlock()

	authenticatedSessionsMu.Lock()
	prevSessions := authenticatedSessions
	authenticatedSessions = make(map[string]int)
	authenticatedSessionsMu.Unlock()

	chatRateMu.Lock()
	prevChatRates := chatRateByClient
	chatRateByClient = make(map[string][]time.Time)
	chatRateMu.Unlock()

	r := gin.New()
	r.GET("/ws", HandleSocket)
	server := httptest.NewServer(r)

	t.Cleanup(func() {
		server.CloseClientConnections()

		hostsMu.Lock()
		for _, host := range Hosts {
			if host == nil {
				continue
			}
			host.mu.Lock()
			for conn := range host.ClientsByConn {
				_ = conn.Close()
			}
			host.mu.Unlock()
		}
		hostsMu.Unlock()

		server.Close()
		time.Sleep(50 * time.Millisecond)

		hostsMu.Lock()
		Hosts = prevHosts
		hostsMu.Unlock()

		hostHealthChecksMu.Lock()
		hostHealthChecks = prevHealthChecks
		hostHealthChecksMu.Unlock()

		authenticatedSessionsMu.Lock()
		authenticatedSessions = prevSessions
		authenticatedSessionsMu.Unlock()

		chatRateMu.Lock()
		chatRateByClient = prevChatRates
		chatRateMu.Unlock()

		db.HostDB = prevHostDB
		_ = hostDB.Close()

		removeAllWithRetry(tempDir, 5, 100*time.Millisecond)
	})

	return &relayIntegrationEnv{
		hostUUID:          hostUUID,
		server:            server,
		signingPrivateKey: signingPrivateKey,
	}
}

func (e *relayIntegrationEnv) mustIssueCapabilityToken(t *testing.T, subjectPublicKey, spaceUUID string, scopes []string, ttl time.Duration) string {
	t.Helper()
	if len(e.signingPrivateKey) == 0 {
		t.Fatal("test env has no signing private key")
	}
	now := time.Now().UTC()
	claims := SpaceCapabilityClaims{
		Version:      1,
		HostUUID:     e.hostUUID,
		SpaceUUID:    spaceUUID,
		SubjectKey:   subjectPublicKey,
		Scopes:       scopes,
		ExpiresAt:    now.Add(ttl).Unix(),
		IssuedAt:     now.Unix(),
		TokenID:      uuid.NewString(),
		ChannelScope: "*",
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal capability claims: %v", err)
	}
	signature := ed25519.Sign(e.signingPrivateKey, payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (e *relayIntegrationEnv) wsURL() string {
	return "ws" + strings.TrimPrefix(e.server.URL, "http") + "/ws"
}

func (e *relayIntegrationEnv) dialWS(t *testing.T) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(e.wsURL(), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
}

func (e *relayIntegrationEnv) joinHost(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	mustWriteMessage(t, conn, WSMessage{
		Type: "join_host",
		Data: JoinHost{
			UUID: e.hostUUID,
		},
	})

	mustReadType(t, conn, "join_ack", testReadTimeout)
	challengeMsg := mustReadType(t, conn, "auth_challenge", testReadTimeout)
	challenge, err := decodeData[AuthChallenge](challengeMsg.Data)
	if err != nil {
		t.Fatalf("decode auth challenge: %v", err)
	}
	if challenge.Challenge == "" {
		t.Fatal("expected non-empty auth challenge")
	}
	return challenge.Challenge
}

func (e *relayIntegrationEnv) joinHostAsAuthor(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	mustWriteMessage(t, conn, WSMessage{
		Type: "join_host",
		Data: JoinHost{
			UUID: e.hostUUID,
			Role: "host",
		},
	})
	mustReadType(t, conn, "join_ack", testReadTimeout)
	mustReadType(t, conn, "auth_challenge", testReadTimeout)
	hostChallengeMsg := mustReadType(t, conn, "host_auth_challenge", testReadTimeout)
	hostChallenge, err := decodeData[HostAuthChallenge](hostChallengeMsg.Data)
	if err != nil {
		t.Fatalf("decode host auth challenge: %v", err)
	}
	if hostChallenge.Challenge == "" {
		t.Fatal("expected non-empty host auth challenge")
	}
	signature := ed25519.Sign(e.signingPrivateKey, []byte(hostAuthMessage(e.hostUUID, hostChallenge.Challenge)))
	mustWriteMessage(t, conn, WSMessage{
		Type: "host_auth",
		Data: HostAuthClient{
			Challenge: hostChallenge.Challenge,
			Signature: base64.RawStdEncoding.EncodeToString(signature),
		},
	})
	mustReadType(t, conn, "host_auth_success", testReadTimeout)
}

func (e *relayIntegrationEnv) connectAuthor(t *testing.T) *authorPeer {
	t.Helper()
	conn := e.dialWS(t)
	e.joinHostAsAuthor(t, conn)

	peer := &authorPeer{
		t:     t,
		conn:  conn,
		inbox: make(chan WSMessage, 64),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	peer.start()
	t.Cleanup(func() {
		peer.close()
	})
	return peer
}

func (a *authorPeer) start() {
	go func() {
		defer close(a.done)
		defer close(a.inbox)
		for {
			msg, err := readOneMessage(a.conn, testReadTimeout)
			if err != nil {
				return
			}

			if msg.Type == "relay_health_check" {
				health, err := decodeData[RelayHealthCheck](msg.Data)
				if err == nil && health.Nonce != "" {
					a.mustSend(WSMessage{
						Type: "relay_health_check_ack",
						Data: RelayHealthCheckAck{Nonce: health.Nonce},
					})
				}
				continue
			}

			select {
			case <-a.stop:
				return
			case a.inbox <- msg:
			}
		}
	}()
}

func (a *authorPeer) close() {
	a.closeMu.Lock()
	if a.closed {
		a.closeMu.Unlock()
		return
	}
	a.closed = true
	close(a.stop)
	_ = a.conn.Close()
	a.closeMu.Unlock()

	select {
	case <-a.done:
	case <-time.After(2 * time.Second):
	}
}

func (a *authorPeer) mustSend(msg WSMessage) {
	a.t.Helper()
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if err := a.conn.SetWriteDeadline(time.Now().Add(testReadTimeout)); err != nil {
		a.t.Fatalf("set write deadline: %v", err)
	}
	if err := a.conn.WriteJSON(msg); err != nil {
		a.t.Fatalf("author write json: %v", err)
	}
}

func (a *authorPeer) mustNextType(msgType string) WSMessage {
	a.t.Helper()
	timer := time.NewTimer(testReadTimeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			a.t.Fatalf("timed out waiting for author message type %q", msgType)
		case msg, ok := <-a.inbox:
			if !ok {
				a.t.Fatalf("author inbox closed while waiting for %q", msgType)
			}
			if msg.Type == msgType {
				return msg
			}
		}
	}
}

func mustWriteMessage(t *testing.T, conn *websocket.Conn, msg WSMessage) {
	t.Helper()
	if err := conn.SetWriteDeadline(time.Now().Add(testReadTimeout)); err != nil {
		t.Fatalf("set write deadline: %v", err)
	}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func readOneMessage(conn *websocket.Conn, timeout time.Duration) (WSMessage, error) {
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return WSMessage{}, err
	}
	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		return WSMessage{}, err
	}
	return msg, nil
}

func mustReadType(t *testing.T, conn *websocket.Conn, msgType string, timeout time.Duration) WSMessage {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("timed out waiting for message type %q", msgType)
		}
		msg, err := readOneMessage(conn, remaining)
		if err != nil {
			t.Fatalf("read message: %v", err)
		}
		if msg.Type == msgType {
			return msg
		}
	}
}

func removeAllWithRetry(path string, attempts int, delay time.Duration) {
	if path == "" {
		return
	}
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		if err := os.RemoveAll(path); err == nil {
			return
		}
		time.Sleep(delay)
	}
}

func mustReadUnauthorizedError(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	msg := mustReadType(t, conn, "error", testReadTimeout)
	chatErr, err := decodeData[ChatError](msg.Data)
	if err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if !strings.Contains(strings.ToLower(chatErr.Content), "unauthorized") {
		t.Fatalf("expected unauthorized error, got: %q", chatErr.Content)
	}
	return chatErr.Content
}

func authenticateClient(t *testing.T, env *relayIntegrationEnv, conn *websocket.Conn, challenge string, username string) AuthPubKeySuccess {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	publicKey := base64.RawStdEncoding.EncodeToString(pub)
	encPublicKey := base64.RawStdEncoding.EncodeToString([]byte("enc-" + publicKey[:8]))
	sig := ed25519.Sign(priv, []byte(authMessage(env.hostUUID, challenge, encPublicKey)))
	signature := base64.RawStdEncoding.EncodeToString(sig)

	mustWriteMessage(t, conn, WSMessage{
		Type: "auth_pubkey",
		Data: AuthPubKeyClient{
			PublicKey:    publicKey,
			EncPublicKey: encPublicKey,
			Username:     username,
			Challenge:    challenge,
			Signature:    signature,
		},
	})

	successMsg := mustReadType(t, conn, "auth_pubkey_success", testReadTimeout)
	success, err := decodeData[AuthPubKeySuccess](successMsg.Data)
	if err != nil {
		t.Fatalf("decode auth success: %v", err)
	}
	return success
}

func TestRelayIntegrationAuthenticationFlow(t *testing.T) {
	env := newRelayIntegrationEnv(t)
	_ = env.connectAuthor(t)

	client := env.dialWS(t)
	defer client.Close()
	challenge := env.joinHost(t, client)

	mustWriteMessage(t, client, WSMessage{Type: "get_dash_data", Data: map[string]interface{}{}})
	preAuthMsg := mustReadType(t, client, "authentication-error", testReadTimeout)
	preAuthErr, err := decodeData[ChatError](preAuthMsg.Data)
	if err != nil {
		t.Fatalf("decode pre-auth error: %v", err)
	}
	if !strings.Contains(preAuthErr.Content, "Authentication required") {
		t.Fatalf("expected authentication required error, got: %q", preAuthErr.Content)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	publicKey := base64.RawStdEncoding.EncodeToString(pub)
	encPublicKey := "enc-test-auth"
	invalidSignature := base64.RawStdEncoding.EncodeToString(
		ed25519.Sign(priv, []byte(authMessage(env.hostUUID, challenge+"-wrong", encPublicKey))),
	)

	mustWriteMessage(t, client, WSMessage{
		Type: "auth_pubkey",
		Data: AuthPubKeyClient{
			PublicKey:    publicKey,
			EncPublicKey: encPublicKey,
			Username:     "alice",
			Challenge:    challenge,
			Signature:    invalidSignature,
		},
	})
	invalidAuthMsg := mustReadType(t, client, "authentication-error", testReadTimeout)
	invalidAuthErr, err := decodeData[ChatError](invalidAuthMsg.Data)
	if err != nil {
		t.Fatalf("decode invalid auth error: %v", err)
	}
	if !strings.Contains(invalidAuthErr.Content, "Invalid signature") {
		t.Fatalf("expected invalid signature error, got: %q", invalidAuthErr.Content)
	}

	validSignature := base64.RawStdEncoding.EncodeToString(
		ed25519.Sign(priv, []byte(authMessage(env.hostUUID, challenge, encPublicKey))),
	)
	mustWriteMessage(t, client, WSMessage{
		Type: "auth_pubkey",
		Data: AuthPubKeyClient{
			PublicKey:    publicKey,
			EncPublicKey: encPublicKey,
			Username:     "alice",
			Challenge:    challenge,
			Signature:    validSignature,
		},
	})

	successMsg := mustReadType(t, client, "auth_pubkey_success", testReadTimeout)
	success, err := decodeData[AuthPubKeySuccess](successMsg.Data)
	if err != nil {
		t.Fatalf("decode auth success: %v", err)
	}
	if success.PublicKey != publicKey {
		t.Fatalf("expected auth success public key %q, got %q", publicKey, success.PublicKey)
	}
}

func TestRelayIntegrationGetDashDataRoundTrip(t *testing.T) {
	env := newRelayIntegrationEnv(t)
	author := env.connectAuthor(t)

	client := env.dialWS(t)
	defer client.Close()
	challenge := env.joinHost(t, client)
	auth := authenticateClient(t, env, client, challenge, "bob")

	mustWriteMessage(t, client, WSMessage{Type: "get_dash_data", Data: map[string]interface{}{}})
	requestMsg := author.mustNextType("get_dash_data_request")
	request, err := decodeData[GetDashDataRequest](requestMsg.Data)
	if err != nil {
		t.Fatalf("decode get_dash_data_request: %v", err)
	}

	spaceUUID := uuid.NewString()
	channelUUID := uuid.NewString()
	author.mustSend(WSMessage{
		Type: "get_dash_data_response",
		Data: GetDashDataResponse{
			User: DashDataUser{
				ID:           request.UserID,
				Username:     auth.Username,
				PublicKey:    request.UserPublicKey,
				EncPublicKey: request.UserEncPublicKey,
			},
			Spaces: []DashDataSpace{
				{
					ID:       1,
					UUID:     spaceUUID,
					Name:     "Engineering",
					AuthorID: request.UserID,
					Channels: []DashDataChannel{{
						ID:        1,
						UUID:      channelUUID,
						Name:      "general",
						SpaceUUID: spaceUUID,
					}},
				},
			},
			Invites:    nil,
			ClientUUID: request.ClientUUID,
		},
	})

	dashMsg := mustReadType(t, client, "dash_data_payload", testReadTimeout)
	dash, err := decodeData[GetDashDataSuccess](dashMsg.Data)
	if err != nil {
		t.Fatalf("decode dash_data_payload: %v", err)
	}

	if len(dash.Spaces) != 1 {
		t.Fatalf("expected 1 space, got %d", len(dash.Spaces))
	}
	if dash.Spaces[0].UUID != spaceUUID {
		t.Fatalf("expected space %q, got %q", spaceUUID, dash.Spaces[0].UUID)
	}
	if len(dash.Spaces[0].Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(dash.Spaces[0].Channels))
	}
	if dash.Spaces[0].Channels[0].UUID != channelUUID {
		t.Fatalf("expected channel %q, got %q", channelUUID, dash.Spaces[0].Channels[0].UUID)
	}
}

func TestRelayIntegrationChannelJoinRequiresSpaceMembership(t *testing.T) {
	env := newRelayIntegrationEnv(t)
	author := env.connectAuthor(t)

	client := env.dialWS(t)
	defer client.Close()
	challenge := env.joinHost(t, client)
	auth := authenticateClient(t, env, client, challenge, "carol")

	mustWriteMessage(t, client, WSMessage{Type: "get_dash_data", Data: map[string]interface{}{}})
	requestMsg := author.mustNextType("get_dash_data_request")
	request, err := decodeData[GetDashDataRequest](requestMsg.Data)
	if err != nil {
		t.Fatalf("decode get_dash_data_request: %v", err)
	}

	spaceUUID := uuid.NewString()
	channelUUID := uuid.NewString()
	capabilityToken := env.mustIssueCapabilityToken(
		t,
		request.UserPublicKey,
		spaceUUID,
		[]string{scopeJoinChannel, scopeSendMessage, scopeReadHistory},
		5*time.Minute,
	)
	author.mustSend(WSMessage{
		Type: "get_dash_data_response",
		Data: GetDashDataResponse{
			User: DashDataUser{
				ID:           request.UserID,
				Username:     auth.Username,
				PublicKey:    request.UserPublicKey,
				EncPublicKey: request.UserEncPublicKey,
			},
			Spaces: []DashDataSpace{{
				ID:       1,
				UUID:     spaceUUID,
				Name:     "Ops",
				AuthorID: request.UserID,
				Channels: []DashDataChannel{{
					ID:        1,
					UUID:      channelUUID,
					Name:      "alerts",
					SpaceUUID: spaceUUID,
				}},
			}},
			Capabilities: []SpaceCapability{{
				SpaceUUID: spaceUUID,
				Token:     capabilityToken,
				Scopes:    []string{scopeJoinChannel, scopeSendMessage, scopeReadHistory},
				ExpiresAt: time.Now().UTC().Add(5 * time.Minute).Unix(),
			}},
			ClientUUID: request.ClientUUID,
		},
	})
	mustReadType(t, client, "dash_data_payload", testReadTimeout)

	mustWriteMessage(t, client, WSMessage{Type: "join_channel", Data: JoinUUID{UUID: channelUUID}})
	errorMsg := mustReadType(t, client, "error", testReadTimeout)
	joinErr, err := decodeData[ChatError](errorMsg.Data)
	if err != nil {
		t.Fatalf("decode join channel error: %v", err)
	}
	if !strings.Contains(joinErr.Content, "Unauthorized channel join") {
		t.Fatalf("expected unauthorized channel join error, got: %q", joinErr.Content)
	}

	mustWriteMessage(t, client, WSMessage{
		Type: "join_all_spaces",
		Data: JoinAllSpacesClient{
			SpaceUUIDs:       []string{spaceUUID},
			CapabilityTokens: map[string]string{spaceUUID: capabilityToken},
		},
	})
	mustReadType(t, client, "join_all_spaces_success", testReadTimeout)

	mustWriteMessage(t, client, WSMessage{
		Type: "join_channel",
		Data: JoinUUID{
			UUID:            channelUUID,
			CapabilityToken: capabilityToken,
		},
	})
	joinedMsg := mustReadType(t, client, "joined_channel", testReadTimeout)
	if joinedMsg.Type != "joined_channel" {
		t.Fatalf("expected joined_channel response, got %q", joinedMsg.Type)
	}
}

func TestRelayIntegrationAcceptInviteEnablesImmediateChannelJoin(t *testing.T) {
	env := newRelayIntegrationEnv(t)
	author := env.connectAuthor(t)

	client := env.dialWS(t)
	defer client.Close()
	challenge := env.joinHost(t, client)
	auth := authenticateClient(t, env, client, challenge, "dave")

	mustWriteMessage(t, client, WSMessage{Type: "accept_invite", Data: AcceptInviteClient{SpaceUserID: 77}})
	requestMsg := author.mustNextType("accept_invite_request")
	request, err := decodeData[AcceptInviteRequest](requestMsg.Data)
	if err != nil {
		t.Fatalf("decode accept_invite_request: %v", err)
	}

	spaceUUID := uuid.NewString()
	channelUUID := uuid.NewString()
	capabilityToken := env.mustIssueCapabilityToken(
		t,
		request.UserPublicKey,
		spaceUUID,
		[]string{scopeJoinChannel, scopeSendMessage, scopeReadHistory},
		5*time.Minute,
	)
	author.mustSend(WSMessage{
		Type: "accept_invite_success",
		Data: AcceptInviteResponse{
			SpaceUserID: request.SpaceUserID,
			User: DashDataUser{
				ID:           request.UserID,
				Username:     auth.Username,
				PublicKey:    request.UserPublicKey,
				EncPublicKey: request.UserEncPublicKey,
			},
			Space: DashDataSpace{
				ID:       1,
				UUID:     spaceUUID,
				Name:     "Invited",
				AuthorID: 999,
				Channels: []DashDataChannel{{
					ID:        1,
					UUID:      channelUUID,
					Name:      "private",
					SpaceUUID: spaceUUID,
				}},
			},
			Capability: SpaceCapability{
				SpaceUUID: spaceUUID,
				Token:     capabilityToken,
				Scopes:    []string{scopeJoinChannel, scopeSendMessage, scopeReadHistory},
				ExpiresAt: time.Now().UTC().Add(5 * time.Minute).Unix(),
			},
			ClientUUID: request.ClientUUID,
		},
	})

	mustReadType(t, client, "accept_invite_success", testReadTimeout)
	mustWriteMessage(t, client, WSMessage{
		Type: "join_channel",
		Data: JoinUUID{
			UUID:            channelUUID,
			CapabilityToken: capabilityToken,
		},
	})
	mustReadType(t, client, "joined_channel", testReadTimeout)
}

func TestRelayIntegrationCapabilityEnforcementForChannelActions(t *testing.T) {
	env := newRelayIntegrationEnvWithSigningKey(t, true)
	author := env.connectAuthor(t)

	client := env.dialWS(t)
	defer client.Close()
	challenge := env.joinHost(t, client)
	auth := authenticateClient(t, env, client, challenge, "erin")

	mustWriteMessage(t, client, WSMessage{Type: "get_dash_data", Data: map[string]interface{}{}})
	requestMsg := author.mustNextType("get_dash_data_request")
	request, err := decodeData[GetDashDataRequest](requestMsg.Data)
	if err != nil {
		t.Fatalf("decode get_dash_data_request: %v", err)
	}

	spaceUUID := uuid.NewString()
	channelUUID := uuid.NewString()
	capabilityToken := env.mustIssueCapabilityToken(
		t,
		request.UserPublicKey,
		spaceUUID,
		[]string{scopeJoinChannel, scopeSendMessage, scopeReadHistory},
		5*time.Minute,
	)
	author.mustSend(WSMessage{
		Type: "get_dash_data_response",
		Data: GetDashDataResponse{
			User: DashDataUser{
				ID:           request.UserID,
				Username:     auth.Username,
				PublicKey:    request.UserPublicKey,
				EncPublicKey: request.UserEncPublicKey,
			},
			Spaces: []DashDataSpace{{
				ID:       1,
				UUID:     spaceUUID,
				Name:     "Security",
				AuthorID: request.UserID,
				Channels: []DashDataChannel{{
					ID:        1,
					UUID:      channelUUID,
					Name:      "red-team",
					SpaceUUID: spaceUUID,
				}},
			}},
			Capabilities: []SpaceCapability{{
				SpaceUUID: spaceUUID,
				Token:     capabilityToken,
				Scopes:    []string{scopeJoinChannel, scopeSendMessage, scopeReadHistory},
				ExpiresAt: time.Now().UTC().Add(5 * time.Minute).Unix(),
			}},
			ClientUUID: request.ClientUUID,
		},
	})
	mustReadType(t, client, "dash_data_payload", testReadTimeout)
	mustWriteMessage(t, client, WSMessage{
		Type: "join_all_spaces",
		Data: JoinAllSpacesClient{
			SpaceUUIDs:       []string{spaceUUID},
			CapabilityTokens: map[string]string{spaceUUID: capabilityToken},
		},
	})
	mustReadType(t, client, "join_all_spaces_success", testReadTimeout)

	mustWriteMessage(t, client, WSMessage{Type: "join_channel", Data: JoinUUID{UUID: channelUUID}})
	joinMissingTokenMsg := mustReadType(t, client, "error", testReadTimeout)
	joinMissingTokenErr, err := decodeData[ChatError](joinMissingTokenMsg.Data)
	if err != nil {
		t.Fatalf("decode join missing token error: %v", err)
	}
	if !strings.Contains(joinMissingTokenErr.Content, "Unauthorized channel join") {
		t.Fatalf("expected unauthorized channel join for missing token, got: %q", joinMissingTokenErr.Content)
	}

	mustWriteMessage(t, client, WSMessage{
		Type: "join_channel",
		Data: JoinUUID{UUID: channelUUID, CapabilityToken: "invalid.token"},
	})
	joinInvalidTokenMsg := mustReadType(t, client, "error", testReadTimeout)
	joinInvalidTokenErr, err := decodeData[ChatError](joinInvalidTokenMsg.Data)
	if err != nil {
		t.Fatalf("decode join invalid token error: %v", err)
	}
	if !strings.Contains(joinInvalidTokenErr.Content, "Unauthorized channel join") {
		t.Fatalf("expected unauthorized channel join for invalid token, got: %q", joinInvalidTokenErr.Content)
	}

	mustWriteMessage(t, client, WSMessage{
		Type: "join_channel",
		Data: JoinUUID{UUID: channelUUID, CapabilityToken: capabilityToken},
	})
	mustReadType(t, client, "joined_channel", testReadTimeout)

	mustWriteMessage(t, client, WSMessage{
		Type: "chat",
		Data: ChatData{
			Envelope: map[string]interface{}{
				"ciphertext": "hello",
			},
		},
	})
	chatMissingTokenMsg := mustReadType(t, client, "error", testReadTimeout)
	chatMissingTokenErr, err := decodeData[ChatError](chatMissingTokenMsg.Data)
	if err != nil {
		t.Fatalf("decode chat missing token error: %v", err)
	}
	if !strings.Contains(chatMissingTokenErr.Content, "Unauthorized channel access") {
		t.Fatalf("expected unauthorized channel access for missing chat token, got: %q", chatMissingTokenErr.Content)
	}

	mustWriteMessage(t, client, WSMessage{
		Type: "chat",
		Data: ChatData{
			Envelope: map[string]interface{}{
				"ciphertext": "ok",
			},
			CapabilityToken: capabilityToken,
		},
	})
	author.mustNextType("save_chat_message_request")

	mustWriteMessage(t, client, WSMessage{
		Type: "get_messages",
		Data: GetMessagesClient{BeforeUnixTime: time.Now().UTC().Format(time.RFC3339)},
	})
	getMissingTokenMsg := mustReadType(t, client, "error", testReadTimeout)
	getMissingTokenErr, err := decodeData[ChatError](getMissingTokenMsg.Data)
	if err != nil {
		t.Fatalf("decode get_messages missing token error: %v", err)
	}
	if !strings.Contains(getMissingTokenErr.Content, "Unauthorized channel access") {
		t.Fatalf("expected unauthorized channel access for missing get_messages token, got: %q", getMissingTokenErr.Content)
	}

	mustWriteMessage(t, client, WSMessage{
		Type: "get_messages",
		Data: GetMessagesClient{
			BeforeUnixTime:  time.Now().UTC().Format(time.RFC3339),
			CapabilityToken: capabilityToken,
		},
	})
	getReqMsg := author.mustNextType("get_messages_request")
	getReq, err := decodeData[GetMessagesRequest](getReqMsg.Data)
	if err != nil {
		t.Fatalf("decode get_messages_request: %v", err)
	}
	if getReq.ChannelUUID != channelUUID {
		t.Fatalf("expected channel uuid %q in get_messages_request, got %q", channelUUID, getReq.ChannelUUID)
	}
}

func TestRelayIntegrationCapabilityEnforcementForAdminActions(t *testing.T) {
	env := newRelayIntegrationEnvWithSigningKey(t, true)
	author := env.connectAuthor(t)

	client := env.dialWS(t)
	defer client.Close()
	challenge := env.joinHost(t, client)
	auth := authenticateClient(t, env, client, challenge, "frank")

	mustWriteMessage(t, client, WSMessage{Type: "get_dash_data", Data: map[string]interface{}{}})
	requestMsg := author.mustNextType("get_dash_data_request")
	request, err := decodeData[GetDashDataRequest](requestMsg.Data)
	if err != nil {
		t.Fatalf("decode get_dash_data_request: %v", err)
	}

	spaceUUID := uuid.NewString()
	channelUUID := uuid.NewString()
	memberToken := env.mustIssueCapabilityToken(
		t,
		request.UserPublicKey,
		spaceUUID,
		[]string{scopeJoinChannel, scopeSendMessage, scopeReadHistory},
		5*time.Minute,
	)
	adminToken := env.mustIssueCapabilityToken(
		t,
		request.UserPublicKey,
		spaceUUID,
		[]string{
			scopeJoinChannel,
			scopeSendMessage,
			scopeReadHistory,
			scopeCreateChannel,
			scopeDeleteChannel,
			scopeInviteUser,
			scopeRemoveSpaceUser,
			scopeDeleteSpace,
		},
		5*time.Minute,
	)
	author.mustSend(WSMessage{
		Type: "get_dash_data_response",
		Data: GetDashDataResponse{
			User: DashDataUser{
				ID:           request.UserID,
				Username:     auth.Username,
				PublicKey:    request.UserPublicKey,
				EncPublicKey: request.UserEncPublicKey,
			},
			Spaces: []DashDataSpace{{
				ID:       1,
				UUID:     spaceUUID,
				Name:     "Admin Space",
				AuthorID: request.UserID,
				Channels: []DashDataChannel{{
					ID:        1,
					UUID:      channelUUID,
					Name:      "ops",
					SpaceUUID: spaceUUID,
				}},
			}},
			Capabilities: []SpaceCapability{{
				SpaceUUID: spaceUUID,
				Token:     adminToken,
				Scopes: []string{
					scopeJoinChannel,
					scopeSendMessage,
					scopeReadHistory,
					scopeCreateChannel,
					scopeDeleteChannel,
					scopeInviteUser,
					scopeRemoveSpaceUser,
					scopeDeleteSpace,
				},
				ExpiresAt: time.Now().UTC().Add(5 * time.Minute).Unix(),
			}},
			ClientUUID: request.ClientUUID,
		},
	})
	mustReadType(t, client, "dash_data_payload", testReadTimeout)
	mustWriteMessage(t, client, WSMessage{
		Type: "join_all_spaces",
		Data: JoinAllSpacesClient{
			SpaceUUIDs:       []string{spaceUUID},
			CapabilityTokens: map[string]string{spaceUUID: memberToken},
		},
	})
	mustReadType(t, client, "join_all_spaces_success", testReadTimeout)

	mustWriteMessage(t, client, WSMessage{
		Type: "create_channel",
		Data: CreateChannelClient{
			Name:            "blocked",
			SpaceUUID:       spaceUUID,
			CapabilityToken: memberToken,
		},
	})
	_ = mustReadUnauthorizedError(t, client)
	mustWriteMessage(t, client, WSMessage{
		Type: "create_channel",
		Data: CreateChannelClient{
			Name:            "allowed",
			SpaceUUID:       spaceUUID,
			CapabilityToken: adminToken,
		},
	})
	author.mustNextType("create_channel_request")

	mustWriteMessage(t, client, WSMessage{
		Type: "delete_channel",
		Data: DeleteChannelClient{
			UUID:            channelUUID,
			SpaceUUID:       spaceUUID,
			CapabilityToken: memberToken,
		},
	})
	_ = mustReadUnauthorizedError(t, client)
	mustWriteMessage(t, client, WSMessage{
		Type: "delete_channel",
		Data: DeleteChannelClient{
			UUID:            channelUUID,
			SpaceUUID:       spaceUUID,
			CapabilityToken: adminToken,
		},
	})
	author.mustNextType("delete_channel_request")

	mustWriteMessage(t, client, WSMessage{
		Type: "invite_user",
		Data: InviteUserClient{
			PublicKey:       "target-pubkey",
			SpaceUUID:       spaceUUID,
			CapabilityToken: memberToken,
		},
	})
	_ = mustReadUnauthorizedError(t, client)
	mustWriteMessage(t, client, WSMessage{
		Type: "invite_user",
		Data: InviteUserClient{
			PublicKey:       "target-pubkey",
			SpaceUUID:       spaceUUID,
			CapabilityToken: adminToken,
		},
	})
	author.mustNextType("invite_user_request")

	mustWriteMessage(t, client, WSMessage{
		Type: "remove_space_user",
		Data: RemoveSpaceUserClient{
			SpaceUUID:       spaceUUID,
			UserID:          55,
			UserPublicKey:   "target-pubkey",
			CapabilityToken: memberToken,
		},
	})
	_ = mustReadUnauthorizedError(t, client)
	mustWriteMessage(t, client, WSMessage{
		Type: "remove_space_user",
		Data: RemoveSpaceUserClient{
			SpaceUUID:       spaceUUID,
			UserID:          55,
			UserPublicKey:   "target-pubkey",
			CapabilityToken: adminToken,
		},
	})
	author.mustNextType("remove_space_user_request")

	mustWriteMessage(t, client, WSMessage{
		Type: "delete_space",
		Data: DeleteSpaceClient{
			UUID:            spaceUUID,
			CapabilityToken: memberToken,
		},
	})
	_ = mustReadUnauthorizedError(t, client)
	mustWriteMessage(t, client, WSMessage{
		Type: "delete_space",
		Data: DeleteSpaceClient{
			UUID:            spaceUUID,
			CapabilityToken: adminToken,
		},
	})
	author.mustNextType("delete_space_request")
}

func TestRelayIntegrationHostAuthorJoinTrustRules(t *testing.T) {
	env := newRelayIntegrationEnv(t)
	authorConn := env.dialWS(t)
	defer authorConn.Close()

	env.joinHostAsAuthor(t, authorConn)
	host, ok := GetHost(env.hostUUID)
	if !ok {
		t.Fatal("expected host to exist after author join")
	}

	host.mu.Lock()
	defer host.mu.Unlock()
	if host.AuthorConn == nil {
		t.Fatal("expected host author connection to be registered")
	}
	if len(host.ClientsByConn) != 1 {
		t.Fatalf("expected exactly one connected author client, got %d", len(host.ClientsByConn))
	}
	for _, client := range host.ClientsByConn {
		if !client.IsHostAuthor {
			t.Fatal("expected connected client to be marked as host author")
		}
	}
}
