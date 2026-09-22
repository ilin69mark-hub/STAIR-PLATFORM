package ws

// S-132a: протокол подписок на pipeline-комнаты (client→server).

import (
	"encoding/json"
	"testing"
	"time"
)

const testRoom = "pipeline:123e4567-e89b-12d3-a456-426614174000"

func newTestClient(hub *Hub) *Client {
	return &Client{
		send:   make(chan []byte, 256),
		hub:    hub,
		userID: "user-1",
		rooms:  make(map[string]bool),
	}
}

func subMsg(typ MessageType, room string) Message {
	p, _ := json.Marshal(roomPayload{Room: room})
	return Message{Type: typ, Payload: p, Time: time.Now()}
}

func readAck(t *testing.T, c *Client) Message {
	t.Helper()
	select {
	case data := <-c.send:
		var m Message
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("ack unmarshal: %v", err)
		}
		return m
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ack")
		return Message{}
	}
}

func assertNoMsg(t *testing.T, c *Client) {
	t.Helper()
	select {
	case data := <-c.send:
		t.Fatalf("unexpected message: %s", data)
	case <-time.After(50 * time.Millisecond):
	}
}

func inRoom(hub *Hub, c *Client, room string) bool {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	clients, ok := hub.rooms[room]
	return ok && clients[c]
}

func TestValidPipelineRoom(t *testing.T) {
	for _, room := range []string{
		testRoom,
		"pipeline:AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
	} {
		if !validPipelineRoom(room) {
			t.Errorf("expected valid: %q", room)
		}
	}
	for _, room := range []string{
		"",
		"pipeline:",
		"pipeline:not-a-uuid",
		"pipeline:123",
		"other:123e4567-e89b-12d3-a456-426614174000",
		"pipeline:123e4567-e89b-12d3-a456-426614174000/extra",
		"PIPELINE:123e4567-e89b-12d3-a456-426614174000",
	} {
		if validPipelineRoom(room) {
			t.Errorf("expected invalid: %q", room)
		}
	}
}

func TestSubscribeJoinAndAck(t *testing.T) {
	hub := NewHub()
	c := newTestClient(hub)

	c.handleClientMessage(subMsg(MessageTypeSubscribe, testRoom))

	if !inRoom(hub, c, testRoom) {
		t.Fatal("client not joined after subscribe")
	}
	ack := readAck(t, c)
	if ack.Type != MessageTypeSubscribed {
		t.Fatalf("ack type = %q, want subscribed", ack.Type)
	}
	var p roomPayload
	if err := json.Unmarshal(ack.Payload, &p); err != nil || p.Room != testRoom {
		t.Fatalf("ack payload = %s, want room %q", ack.Payload, testRoom)
	}
}

func TestSubscribeInvalidIgnored(t *testing.T) {
	hub := NewHub()
	c := newTestClient(hub)

	for _, room := range []string{"", "admin-room", "pipeline:nope", "pipeline:"} {
		c.handleClientMessage(subMsg(MessageTypeSubscribe, room))
	}
	assertNoMsg(t, c)
	hub.mu.RLock()
	n := len(hub.rooms)
	hub.mu.RUnlock()
	if n != 0 {
		t.Fatalf("expected no rooms, got %d", n)
	}
}

func TestBroadcastReachesSubscriber(t *testing.T) {
	hub := NewHub()
	member := newTestClient(hub)
	outsider := newTestClient(hub)

	member.handleClientMessage(subMsg(MessageTypeSubscribe, testRoom))
	readAck(t, member) // drain ack

	hub.BroadcastToRoom(testRoom, Message{Type: MessageTypePipelineStatus, Time: time.Now()})

	select {
	case <-member.send:
	case <-time.After(time.Second):
		t.Fatal("subscriber got no broadcast")
	}
	assertNoMsg(t, outsider)
}

func TestUnsubscribeLeavesAndAck(t *testing.T) {
	hub := NewHub()
	c := newTestClient(hub)

	c.handleClientMessage(subMsg(MessageTypeSubscribe, testRoom))
	readAck(t, c)
	c.handleClientMessage(subMsg(MessageTypeUnsubscribe, testRoom))

	if inRoom(hub, c, testRoom) {
		t.Fatal("client still in room after unsubscribe")
	}
	ack := readAck(t, c)
	if ack.Type != MessageTypeUnsubscribed {
		t.Fatalf("ack type = %q, want unsubscribed", ack.Type)
	}

	// После отписки broadcast не доходит.
	hub.BroadcastToRoom(testRoom, Message{Type: MessageTypePing, Time: time.Now()})
	assertNoMsg(t, c)
}

func TestServerEventsStillIgnored(t *testing.T) {
	hub := NewHub()
	c := newTestClient(hub)

	for _, typ := range []MessageType{
		MessageTypePipelineStatus, MessageTypeAnalysisProgress,
		MessageTypeDocumentGenerated, MessageTypeNotification,
		MessageTypePong, MessageTypeSubscribed, MessageTypeUnsubscribed,
		"bogus-type",
	} {
		c.handleClientMessage(Message{Type: typ, Time: time.Now()})
	}
	assertNoMsg(t, c)
}

func TestPingStillPong(t *testing.T) {
	hub := NewHub()
	c := newTestClient(hub)

	c.handleClientMessage(Message{Type: MessageTypePing, Time: time.Now()})
	m := readAck(t, c)
	if m.Type != MessageTypePong {
		t.Fatalf("type = %q, want pong", m.Type)
	}
}
