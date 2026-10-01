package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// waitClientCount ждёт, пока ClientCount станет равен 2 (или таймаут) — без
// flaky-sleep на каждую регистрацию.
func waitClientCount(t *testing.T, hub *Hub) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hub.ClientCount() == 2 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("ClientCount = %d, want 2", hub.ClientCount())
}

// TestHubConnCapPerUser — ловушка S-141 №10: N+1 коннект того же userID
// отклоняется (не регистрируется, send закрыт).
func TestHubConnCapPerUser(t *testing.T) {
	hub := NewHub(WithMaxConnsPerUser(2))
	go hub.Run()
	defer hub.Stop()

	mk := func() *Client {
		return &Client{send: make(chan []byte, 2), hub: hub, userID: "flooder", ip: "10.9.0.1", rooms: make(map[string]bool)}
	}
	c1, c2, c3 := mk(), mk(), mk()
	hub.register <- c1
	hub.register <- c2
	waitClientCount(t, hub)

	hub.register <- c3
	// Отклонение сигнализируется закрытием send; ждём его (не сон).
	closed := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !closed {
		select {
		case _, ok := <-c3.send:
			if !ok {
				closed = true
			} else {
				t.Fatal("rejected client must not receive messages")
			}
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !closed {
		t.Fatal("rejected client send must be closed promptly")
	}
	if hub.ClientCount() != 2 {
		t.Fatalf("ClientCount = %d, want 2 (3rd connection must be rejected)", hub.ClientCount())
	}
}

// TestHubConnCapPerIP — cap на IP: третий клиент с того же IP (другие
// userID) отклоняется.
func TestHubConnCapPerIP(t *testing.T) {
	hub := NewHub(WithMaxConnsPerIP(2))
	go hub.Run()
	defer hub.Stop()

	mk := func(user string) *Client {
		return &Client{send: make(chan []byte, 2), hub: hub, userID: user, ip: "10.9.0.7", rooms: make(map[string]bool)}
	}
	hub.register <- mk("u1")
	hub.register <- mk("u2")
	waitClientCount(t, hub)

	c3 := mk("u3")
	hub.register <- c3
	waitClientCount(t, hub) // не должен вырасти
	if hub.ClientCountByIP("10.9.0.7") != 2 {
		t.Fatalf("ClientCountByIP = %d, want 2", hub.ClientCountByIP("10.9.0.7"))
	}
}

// TestClientMessageFlood — ловушка S-141 №10: флуд входящих сообщений —
// первые maxWSMessagesPerWindow проходят, дальше drop (false).
func TestClientMessageFlood(t *testing.T) {
	c := &Client{rooms: make(map[string]bool)}
	for i := 0; i < maxWSMessagesPerWindow; i++ {
		if !c.allowMessage() {
			t.Fatalf("message %d must be allowed", i+1)
		}
	}
	if c.allowMessage() {
		t.Fatalf("message %d must be dropped (flood)", maxWSMessagesPerWindow+1)
	}
}

// TestHandleWebSocketConnCapReject — live-вариант ловушки №10: третий
// upgrade того же userID закрывается сервером (1008), в хабе остаются 2.
func TestHandleWebSocketConnCapReject(t *testing.T) {
	hub := NewHub(WithMaxConnsPerUser(2))
	go hub.Run()
	defer hub.Stop()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, w, r, "live-flooder", nil)
	}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	dial := func() *websocket.Conn {
		t.Helper()
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return conn
	}
	c1, c2 := dial(), dial()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()
	waitClientCount(t, hub)

	c3 := dial()
	defer func() { _ = c3.Close() }()
	// Handshake проходит (upgrade уже выполнен), затем сервер закрывает:
	// чтение должно быстро вернуть ошибку закрытия.
	_ = c3.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, _, err := c3.ReadMessage(); err == nil {
		t.Fatal("3rd connection must be closed by server (limit exceeded)")
	}
	if n := hub.ClientCountByUser("live-flooder"); n != 2 {
		t.Fatalf("ClientCountByUser = %d, want 2", n)
	}
}
