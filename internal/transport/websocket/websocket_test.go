package ws

import (
	"testing"
	"time"
)

func TestHubCreation(t *testing.T) {
	hub := NewHub()
	if hub == nil {
		t.Fatal("expected non-nil hub")
	}
	if hub.clients == nil {
		t.Fatal("expected non-nil clients map")
	}
	if hub.rooms == nil {
		t.Fatal("expected non-nil rooms map")
	}
}

func TestHubRun(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Создаём клиент
	client := &Client{
		send:  make(chan []byte, 256),
		hub:   hub,
		rooms: make(map[string]bool),
	}

	// Регистрируем клиента
	hub.register <- client

	// Проверяем что клиент зарегистрирован
	time.Sleep(10 * time.Millisecond)
	hub.mu.RLock()
	if len(hub.clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(hub.clients))
	}
	hub.mu.RUnlock()

	// Удаляем клиента
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)
}

func TestHubJoinLeaveRoom(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Создаём клиент
	client := &Client{
		send:  make(chan []byte, 256),
		hub:   hub,
		rooms: make(map[string]bool),
	}

	// Регистрируем клиента
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Присоединяем к комнате
	hub.JoinRoom(client, "pipeline:123")
	hub.mu.RLock()
	if len(hub.rooms["pipeline:123"]) != 1 {
		t.Fatalf("expected 1 client in room, got %d", len(hub.rooms["pipeline:123"]))
	}
	hub.mu.RUnlock()

	// Покидаем комнату
	hub.LeaveRoom(client, "pipeline:123")
	hub.mu.RLock()
	if len(hub.rooms["pipeline:123"]) != 0 {
		t.Fatalf("expected 0 clients in room, got %d", len(hub.rooms["pipeline:123"]))
	}
	hub.mu.RUnlock()
}

func TestHubBroadcastToRoom(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Создаём клиент
	client := &Client{
		send:  make(chan []byte, 256),
		hub:   hub,
		rooms: make(map[string]bool),
	}

	// Регистрируем клиента
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Присоединяем к комнате
	hub.JoinRoom(client, "pipeline:123")

	// Отправляем сообщение
	msg := Message{
		Type:    MessageTypePipelineStatus,
		Payload: []byte(`{"status":"completed"}`),
		Time:    time.Now(),
	}
	hub.BroadcastToRoom("pipeline:123", msg)

	// Проверяем что сообщение получено
	select {
	case <-client.send:
		// OK
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected message")
	}
}

func TestHubBroadcastToUser(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Создаём клиент
	client := &Client{
		send:   make(chan []byte, 256),
		hub:    hub,
		userID: "user-123",
		rooms:  make(map[string]bool),
	}

	// Регистрируем клиента
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Отправляем сообщение
	msg := Message{
		Type:    MessageTypeNotification,
		Payload: []byte(`{"message":"test"}`),
		Time:    time.Now(),
	}
	hub.BroadcastToUser("user-123", msg)

	// Проверяем что сообщение получено
	select {
	case <-client.send:
		// OK
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected message")
	}
}

func TestMessageTypes(t *testing.T) {
	msgTypes := []MessageType{
		MessageTypePipelineStatus,
		MessageTypeAnalysisProgress,
		MessageTypeDocumentGenerated,
		MessageTypeNotification,
		MessageTypePing,
		MessageTypePong,
	}

	for _, msgType := range msgTypes {
		if msgType == "" {
			t.Fatal("expected non-empty message type")
		}
	}
}

func TestPipelineStatusPayload(t *testing.T) {
	payload := PipelineStatusPayload{
		ConfigID: "config-123",
		Status:   "completed",
		Stage:    "analysis",
		Progress: 100,
	}

	if payload.ConfigID != "config-123" {
		t.Fatalf("expected configID 'config-123', got %q", payload.ConfigID)
	}
}

func TestAnalysisProgressPayload(t *testing.T) {
	payload := AnalysisProgressPayload{
		ConfigID: "config-123",
		Stage:    "completed",
		Progress: 100,
		Score:    95.5,
	}

	if payload.ConfigID != "config-123" {
		t.Fatalf("expected configID 'config-123', got %q", payload.ConfigID)
	}
}

func TestDocumentGeneratedPayload(t *testing.T) {
	payload := DocumentGeneratedPayload{
		ConfigID: "config-123",
		DocID:    "doc-456",
		Type:     "technical_spec",
		Format:   "json",
	}

	if payload.ConfigID != "config-123" {
		t.Fatalf("expected configID 'config-123', got %q", payload.ConfigID)
	}
}

func TestEventBridgeCreation(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	// EventBridge requires typed bus — integration test only
}

func TestHubStopGracefully(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	hub.StopGracefully()
	time.Sleep(10 * time.Millisecond)
}

func TestHubClientCountByUser(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	c1 := &Client{send: make(chan []byte, 256), hub: hub, userID: "user-1", rooms: make(map[string]bool)}
	c2 := &Client{send: make(chan []byte, 256), hub: hub, userID: "user-1", rooms: make(map[string]bool)}
	c3 := &Client{send: make(chan []byte, 256), hub: hub, userID: "user-2", rooms: make(map[string]bool)}

	hub.register <- c1
	hub.register <- c2
	hub.register <- c3
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCountByUser("user-1") != 2 {
		t.Errorf("expected 2 clients for user-1, got %d", hub.ClientCountByUser("user-1"))
	}
	if hub.ClientCountByUser("user-2") != 1 {
		t.Errorf("expected 1 client for user-2, got %d", hub.ClientCountByUser("user-2"))
	}
	if hub.ClientCountByUser("user-3") != 0 {
		t.Errorf("expected 0 clients for user-3, got %d", hub.ClientCountByUser("user-3"))
	}
}

func TestHubDisconnectClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := &Client{
		send:   make(chan []byte, 256),
		hub:    hub,
		userID: "user-1",
		rooms:  make(map[string]bool),
	}
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.DisconnectClient(client, "test disconnect")
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", hub.ClientCount())
	}
}
