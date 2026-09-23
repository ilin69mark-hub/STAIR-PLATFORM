package ws

// S-132c: авторизация подписок на pipeline-комнаты.

import (
	"testing"
)

type fakeAuthorizer struct {
	// allow — ответ авторизатора; if deny for user, only userID="" allowed.
	allow    map[string]bool
	allowAll bool
	gotUser  []string
	gotRoom  []string
	gotRole  []string
}

func (f *fakeAuthorizer) CanSubscribe(userID, role, room string) bool {
	f.gotUser = append(f.gotUser, userID)
	f.gotRole = append(f.gotRole, role)
	f.gotRoom = append(f.gotRoom, room)
	if f.allowAll {
		return true
	}
	return f.allow[userID]
}

func TestSubscribeWithAuthorizerAllowed(t *testing.T) {
	hub := NewHub()
	auth := &fakeAuthorizer{allow: map[string]bool{"user-1": true}}
	c := newTestClient(hub)
	WithSubscriberAuthorizer(auth)(c)

	room := "pipeline:11111111-2222-3333-4444-555555555555"
	c.handleClientMessage(subMsg(MessageTypeSubscribe, room))

	if !inRoom(hub, c, room) {
		t.Fatal("client not joined after allowed subscribe")
	}
	if len(auth.gotRoom) != 1 || auth.gotRoom[0] != room {
		t.Fatalf("authorizer got rooms %v, want [%s]", auth.gotRoom, room)
	}
}

func TestSubscribeWithAuthorizerDenied(t *testing.T) {
	hub := NewHub()
	auth := &fakeAuthorizer{allow: map[string]bool{}}
	c := newTestClient(hub)
	WithSubscriberAuthorizer(auth)(c)

	room := "pipeline:77777777-8888-9999-aaaa-bbbbbbbbbbbb"
	c.handleClientMessage(subMsg(MessageTypeSubscribe, room))

	if inRoom(hub, c, room) {
		t.Fatal("client joined despite denied subscribe")
	}
	assertNoMsg(t, c)
	if len(auth.gotUser) != 1 || auth.gotUser[0] != "user-1" {
		t.Fatalf("authorizer got users %v, want [user-1]", auth.gotUser)
	}
}

func TestSubscribeRolePassedToAuthorizer(t *testing.T) {
	hub := NewHub()
	auth := &fakeAuthorizer{allowAll: true}
	c := newTestClient(hub)
	WithRole("editor")(c)
	WithSubscriberAuthorizer(auth)(c)

	c.handleClientMessage(subMsg(MessageTypeSubscribe, testRoom))

	if !inRoom(hub, c, testRoom) {
		t.Fatal("authorizer allowed subscribe, client should join")
	}
	if len(auth.gotRole) != 1 || auth.gotRole[0] != "editor" {
		t.Fatalf("authorizer got roles %v, want [editor]", auth.gotRole)
	}
	if len(auth.gotUser) != 1 || auth.gotUser[0] != "user-1" {
		t.Fatalf("authorizer got users %v, want [user-1]", auth.gotUser)
	}
}
