// Package ws реализует WebSocket для real-time обновлений (ENG-WS-0001).
// Использует gorilla/websocket для双向 общения.
package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	domevents "stairplatform/internal/domain/events"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Allow non-browser clients
		}
		// В проде проверять against allowed origins из config
		// Пока разрешаем localhost origins
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://localhost:8080",
		}
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
}

// MessageType — тип WebSocket сообщения.
type MessageType string

const (
	MessageTypePipelineStatus MessageType = "pipeline_status"
	MessageTypeAnalysisProgress MessageType = "analysis_progress"
	MessageTypeDocumentGenerated MessageType = "document_generated"
	MessageTypeNotification MessageType = "notification"
	MessageTypePing MessageType = "ping"
	MessageTypePong MessageType = "pong"
)

// Message — структура WebSocket сообщения.
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Time    time.Time       `json:"time"`
}

// Client — WebSocket клиент.
type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	hub    *Hub
	userID string
	rooms  map[string]bool
	mu     sync.RWMutex
}

// Hub — центральный хаб для управления клиентами.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	rooms      map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	stop       chan struct{}
	mu         sync.RWMutex
}

// NewHub создаёт новый хаб.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		stop:       make(chan struct{}),
	}
}

// Run запускает хаб.
func (h *Hub) Run() {
	for {
		select {
		case <-h.stop:
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			log.Printf("WS: hub stopped, all clients disconnected")
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WS: client connected (user=%s)", client.userID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				// Удаляем из всех комнат
				for room := range client.rooms {
					if clients, ok := h.rooms[room]; ok {
						delete(clients, client)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("WS: client disconnected (user=%s)", client.userID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Stop останавливает хаб и отключает всех клиентов.
func (h *Hub) Stop() {
	close(h.stop)
}

// StopGracefully отправляет close message всем клиентам перед отключением.
func (h *Hub) StopGracefully() {
	h.mu.RLock()
	for client := range h.clients {
		// Отправляем close message (игнорируем ошибку — клиент может быть уже отключен)
		_ = client.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"))
	}
	h.mu.RUnlock()
	h.Stop()
}

// DisconnectClient отключает конкретного клиента с close message.
func (h *Hub) DisconnectClient(client *Client, reason string) {
	if client.conn != nil {
		_ = client.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, reason))
	}
	h.unregister <- client
}

// ClientCountByUser возвращает количество клиентов для пользователя.
func (h *Hub) ClientCountByUser(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for client := range h.clients {
		if client.userID == userID {
			count++
		}
	}
	return count
}

// ClientCount возвращает количество подключенных клиентов.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// BroadcastToRoom отправляет сообщение во все клиенты в комнате.
func (h *Hub) BroadcastToRoom(room string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("WS: failed to marshal message: %v", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.rooms[room]; ok {
		for client := range clients {
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// BroadcastToUser отправляет сообщение конкретному пользователю.
func (h *Hub) BroadcastToUser(userID string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("WS: failed to marshal message: %v", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.userID == userID {
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// JoinRoom добавляет клиента в комнату.
func (h *Hub) JoinRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][client] = true
	client.mu.Lock()
	client.rooms[room] = true
	client.mu.Unlock()
}

// LeaveRoom удаляет клиента из комнаты.
func (h *Hub) LeaveRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.rooms[room]; ok {
		delete(clients, client)
	}
	client.mu.Lock()
	delete(client.rooms, room)
	client.mu.Unlock()
}

// HandleWebSocket обрабатывает WebSocket подключения.
func HandleWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS: upgrade error: %v", err)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	client := &Client{
		conn:   conn,
		send:   make(chan []byte, 256),
		hub:    hub,
		userID: userID,
		rooms:  make(map[string]bool),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

// readPump читает сообщения от клиента.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS: read error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("WS: unmarshal error: %v", err)
			continue
		}

		// Обработка ping/pong
		switch msg.Type {
		case MessageTypePing:
			pong := Message{
				Type:    MessageTypePong,
				Payload: json.RawMessage(`{}`),
				Time:    time.Now(),
			}
			data, _ := json.Marshal(pong)
			c.send <- data
		}
	}
}

// writePump отправляет сообщения клиенту.
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// PipelineStatusPayload — payload для обновления статуса pipeline.
type PipelineStatusPayload struct {
	ConfigID   string `json:"configId"`
	Status     string `json:"status"`
	Stage      string `json:"stage"`
	Progress   int    `json:"progress"`
	Error      string `json:"error,omitempty"`
}

// AnalysisProgressPayload — payload для обновления анализа.
type AnalysisProgressPayload struct {
	ConfigID string  `json:"configId"`
	Stage    string  `json:"stage"`
	Progress int     `json:"progress"`
	Score    float64 `json:"score,omitempty"`
}

// DocumentGeneratedPayload — payload для уведомления о документе.
type DocumentGeneratedPayload struct {
	ConfigID string `json:"configId"`
	DocID    string `json:"docId"`
	Type     string `json:"type"`
	Format   string `json:"format"`
}

// EventBridge связывает Event Bus с WebSocket Hub.
type EventBridge struct {
	bus interface {
		Subscribe(typ domevents.EventType, handler func(ctx context.Context, event domevents.Event) error) string
	}
	hub *Hub
}

// NewEventBridge создаёт новый EventBridge.
func NewEventBridge(bus interface {
	Subscribe(typ domevents.EventType, handler func(ctx context.Context, event domevents.Event) error) string
}, hub *Hub) *EventBridge {
	return &EventBridge{
		bus: bus,
		hub: hub,
	}
}

// Start запускает bridge.
func (b *EventBridge) Start() {
	// Подписываемся на события pipeline
	b.bus.Subscribe(domevents.EventPipelineCompleted, func(ctx context.Context, e domevents.Event) error {
		configID := ""
		if meta := e.Metadata(); meta.ID != "" {
			configID = meta.ID
		}
		payload, _ := json.Marshal(PipelineStatusPayload{
			ConfigID: configID,
			Status:   "completed",
			Stage:    "completed",
			Progress: 100,
		})
		msg := Message{
			Type:    MessageTypePipelineStatus,
			Payload: payload,
			Time:    time.Now(),
		}
		b.hub.BroadcastToRoom("pipeline:"+configID, msg)
		return nil
	})

	// Подписываемся на события анализа
	b.bus.Subscribe(domevents.EventAnalysisCompleted, func(ctx context.Context, e domevents.Event) error {
		configID := ""
		if meta := e.Metadata(); meta.ID != "" {
			configID = meta.ID
		}
		payload, _ := json.Marshal(AnalysisProgressPayload{
			ConfigID: configID,
			Stage:    "completed",
			Progress: 100,
		})
		msg := Message{
			Type:    MessageTypeAnalysisProgress,
			Payload: payload,
			Time:    time.Now(),
		}
		b.hub.BroadcastToRoom("pipeline:"+configID, msg)
		return nil
	})

	// Подписываемся на события документа
	b.bus.Subscribe(domevents.EventDocumentGenerated, func(ctx context.Context, e domevents.Event) error {
		configID := ""
		if meta := e.Metadata(); meta.ID != "" {
			configID = meta.ID
		}
		payload, _ := json.Marshal(DocumentGeneratedPayload{
			ConfigID: configID,
			DocID:    configID,
			Type:     "document",
			Format:   "json",
		})
		msg := Message{
			Type:    MessageTypeDocumentGenerated,
			Payload: payload,
			Time:    time.Now(),
		}
		b.hub.BroadcastToRoom("pipeline:"+configID, msg)
		return nil
	})
}
