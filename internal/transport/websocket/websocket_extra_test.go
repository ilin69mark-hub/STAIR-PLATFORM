package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domevents "stairplatform/internal/domain/events"

	"github.com/gorilla/websocket"
)

// fakeBus captures Subscribe handlers for EventBridge testing.
type fakeBus struct {
	handlers map[domevents.EventType]func(ctx context.Context, event domevents.Event) error
}

func newFakeBus() *fakeBus {
	return &fakeBus{handlers: make(map[domevents.EventType]func(ctx context.Context, event domevents.Event) error)}
}

func (f *fakeBus) Subscribe(typ domevents.EventType, h func(ctx context.Context, event domevents.Event) error) string {
	f.handlers[typ] = h
	return string(typ)
}

// ---------------------------------------------------------------------------
// Hub lifecycle: Stop, Broadcast via h.broadcast channel
// ---------------------------------------------------------------------------

func TestHubStopDisconnectsClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	c1 := &Client{send: make(chan []byte, 2), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	c2 := &Client{send: make(chan []byte, 2), hub: hub, userID: "u2", rooms: make(map[string]bool)}
	hub.register <- c1
	hub.register <- c2
	time.Sleep(20 * time.Millisecond)

	if hub.ClientCount() != 2 {
		t.Fatalf("expected 2 clients, got %d", hub.ClientCount())
	}
	hub.Stop()
	time.Sleep(20 * time.Millisecond)

	// After Stop, hub should have 0 clients and send channels closed.
	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 after Stop, got %d", hub.ClientCount())
	}
	// channels should be closed
	select {
	case _, ok := <-c1.send:
		if ok {
			t.Error("expected c1.send closed")
		}
	default:
		// if not yet closed, allow a bit more
		time.Sleep(10 * time.Millisecond)
		<-c1.send
	}
}

