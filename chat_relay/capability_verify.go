package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	scopeJoinChannel     = "join_channel"
	scopeSendMessage     = "send_message"
	scopeReadHistory     = "read_history"
	scopeCreateChannel   = "create_channel"
	scopeDeleteChannel   = "delete_channel"
	scopeInviteUser      = "invite_user"
	scopeRemoveSpaceUser = "remove_space_user"
	scopeDeleteSpace     = "delete_space"
)

func parseHostSigningPublicKey(signingPublicKey string) (ed25519.PublicKey, error) {
	trimmed := strings.TrimSpace(signingPublicKey)
	if trimmed == "" {
		return nil, fmt.Errorf("missing host signing key")
	}
	decoded, err := base64.RawStdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid host signing key")
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid host signing key length")
	}
	return ed25519.PublicKey(decoded), nil
}

func containsScope(scopes []string, required string) bool {
	if required == "" {
		return true
	}
	for _, scope := range scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func verifySpaceCapability(
	client *Client,
	hostUUID string,
	signingPublicKey string,
	spaceUUID string,
	channelUUID string,
	token string,
	requiredScope string,
	now time.Time,
) error {
	if client == nil {
		return fmt.Errorf("missing client session")
	}
	if strings.TrimSpace(client.PublicKey) == "" {
		return fmt.Errorf("missing authenticated public key")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("missing capability token")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return fmt.Errorf("malformed capability token")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("invalid capability payload")
	}
	signatureBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("invalid capability signature")
	}
	if len(signatureBytes) != ed25519.SignatureSize {
		return fmt.Errorf("invalid capability signature length")
	}

	publicKey, err := parseHostSigningPublicKey(signingPublicKey)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payloadBytes, signatureBytes) {
		return fmt.Errorf("invalid capability signature")
	}

	var claims SpaceCapabilityClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return fmt.Errorf("invalid capability claims")
	}
	if claims.Version != 1 {
		return fmt.Errorf("unsupported capability version")
	}
	if claims.HostUUID != hostUUID {
		return fmt.Errorf("capability host mismatch")
	}
	if claims.SpaceUUID != spaceUUID {
		return fmt.Errorf("capability space mismatch")
	}
	if claims.SubjectKey != client.PublicKey {
		return fmt.Errorf("capability subject mismatch")
	}
	if claims.ExpiresAt <= 0 || now.Unix() >= claims.ExpiresAt {
		return fmt.Errorf("capability expired")
	}
	if claims.IssuedAt > now.Unix()+90 {
		return fmt.Errorf("capability issued-at is invalid")
	}
	if !containsScope(claims.Scopes, requiredScope) {
		return fmt.Errorf("capability scope denied")
	}

	channelScope := strings.TrimSpace(claims.ChannelScope)
	if channelUUID != "" && channelScope != "" && channelScope != "*" && channelScope != channelUUID {
		return fmt.Errorf("capability channel mismatch")
	}

	return nil
}
