// Package ws реализует WebSocket для real-time обновлений (ENG-WS-0001).
// Использует gorilla/websocket для двустороннего общения.
package ws

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	domevents "stairplatform/internal/domain/events"
)

// OriginChecker возвращает функцию проверки Origin для gorilla Upgrader.
// Ориджины приходят из конфига (STAIR_WS_ORIGINS / STAIR_CORS_ORIGINS),
// hardcoded-список localhost убран (S-112, WS-ORIGIN-HARDCODED-LOCALHOST).
// Политика (безопасный дефолт):
//   - отсутствие Origin разрешено — не-браузерные клиенты (серверные/CLI;
//     браузеры всегда шлют Origin на WS-handshake);
//   - пустой allowedOrigins — только same-origin (Origin host == Host запроса);
//   - непустой список — exact-match, "*" или wildcard "*.domain"
//     (симметрично CORS-политике isOriginAllowed).
func OriginChecker(allowedOrigins []string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // не-браузерный клиент
		}
		if len(allowedOrigins) == 0 {
			return sameOrigin(r, origin)
		}
		return originAllowed(origin, allowedOrigins)
	}
}

// sameOrigin сравнивает host Origin с Host'ом запроса (same-origin дефолт).
func sameOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// originAllowed проверяет origin по списку (exact, "*", "*.suffix").
func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
		if strings.HasPrefix(a, "*.") && strings.HasSuffix(origin, a[1:]) {
			return true
		}
	}
	return false
}

// MessageType — тип WebSocket сообщения.
type MessageType string

const (
	MessageTypePipelineStatus    MessageType = "pipeline_status"
	MessageTypeAnalysisProgress  MessageType = "analysis_progress"
	MessageTypeDocumentGenerated MessageType = "document_generated"
	MessageTypeNotification      MessageType = "notification"
	MessageTypePing              MessageType = "ping"
	MessageTypePong              MessageType = "pong"
	// S-132a: client→server управление подписками на комнаты.
	MessageTypeSubscribe    MessageType = "subscribe"
	MessageTypeUnsubscribe  MessageType = "unsubscribe"
	MessageTypeSubscribed   MessageType = "subscribed"
	MessageTypeUnsubscribed MessageType = "unsubscribed"
)

// roomPayload — тело subscribe/unsubscribe (S-132a).
type roomPayload struct {
	Room string `json:"room"`
}

// pipelineRoomPrefix — единственный разрешённый префикс комнат:
// клиенты подписываются только на pipeline-комнаты конфигов
// ("pipeline:<uuid>"). Clip: знание UUID = capability (S-132a);
// per-config authorization — follow-up S-132c.
const pipelineRoomPrefix = "pipeline:"

var uuidSuffix = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validPipelineRoom проверяет комнату: префикс pipeline: + UUID-суффикс.
// Остальное (в т.ч. произвольные имена) отклоняется — защита от room-spray
// и подписки на чужие каналы перебором имён.
func validPipelineRoom(room string) bool {
	id, ok := strings.CutPrefix(room, pipelineRoomPrefix)
	if !ok || id == "" {
		return false
	}
	return uuidSuffix.MatchString(id)
}

// Message — структура WebSocket сообщения.
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Time    time.Time       `json:"time"`
}

// Client — WebSocket клиент.
type Client struct {
	conn       *websocket.Conn
	send       chan []byte
	hub        *Hub
	userID     string
	ip         string
	role       string
	authorizer SubscriberAuthorizer
	rooms      map[string]bool
	mu         sync.RWMutex
	// msgCount/msgWindowStart — счётчик входящих сообщений в скользящем
	// окне (S-147, флуд-контроль): превышение закрывает соединение.
	msgCount       int
	msgWindowStart time.Time
}

// SubscriberAuthorizer проверяет право клиента подписаться на комнату
// (S-132c). Реализация живёт в transport/http (где есть доступ к сервисам);
// здесь — только порт. Nil-авторизатор трактуется как «нет проверки» —
// ранее подключённые клиенты не ломаются, а прод wiring задаёт проверку.
type SubscriberAuthorizer interface {
	// CanSubscribe возвращает true, если userID (роль role) может
	// подписаться на комнату room.
	CanSubscribe(userID, role, room string) bool
}

// ConnectOption настраивает Client при подключении.
type ConnectOption func(*Client)

// WithRole задаёт роль пользователя (нужна S-132c: admin пропускается
// без surplus проверки в реализациях авторизатора).
func WithRole(role string) ConnectOption {
	return func(c *Client) { c.role = role }
}