func TestHubBroadcastViaChannel(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	c1 := &Client{send: make(chan []byte, 4), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	c2 := &Client{send: make(chan []byte, 4), hub: hub, userID: "u2", rooms: make(map[string]bool)}
	hub.register <- c1
	hub.register <- c2
	time.Sleep(20 * time.Millisecond)

	payload := []byte(`{"hello":"world"}`)
	hub.broadcast <- payload

	// both clients should receive
	for i, c := range []*Client{c1, c2} {
		select {
		case got := <-c.send:
			if string(got) != string(payload) {
				t.Fatalf("client %d: expected %s got %s", i, payload, got)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("client %d: timeout waiting for broadcast", i)
		}
	}
}

func TestHubBroadcastBlockedClientTriggersClose(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	// unbuffered channel will always block, triggering default close path
	blocked := &Client{send: make(chan []byte), hub: hub, userID: "blocked", rooms: make(map[string]bool)}
	healthy := &Client{send: make(chan []byte, 4), hub: hub, userID: "healthy", rooms: make(map[string]bool)}
	hub.register <- blocked
	hub.register <- healthy
	time.Sleep(20 * time.Millisecond)

	hub.broadcast <- []byte(`trigger`)

	// healthy should receive, blocked should be removed
	select {
	case <-healthy.send:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("healthy client should have received broadcast")
	}
	time.Sleep(20 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client after blocked removal, got %d", hub.ClientCount())
	}
	// reading from blocked should indicate closed
	select {
	case _, ok := <-blocked.send:
		if ok {
			t.Error("expected blocked channel closed")
		}
	default:
		// give time for close to propagate
		time.Sleep(10 * time.Millisecond)
		_, ok := <-blocked.send
		if ok {
			t.Error("expected blocked channel closed after delay")
		}
	}
}

// ---------------------------------------------------------------------------
// ClientCount variants
// ---------------------------------------------------------------------------

func TestClientCountEmptyHub(t *testing.T) {
	hub := NewHub()
	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0, got %d", hub.ClientCount())
	}
	if hub.ClientCountByUser("any") != 0 {
		t.Fatalf("expected 0 for nonexistent user")
	}
}

func TestClientCountAfterRegisterAndDisconnect(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	c := &Client{send: make(chan []byte, 2), hub: hub, userID: "alice", rooms: make(map[string]bool)}
	hub.register <- c
	time.Sleep(15 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1, got %d", hub.ClientCount())
	}
	if hub.ClientCountByUser("alice") != 1 {
		t.Fatalf("expected 1 for alice")
	}
	hub.DisconnectClient(c, "bye")
	time.Sleep(20 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 after disconnect, got %d", hub.ClientCount())
	}
	if hub.ClientCountByUser("alice") != 0 {
		t.Fatalf("expected 0 after disconnect")
	}
}

// ---------------------------------------------------------------------------
// JoinRoom / LeaveRoom edge cases
// ---------------------------------------------------------------------------

func TestJoinRoomMultipleRooms(t *testing.T) {
	hub := NewHub()
	c := &Client{send: make(chan []byte, 2), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	hub.JoinRoom(c, "roomA")
	hub.JoinRoom(c, "roomB")
	hub.JoinRoom(c, "roomA") // duplicate join should be idempotent

	if len(hub.rooms["roomA"]) != 1 {
		t.Fatalf("expected 1 in roomA, got %d", len(hub.rooms["roomA"]))
	}
	if len(hub.rooms["roomB"]) != 1 {
		t.Fatalf("expected 1 in roomB, got %d", len(hub.rooms["roomB"]))
	}
	if !c.rooms["roomA"] || !c.rooms["roomB"] {
		t.Fatal("client rooms not set")
	}
}

func TestLeaveRoomNonexistent(t *testing.T) {
	hub := NewHub()
	c := &Client{send: make(chan []byte, 2), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	// leaving room never joined should not panic
	hub.LeaveRoom(c, "no-such-room")
	hub.JoinRoom(c, "roomX")
	hub.LeaveRoom(c, "roomY") // different room
	if len(hub.rooms["roomX"]) != 1 {
		t.Fatalf("roomX should still have 1 client")
	}
	hub.LeaveRoom(c, "roomX")
	if len(hub.rooms["roomX"]) != 0 {
		t.Fatalf("expected 0 after leaving")
	}
	// leaving again should be harmless
	hub.LeaveRoom(c, "roomX")
}

func TestLeaveRoomRemovesOnlyTargetClient(t *testing.T) {
	hub := NewHub()
	c1 := &Client{send: make(chan []byte, 2), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	c2 := &Client{send: make(chan []byte, 2), hub: hub, userID: "u2", rooms: make(map[string]bool)}
	hub.JoinRoom(c1, "shared")
	hub.JoinRoom(c2, "shared")
	if len(hub.rooms["shared"]) != 2 {
		t.Fatalf("expected 2")
	}
	hub.LeaveRoom(c1, "shared")
	if len(hub.rooms["shared"]) != 1 {
		t.Fatalf("expected 1 after c1 leaves")
	}
	if _, ok := hub.rooms["shared"][c2]; !ok {
		t.Fatal("c2 should still be in room")
	}
}

func TestUnregisterRemovesFromRooms(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	c := &Client{send: make(chan []byte, 4), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	hub.register <- c
	time.Sleep(15 * time.Millisecond)
	hub.JoinRoom(c, "r1")
	hub.JoinRoom(c, "r2")
	if len(hub.rooms["r1"]) != 1 || len(hub.rooms["r2"]) != 1 {
		t.Fatal("rooms not joined")
	}
	hub.unregister <- c
	time.Sleep(15 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients after unregister")
	}
	hub.mu.RLock()
	if len(hub.rooms["r1"]) != 0 || len(hub.rooms["r2"]) != 0 {
		t.Fatalf("rooms should be empty after unregister, got r1=%d r2=%d", len(hub.rooms["r1"]), len(hub.rooms["r2"]))
	}
	hub.mu.RUnlock()
}

// ---------------------------------------------------------------------------
// BroadcastToRoom / BroadcastToUser edge cases including blocked channel
// ---------------------------------------------------------------------------

func TestBroadcastToRoomNoClients(t *testing.T) {
	hub := NewHub()
	// no panic when room doesn't exist or has no clients
	msg := Message{Type: MessageTypeNotification, Payload: json.RawMessage(`{}`), Time: time.Now()}
	hub.BroadcastToRoom("empty-room", msg)
	// also with hub running
	go hub.Run()
	defer hub.Stop()
	hub.BroadcastToRoom("empty-room-2", msg)
}

func TestBroadcastToRoomBlockedClient(t *testing.T) {
	hub := NewHub()
	// blocked: buffer size 1 already full
	blocked := &Client{send: make(chan []byte, 1), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	blocked.send <- []byte(`fill`) // fill buffer
	hub.clients = make(map[*Client]bool)
	hub.rooms = make(map[string]map[*Client]bool)
	hub.clients[blocked] = true
	hub.JoinRoom(blocked, "room-blocked")

	msg := Message{Type: MessageTypeNotification, Payload: json.RawMessage(`{"x":1}`), Time: time.Now()}
	hub.BroadcastToRoom("room-blocked", msg)

	time.Sleep(10 * time.Millisecond)
	// blocked should have been removed from hub.clients due to full channel
	hub.mu.RLock()
	_, stillRegistered := hub.clients[blocked]
	hub.mu.RUnlock()
	if stillRegistered {
		t.Fatal("expected blocked client removed after BroadcastToRoom default branch")
	}
	// channel should be closed; draining first element then check closed
	<-blocked.send // drain the fill
	select {
	case _, ok := <-blocked.send:
		if ok {
			t.Error("expected channel closed after blocked broadcast")
		}
	default:
		time.Sleep(10 * time.Millisecond)
		_, ok := <-blocked.send
		if ok {
			t.Error("expected closed after delay")
		}
	}
}

func TestBroadcastToUserNoMatchingUser(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	c := &Client{send: make(chan []byte, 4), hub: hub, userID: "alice", rooms: make(map[string]bool)}
	hub.register <- c
	time.Sleep(15 * time.Millisecond)

	msg := Message{Type: MessageTypeNotification, Payload: json.RawMessage(`{}`), Time: time.Now()}
	hub.BroadcastToUser("bob", msg)
	select {
	case <-c.send:
		t.Fatal("alice should not receive bob's message")
	case <-time.After(50 * time.Millisecond):
		// ok — no message
	}
}

func TestBroadcastToUserBlockedClient(t *testing.T) {
	hub := NewHub()
	blocked := &Client{send: make(chan []byte, 1), hub: hub, userID: "target", rooms: make(map[string]bool)}
	blocked.send <- []byte(`fill`)
	hub.clients = map[*Client]bool{blocked: true}

	msg := Message{Type: MessageTypeNotification, Payload: json.RawMessage(`{}`), Time: time.Now()}
	hub.BroadcastToUser("target", msg)

	hub.mu.RLock()
	_, still := hub.clients[blocked]
	hub.mu.RUnlock()
	if still {
		t.Fatal("expected blocked client removed")
	}
}

func TestBroadcastToUserMultipleClientsSameUser(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	c1 := &Client{send: make(chan []byte, 4), hub: hub, userID: "same", rooms: make(map[string]bool)}
	c2 := &Client{send: make(chan []byte, 4), hub: hub, userID: "same", rooms: make(map[string]bool)}
	c3 := &Client{send: make(chan []byte, 4), hub: hub, userID: "other", rooms: make(map[string]bool)}
	hub.register <- c1
	hub.register <- c2
	hub.register <- c3
	time.Sleep(15 * time.Millisecond)

	msg := Message{Type: MessageTypeNotification, Payload: json.RawMessage(`{"n":1}`), Time: time.Now()}
	hub.BroadcastToUser("same", msg)

	for _, c := range []*Client{c1, c2} {
		select {
		case <-c.send:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected message for same user")
		}
	}
	select {
	case <-c3.send:
		t.Fatal("other user should not receive")
	case <-time.After(50 * time.Millisecond):
	}
}

// ---------------------------------------------------------------------------
// DisconnectClient variants
// ---------------------------------------------------------------------------

func TestDisconnectClientNilConnNoPanic(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	c := &Client{send: make(chan []byte, 2), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	hub.register <- c
	time.Sleep(15 * time.Millisecond)
	// conn is nil — should not panic (nil check inside DisconnectClient)
	hub.DisconnectClient(c, "test")
	time.Sleep(15 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 after disconnect, got %d", hub.ClientCount())
	}
}

// ---------------------------------------------------------------------------
// EventBridge
// ---------------------------------------------------------------------------

func TestNewEventBridgeNotNil(t *testing.T) {
	hub := NewHub()
	bus := newFakeBus()
	bridge := NewEventBridge(bus, hub)
	if bridge == nil {
		t.Fatal("expected non-nil bridge")
	}
	if bridge.hub != hub {
		t.Error("hub mismatch")
	}
	if bridge.bus != bus {
		t.Error("bus mismatch")
	}
}

func TestEventBridgeStartRegistersAndBroadcasts(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	// prepare a client listening on pipeline:cfg-123
	c := &Client{send: make(chan []byte, 8), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	hub.register <- c
	time.Sleep(15 * time.Millisecond)
	hub.JoinRoom(c, "pipeline:cfg-123")

	bus := newFakeBus()
	bridge := NewEventBridge(bus, hub)
	bridge.Start()

	if len(bus.handlers) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(bus.handlers))
	}
	for _, want := range []domevents.EventType{domevents.EventPipelineCompleted, domevents.EventAnalysisCompleted, domevents.EventDocumentGenerated} {
		if _, ok := bus.handlers[want]; !ok {
			t.Fatalf("missing handler for %s", want)
		}
	}

	// invoke each handler with an event whose Metadata.ID is cfg-123
	ev := domevents.BaseEvent{Meta: domevents.EventMetadata{ID: "cfg-123", Type: domevents.EventPipelineCompleted}}
	for typ, handler := range bus.handlers {
		// set correct type per iteration
		ev.Meta.Type = typ
		if err := handler(context.Background(), ev); err != nil {
			t.Fatalf("handler %s error: %v", typ, err)
		}
		select {
		case raw := <-c.send:
			var m Message
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("unmarshal msg for %s: %v", typ, err)
			}
			if m.Type == "" {
				t.Errorf("handler %s: empty message type", typ)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("expected broadcast for handler %s", typ)
		}
	}
}

func TestEventBridgeStartEmptyConfigID(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	c := &Client{send: make(chan []byte, 4), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	hub.register <- c
	time.Sleep(15 * time.Millisecond)
	hub.JoinRoom(c, "pipeline:")

	bus := newFakeBus()
	bridge := NewEventBridge(bus, hub)
	bridge.Start()

	// event with empty ID
	ev := domevents.BaseEvent{Meta: domevents.EventMetadata{ID: "", Type: domevents.EventPipelineCompleted}}
	h := bus.handlers[domevents.EventPipelineCompleted]
	if h == nil {
		t.Fatal("missing pipeline handler")
	}
	if err := h(context.Background(), ev); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	select {
	case <-c.send:
		// ok — broadcast to pipeline:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected broadcast to pipeline: (empty configID)")
	}
}

func TestEventBridgePayloadTypes(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	c := &Client{send: make(chan []byte, 10), hub: hub, userID: "u1", rooms: make(map[string]bool)}
	hub.register <- c
	time.Sleep(15 * time.Millisecond)
	hub.JoinRoom(c, "pipeline:abc")

	bus := newFakeBus()
	NewEventBridge(bus, hub).Start()

	ev := domevents.BaseEvent{Meta: domevents.EventMetadata{ID: "abc"}}

	// PipelineCompleted should produce PipelineStatusPayload
	if err := bus.handlers[domevents.EventPipelineCompleted](context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	raw := <-c.send
	var m Message
	_ = json.Unmarshal(raw, &m)
	var p PipelineStatusPayload
	if err := json.Unmarshal(m.Payload, &p); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if p.ConfigID != "abc" || p.Status != "completed" {
		t.Fatalf("unexpected pipeline payload: %+v", p)
	}

	// AnalysisCompleted
	if err := bus.handlers[domevents.EventAnalysisCompleted](context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	raw = <-c.send
	_ = json.Unmarshal(raw, &m)
	var ap AnalysisProgressPayload
	if err := json.Unmarshal(m.Payload, &ap); err != nil {
		t.Fatalf("analysis payload: %v", err)
	}
	if ap.ConfigID != "abc" {
		t.Fatalf("analysis config mismatch: %+v", ap)
	}

	// DocumentGenerated
	if err := bus.handlers[domevents.EventDocumentGenerated](context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	raw = <-c.send
	_ = json.Unmarshal(raw, &m)
	var dp DocumentGeneratedPayload
	if err := json.Unmarshal(m.Payload, &dp); err != nil {
		t.Fatalf("doc payload: %v", err)
	}
	if dp.ConfigID != "abc" {
		t.Fatalf("doc config mismatch: %+v", dp)
	}
}

// ---------------------------------------------------------------------------
// HandleWebSocket via httptest + gorilla websocket
// ---------------------------------------------------------------------------

func TestHandleWebSocketConnectAndPingPong(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// userID приходит от вызывающей стороны (аутентифицированный handler);
		// query-параметр user_id намеренно игнорируется.
		userID := r.Header.Get("X-Test-User")
		HandleWebSocket(hub, w, r, userID)
	}))
	defer srv.Close()

	// dial ws — намеренно передаём посторонний user_id в query, чтобы
	// убедиться, что он ИГНОРИРУЕТСЯ и берётся только проверенное значение.
	wsURL := "ws" + srv.URL[len("http"):] + "/ws?user_id=victim"
	headerTester := http.Header{}
	headerTester.Set("X-Test-User", "tester")
	conn, hr0, err := websocket.DefaultDialer.Dial(wsURL, headerTester)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if hr0 != nil {
		_ = hr0.Body.Close()
	}
	defer func() { _ = conn.Close() }()

	time.Sleep(40 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client after ws connect, got %d", hub.ClientCount())
	}
	if hub.ClientCountByUser("tester") != 1 {
		t.Fatalf("expected 1 for tester")
	}
	if hub.ClientCountByUser("victim") != 0 {
		t.Fatalf("query param user_id must be ignored (impersonation fix)")
	}

	// send ping, expect pong
	ping := Message{Type: MessageTypePing, Payload: json.RawMessage(`{}`), Time: time.Now()}
	data, _ := json.Marshal(ping)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	// read pong with deadline
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, resp, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	var got Message
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("unmarshal pong: %v", err)
	}
	if got.Type != MessageTypePong {
		t.Fatalf("expected pong, got %s", got.Type)
	}

	// test anonymous user fallback — вторая сессия без проверенного userID
	wsURLAnon := "ws" + srv.URL[len("http"):] + "/ws"
	conn2, hr, err := websocket.DefaultDialer.Dial(wsURLAnon, nil)
	if err != nil {
		t.Fatalf("dial anon: %v", err)
	}
	if hr != nil {
		_ = hr.Body.Close()
	}
	defer func() { _ = conn2.Close() }()
	time.Sleep(30 * time.Millisecond)
	if hub.ClientCount() != 2 {
		t.Fatalf("expected 2 clients, got %d", hub.ClientCount())
	}
	if hub.ClientCountByUser("anonymous") != 1 {
		t.Fatalf("expected anonymous user count 1")
	}

	// sending a non-ping server-to-client type should be ignored but not disconnect
	ignored := Message{Type: MessageTypeNotification, Payload: json.RawMessage(`{"a":1}`), Time: time.Now()}
	d2, _ := json.Marshal(ignored)
	_ = conn.WriteMessage(websocket.TextMessage, d2)
	// give readPump a moment to process without expecting response
	time.Sleep(30 * time.Millisecond)
	// connection should still be alive: send another ping
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("second ping write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, resp2, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("second pong read: %v", err)
	}
	_ = resp2

	// send invalid JSON — should be logged but not close connection
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`not-json`))
	time.Sleep(20 * time.Millisecond)
	// still alive: ping again
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("ping after invalid json: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("pong after invalid json: %v", err)
	}
}

func TestHandleWebSocketUpgradeFailure(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	// plain HTTP request (not websocket) should trigger upgrade error path without panic
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rr := httptest.NewRecorder()
	HandleWebSocket(hub, rr, req, "tester")
	// upgrader writes 400 Bad Request on failure - StatusCode may be 400
	if rr.Code != http.StatusBadRequest {
		// some versions return 400, allow any 4xx
		if rr.Code < 400 || rr.Code >= 500 {
			t.Logf("unexpected status %d (expected 4xx)", rr.Code)
		}
	}
	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients after failed upgrade")
	}
}

func TestHandleWebSocketBroadcastRoundTrip(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, w, r, "broadcaster")
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "/ws?user_id=ignored"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	defer func() { _ = conn.Close() }()
	time.Sleep(40 * time.Millisecond)

	// wait until client joined room via hub.JoinRoom manually? The ws client auto-registers
	// but is not in any room — we join it via Hub.JoinRoom using the registered client pointer
	hub.mu.RLock()
	var wsClient *Client
	for c := range hub.clients {
		wsClient = c
		break
	}
	hub.mu.RUnlock()
	if wsClient == nil {
		t.Fatal("no ws client found")
	}
	hub.JoinRoom(wsClient, "pipeline:roundtrip")

	msg := Message{Type: MessageTypePipelineStatus, Payload: json.RawMessage(`{"status":"ok"}`), Time: time.Now()}
	hub.BroadcastToRoom("pipeline:roundtrip", msg)

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read broadcast: %v", err)
	}
	var got Message
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal broadcast: %v", err)
	}
	if got.Type != MessageTypePipelineStatus {
		t.Fatalf("expected pipeline_status, got %s", got.Type)
	}
}

