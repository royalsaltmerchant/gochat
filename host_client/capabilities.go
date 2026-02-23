package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const capabilityTokenTTL = 5 * time.Minute

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

var memberScopes = []string{
	scopeJoinChannel,
	scopeSendMessage,
	scopeReadHistory,
}

var adminScopes = []string{
	scopeCreateChannel,
	scopeDeleteChannel,
	scopeInviteUser,
	scopeRemoveSpaceUser,
	scopeDeleteSpace,
}

func currentSigningPrivateKey() (ed25519.PrivateKey, error) {
	if runtimeHostConfig == nil {
		return nil, fmt.Errorf("host config not initialized")
	}
	raw := strings.TrimSpace(runtimeHostConfig.SigningPrivateKey)
	if raw == "" {
		return nil, fmt.Errorf("missing host signing private key")
	}
	decoded, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid host signing private key: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid host signing private key size")
	}
	return ed25519.PrivateKey(decoded), nil
}

func issueSpaceCapabilitiesForUser(user DashDataUser, spaces []DashDataSpace) ([]SpaceCapability, error) {
	if strings.TrimSpace(user.PublicKey) == "" {
		return nil, fmt.Errorf("missing user public key for capability issuance")
	}
	priv, err := currentSigningPrivateKey()
	if err != nil {
		return nil, err
	}

	caps := make([]SpaceCapability, 0, len(spaces))
	for _, space := range spaces {
		cap, err := issueSpaceCapabilityForSpace(priv, user, space)
		if err != nil {
			return nil, err
		}
		caps = append(caps, cap)
	}
	return caps, nil
}

func issueSpaceCapabilityForSpace(priv ed25519.PrivateKey, user DashDataUser, space DashDataSpace) (SpaceCapability, error) {
	if strings.TrimSpace(space.UUID) == "" {
		return SpaceCapability{}, fmt.Errorf("missing space uuid")
	}

	scopeSet := make(map[string]struct{}, len(memberScopes)+len(adminScopes))
	for _, scope := range memberScopes {
		scopeSet[scope] = struct{}{}
	}
	if space.AuthorID == user.ID {
		for _, scope := range adminScopes {
			scopeSet[scope] = struct{}{}
		}
	}

	scopes := make([]string, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	now := time.Now().UTC()
	claims := SpaceCapabilityClaims{
		Version:      1,
		HostUUID:     currentHostUUID,
		SpaceUUID:    space.UUID,
		SubjectKey:   user.PublicKey,
		Scopes:       scopes,
		ExpiresAt:    now.Add(capabilityTokenTTL).Unix(),
		IssuedAt:     now.Unix(),
		TokenID:      uuid.NewString(),
		ChannelScope: "*",
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return SpaceCapability{}, err
	}
	signature := ed25519.Sign(priv, payload)
	token := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature)

	return SpaceCapability{
		SpaceUUID: space.UUID,
		Token:     token,
		Scopes:    scopes,
		ExpiresAt: claims.ExpiresAt,
	}, nil
}