// WithSubscriberAuthorizer подключает проверку прав на подписки (S-132c).
func WithSubscriberAuthorizer(a SubscriberAuthorizer) ConnectOption {
	return func(c *Client) { c.authorizer = a }
}

// Лимиты WebSocket (S-147, S-141 №10, CWE-400): upgrade плодит pumps +
// hub-записи без бюджета на клиента — флуд upgrade с валидной сессией
// растит горутины/память хаба. Cap'ы щедрые (легитимный клиент — вкладки
// браузера, единицы коннектов), но конечные.
const (
	// defaultMaxWSConnsPerUser — максимум одновременных коннектов одного userID.
	defaultMaxWSConnsPerUser = 32
	// defaultMaxWSConnsPerIP — максимум одновременных коннектов с одного IP
	// (RemoteAddr-only, симметрично EDR-0014 §3.2.1).
	defaultMaxWSConnsPerIP = 128
	// maxWSMessagesPerWindow — максимум входящих сообщений от клиента за окно.
	maxWSMessagesPerWindow = 200
	// wsMessageWindow — окно флуд-контроля входящих сообщений.
	wsMessageWindow = time.Minute
)

// Hub — центральный хаб для управления клиентами.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	rooms      map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	stop       chan struct{}
	mu         sync.RWMutex
	// maxPerUser/maxPerIP — cap'ы коннектов (S-147); 0 = без лимита
	// (только для тестов, в проде всегда заданы дефолты).
	maxPerUser int
	maxPerIP   int
}

// HubOption настраивает Hub при создании.
type HubOption func(*Hub)

// WithMaxConnsPerUser задаёт cap коннектов на userID.
func WithMaxConnsPerUser(n int) HubOption {
	return func(h *Hub) { h.maxPerUser = n }
}

// WithMaxConnsPerIP задаёт cap коннектов на IP.
func WithMaxConnsPerIP(n int) HubOption {
	return func(h *Hub) { h.maxPerIP = n }
}

// NewHub создаёт новый хаб (с дефолтными лимитами S-147).
func NewHub(opts ...HubOption) *Hub {
	h := &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		stop:       make(chan struct{}),
		maxPerUser: defaultMaxWSConnsPerUser,
		maxPerIP:   defaultMaxWSConnsPerIP,
	}
	for _, o := range opts {
		o(h)
	}
	return h
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
			if h.overConnLimit(client) {
				// S-147 (S-141 №10): cap исчерпан — upgrade уже выполнен,
				// поэтому рвём соединение и не регистрируем: pumps клиента
				// завершатся (writePump — на закрытом send, readPump — на
				// закрытом conn), горутины/память не растут. Close-кадр
				// отсюда НЕ пишем: конкурентный WriteMessage с writePump —
				// паника gorilla (один writer на conn).
				log.Printf("WS: connection rejected: over limit (user=%s ip=%s)", client.userID, client.ip)
				close(client.send)
				if client.conn != nil {
					_ = client.conn.Close()
				}
				continue
			}
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
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
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

// ClientCountByIP возвращает количество клиентов с IP (S-147).
func (h *Hub) ClientCountByIP(ip string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for client := range h.clients {
		if client.ip == ip {
			count++
		}
	}
	return count
}