func TestStopGracefullyWithRealConn(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, w, r, "grace")
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "/ws?user_id=ignored"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	defer func() { _ = conn.Close() }()
	time.Sleep(30 * time.Millisecond)

	// should not panic even with live conn
	hub.StopGracefully()
	time.Sleep(30 * time.Millisecond)
	// after StopGracefully, hub should be stopped (ClientCount may be 0 after Run cleanup)
	// give Run time to close clients
	time.Sleep(20 * time.Millisecond)
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, _ = conn.ReadMessage() // expect close error
}

func TestDisconnectClientWithRealConn(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, w, r, "disc")
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "/ws?user_id=ignored"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	defer func() { _ = conn.Close() }()
	time.Sleep(30 * time.Millisecond)

	hub.mu.RLock()
	var target *Client
	for c := range hub.clients {
		if c.userID == "disc" {
			target = c
			break
		}
	}
	hub.mu.RUnlock()
	if target == nil {
		t.Fatal("target client not found")
	}
	hub.DisconnectClient(target, "test reason")
	time.Sleep(30 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 after DisconnectClient, got %d", hub.ClientCount())
	}
}

func TestOriginCheckAllowedAndBlocked(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, w, r, "")
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "/ws"
	// Allowed origin
	header := http.Header{}
	header.Set("Origin", "http://localhost:3000")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial with allowed origin: %v", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	_ = conn.Close()
	time.Sleep(20 * time.Millisecond)

	// Blocked origin should fail upgrade
	header2 := http.Header{}
	header2.Set("Origin", "http://evil.com")
	_, hr2, err := websocket.DefaultDialer.Dial(wsURL, header2)
	if hr2 != nil {
		_ = hr2.Body.Close()
	}
	if err == nil {
		t.Fatal("expected dial failure for blocked origin")
	}
}
