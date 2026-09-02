// Package events реализует Event Bus — in-process pub/sub механизм
// для Event-Driven Architecture (ARCH-0012).
// Использует callback registry с context propagation, retry и DLQ.
package events

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	domevents "stairplatform/internal/domain/events"
)

// Handler — обработчик доменного события. context нёс CorrelationID.
type Handler func(ctx context.Context, event domevents.Event) error

// Bus — in-process Event Bus с callback registry.
// Zero value непригоден — используйте NewBus.
type Bus struct {
	mu       sync.RWMutex
	handlers map[domevents.EventType][]handlerEntry
	dlq      *DLQ
	retry    *RetryPolicy
	logger   *slog.Logger
	closed   atomic.Bool
}

type handlerEntry struct {
	id      string
	handler Handler
	priority int
}

// BusOption — опциональная настройка Bus.
type BusOption func(*Bus)

// WithLogger устанавливает логгер для Event Bus.
func WithLogger(l *slog.Logger) BusOption {
	return func(b *Bus) { b.logger = l }
}

// WithRetryPolicy устанавливает политику повторных попыток.
func WithRetryPolicy(p RetryPolicy) BusOption {
	return func(b *Bus) { b.retry = &p }
}

// WithDLQ включает Dead Letter Queue для failed events.
func WithDLQ(capacity int) BusOption {
	return func(b *Bus) { b.dlq = NewDLQ(capacity) }
}

// NewBus создаёт Event Bus с настройками по умолчанию.
func NewBus(opts ...BusOption) *Bus {
	b := &Bus{
		handlers: make(map[domevents.EventType][]handlerEntry),
		retry:    &DefaultRetryPolicy,
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Subscribe регистрирует обработчик на тип события.
// Возвращает ID подписки для отписки.
func (b *Bus) Subscribe(typ domevents.EventType, handler Handler, priority ...int) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	p := 0
	if len(priority) > 0 {
		p = priority[0]
	}

	id := domevents.NewCorrelationID()
	entry := handlerEntry{id: id, handler: handler, priority: p}
	b.handlers[typ] = append(b.handlers[typ], entry)

	// Сортировка по приоритету (меньше = раньше).
	entries := b.handlers[typ]
	for i := len(entries) - 1; i > 0 && entries[i].priority < entries[i-1].priority; i-- {
		entries[i], entries[i-1] = entries[i-1], entries[i]
	}
	b.handlers[typ] = entries

	return id
}

// Unsubscribe удаляет обработчик по ID подписки.
func (b *Bus) Unsubscribe(typ domevents.EventType, handlerID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	entries := b.handlers[typ]
	for i, e := range entries {
		if e.id == handlerID {
			b.handlers[typ] = append(entries[:i], entries[i+1:]...)
			return true
		}
	}
	return false
}

// Publish публикует событие всем подписчикам (sync, в порядке приоритета).
// Каждый handler вызывается с CorrelationID из metadata события.
// При ошибке применяется retry policy, при исчерпании — DLQ.
func (b *Bus) Publish(ctx context.Context, event domevents.Event) error {
	if b.closed.Load() {
		return fmt.Errorf("events: bus is closed")
	}

	meta := event.Metadata()
	correlationID := meta.CorrelationID
	if correlationID == "" {
		correlationID = meta.ID
	}

	b.mu.RLock()
	entries := make([]handlerEntry, len(b.handlers[event.EventType()]))
	copy(entries, b.handlers[event.EventType()])

	// Также публикуем wildcard подписчикам (подписка на все события).
	wildcards := make([]handlerEntry, len(b.handlers["*"]))
	copy(wildcards, b.handlers["*"])
	b.mu.RUnlock()

	all := append(entries, wildcards...)

	b.logger.Debug("publishing event",
		"type", event.EventType(),
		"event_id", meta.ID,
		"correlation_id", correlationID,
		"handlers", len(all),
	)

	for _, entry := range all {
		if err := b.invokeWithRetry(ctx, entry, event, correlationID); err != nil {
			if b.dlq != nil {
				b.dlq.Push(DLQEntry{
					Event:         event,
					HandlerID:     entry.id,
					Error:         err,
					CorrelationID: correlationID,
					Timestamp:     time.Now().UTC(),
				})
			}
			b.logger.Error("event handler failed (moved to DLQ)",
				"type", event.EventType(),
				"handler_id", entry.id,
				"error", err,
			)
		}
	}
	return nil
}

// PublishAsync публикует событие асинхронно (fire-and-forget).
func (b *Bus) PublishAsync(ctx context.Context, event domevents.Event) {
	go func() {
		_ = b.Publish(ctx, event)
	}()
}

func (b *Bus) invokeWithRetry(ctx context.Context, entry handlerEntry, event domevents.Event, correlationID string) error {
	var lastErr error
	maxAttempts := b.retry.MaxAttempts

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Пропускаем первую попытку (attempt=0) без задержки.
		if attempt > 0 {
			delay := b.retry.Backoff(attempt)
			b.logger.Debug("retrying event handler",
				"handler_id", entry.id,
				"attempt", attempt+1,
				"delay", delay,
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := entry.handler(ctx, event)
		if err == nil {
			return nil
		}
		lastErr = err

		// Не повторяем при отмене контекста.
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return fmt.Errorf("events: handler %s failed after %d attempts: %w",
		entry.id, maxAttempts, lastErr)
}

// Close закрывает Bus — новые публикации отклоняются.
func (b *Bus) Close() {
	b.closed.Store(true)
}

// HandlerCount возвращает количество обработчиков для типа события.
func (b *Bus) HandlerCount(typ domevents.EventType) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[typ])
}

// DLQCount возвращает количество записей в Dead Letter Queue.
func (b *Bus) DLQCount() int {
	if b.dlq == nil {
		return 0
	}
	return b.dlq.Len()
}

// DLQRetry повторно обрабатывает все записи из DLQ.
func (b *Bus) DLQRetry(ctx context.Context) int {
	if b.dlq == nil {
		return 0
	}
	return b.dlq.RetryAll(ctx, b)
}
