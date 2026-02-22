package main

import (
	"testing"

	"github.com/gorilla/websocket"
)

func TestJoinSpaceRequiresAuthorization(t *testing.T) {
	const hostUUID = "host-space-auth-test"
	const spaceUUID = "space-space-auth-test"

	conn := &websocket.Conn{}
	client := &Client{
		Conn:             conn,
		HostUUID:         hostUUID,
		AuthorizedSpaces: map[string]struct{}{},
	}
	host := &Host{
		UUID:               hostUUID,
		ClientsByConn:      map[*websocket.Conn]*Client{conn: client},
		ClientConnsByUUID:  make(map[string]*websocket.Conn),
		ClientsByUserID:    make(map[int]*Client),
		ClientsByPublicKey: make(map[string]*Client),
		ConnByAuthorID:     make(map[string]*websocket.Conn),
		Channels:           make(map[string]*Channel),
		ChannelToSpace:     make(map[string]string),
		Spaces: map[string]*Space{
			spaceUUID: {Users: make(map[*websocket.Conn]int)},
		},
		ChannelSubscriptions: make(map[*websocket.Conn]string),
		SpaceSubscriptions:   make(map[*websocket.Conn][]string),
	}

	hostsMu.Lock()
	originalHosts := Hosts
	Hosts = map[string]*Host{hostUUID: host}
	hostsMu.Unlock()
	t.Cleanup(func() {
		hostsMu.Lock()
		Hosts = originalHosts
		hostsMu.Unlock()
	})

	if joinSpace(client, spaceUUID) {
		t.Fatalf("expected joinSpace to reject unauthorized space")
	}
	if got := len(host.Spaces[spaceUUID].Users); got != 0 {
		t.Fatalf("expected no users in unauthorized space join, got %d", got)
	}

	client.AuthorizedSpaces[spaceUUID] = struct{}{}
	if !joinSpace(client, spaceUUID) {
		t.Fatalf("expected joinSpace to allow authorized space")
	}
	if got := len(host.Spaces[spaceUUID].Users); got != 1 {
		t.Fatalf("expected 1 user in space after authorized join, got %d", got)
	}
}