// overConnLimit — исчерпан ли cap коннектов для клиента (S-147, S-141
// №10): лимит на userID или на IP. Пустой IP не считается (юнит-клиенты
// без сети); 0-лимит = без ограничения. Небольшой TOCTOU между проверкой
// и регистрацией допустим: задача cap'а — сдержать флуд порядков, а не
// дать строгую семафорную гарантию.
func (h *Hub) overConnLimit(c *Client) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.maxPerUser > 0 {
		n := 0
		for client := range h.clients {
			if client.userID == c.userID {
				n++
			}
		}
		if n >= h.maxPerUser {
			return true
		}
	}
	if h.maxPerIP > 0 && c.ip != "" {
		n := 0
		for client := range h.clients {
			if client.ip == c.ip {
				n++
			}
		}
		if n >= h.maxPerIP {
			return true
		}
	}
	return false
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
// userID должен быть установлен вызывающей стороной из верифицированного
// токена (аутентифицированным обработчиком) — никогда не берётся из запроса,
// иначе клиент сможет подписаться на комнаты/уведомления другого пользователя.
// opts (WithRole / WithSubscriberAuthorizer, S-132c) — опциональны: без них
// подключение работает, но подписки на комнаты не проверяются.
func HandleWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request, userID string, allowedOrigins []string, opts ...ConnectOption) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     OriginChecker(allowedOrigins),
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS: upgrade error: %v", err)
		return
	}

	if userID == "" {
		userID = "anonymous"
	}

	client := &Client{
		conn:   conn,
		send:   make(chan []byte, 256),
		hub:    hub,
		userID: userID,
		ip:     remoteIP(r),
		rooms:  make(map[string]bool),
	}
	for _, o := range opts {
		o(client)
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

// remoteIP извлекает IP из RemoteAddr запроса (S-147): только host-часть,
// без порта. Доверяем только RemoteAddr (XFF не читаем — симметрично
// EDR-0014 §3.2.1 и S-112; за L7-прокси это IP балансировщика, cap всё
// равно ограничивает суммарный флуд).
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// allowMessage — флуд-контроль входящих сообщений (S-147, S-141 №10):
// не более maxWSMessagesPerWindow за wsMessageWindow. Превышение → false
// (readPump закрывает соединение). Чистая от conn — unit-тестируема.
func (c *Client) allowMessage() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if now.Sub(c.msgWindowStart) >= wsMessageWindow {
		c.msgWindowStart = now
		c.msgCount = 0
	}
	c.msgCount++
	return c.msgCount <= maxWSMessagesPerWindow
}

// readPump читает сообщения от клиента.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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

		// S-147 (S-141 №10): флуд входящих сообщений — дроп соединения
		// (defer закроет conn + снимет регистрацию). Close-кадр отсюда НЕ
		// пишем: writer на conn — только writePump (конкурентный
		// WriteMessage — паника gorilla); клиент увидит abnormal closure.
		if !c.allowMessage() {
			log.Printf("WS: message rate exceeded, closing connection (user=%s)", c.userID)
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("WS: unmarshal error: %v", err)
			continue
		}

		c.handleClientMessage(msg)
	}
}

// handleClientMessage обрабатывает одно сообщение от клиента (S-132a:
// выделено из readPump ради unit-тестируемости без живого conn).
func (c *Client) handleClientMessage(msg Message) {
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
	case MessageTypeSubscribe, MessageTypeUnsubscribe:
		c.handleSubscription(msg)
	case MessageTypePipelineStatus, MessageTypeAnalysisProgress, MessageTypeDocumentGenerated,
		MessageTypeNotification, MessageTypePong,
		MessageTypeSubscribed, MessageTypeUnsubscribed:
		// Server-to-client события обрабатываются в EventBridge; клиентские
		// сообщения этих типов игнорируем.
	default:
		log.Printf("WS: unknown message type %q from user %s", msg.Type, c.userID)
	}
}

// handleSubscription — подписка/отписка клиента на pipeline-комнату (S-132a).
// Невалидная комната игнорируется (без ack); успех подтверждается ack,
// чтобы фронтенд-клиент знал момент готовности (S-132b).
// Подписка проходит проверку авторзации (S-132c): если авторизатор задан,
// доступ без права → отказ без ack (клиент остаётся вне комнаты).
func (c *Client) handleSubscription(msg Message) {
	var p roomPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil || !validPipelineRoom(p.Room) {
		log.Printf("WS: rejected subscription request type=%q from user %s", msg.Type, c.userID)
		return
	}
	if msg.Type == MessageTypeSubscribe && c.authorizer != nil && !c.authorizer.CanSubscribe(c.userID, c.role, p.Room) {
		log.Printf("WS: denied subscribe to %q for user %s (role %s)", p.Room, c.userID, c.role)
		return
	}
	ack := Message{Time: time.Now(), Payload: mustRoomPayload(p.Room)}
	if msg.Type == MessageTypeSubscribe {
		c.hub.JoinRoom(c, p.Room)
		ack.Type = MessageTypeSubscribed
	} else {
		c.hub.LeaveRoom(c, p.Room)
		ack.Type = MessageTypeUnsubscribed
	}
	if data, err := json.Marshal(ack); err == nil {
		c.send <- data
	}
}

func mustRoomPayload(room string) json.RawMessage {
	data, _ := json.Marshal(roomPayload{Room: room})
	return data
}

// writePump отправляет сообщения клиенту.
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// PipelineStatusPayload — payload для обновления статуса pipeline.
type PipelineStatusPayload struct {
	ConfigID string `json:"configId"`
	Status   string `json:"status"`
	Stage    string `json:"stage"`
	Progress int    `json:"progress"`
	Error    string `json:"error,omitempty"`
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
