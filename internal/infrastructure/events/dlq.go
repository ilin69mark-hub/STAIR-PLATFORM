package events

import (
	"context"
	"log/slog"
	"sync"
	"time"

	domevents "stairplatform/internal/domain/events"
)

// DLQEntry — запись в Dead Letter Queue (failed event + handler + error).
type DLQEntry struct {
	Event         domevents.Event
	HandlerID     string
	Error         error
	CorrelationID string
	Timestamp     time.Time
	RetryCount    int
}

// DLQ — Dead Letter Queue для failed event handlers.
// Хранит записи для последующего retry или анализа.
type DLQ struct {
	mu      sync.RWMutex
	entries []DLQEntry
	capacity int
	logger  *slog.Logger
}

// NewDLQ создаёт DLQ с заданной ёмкостью.
func NewDLQ(capacity int) *DLQ {
	if capacity <= 0 {
		capacity = 1000
	}
	return &DLQ{
		entries:  make([]DLQEntry, 0, capacity),
		capacity: capacity,
		logger:   slog.Default(),
	}
}

// Push добавляет запись в DLQ. При переполнении — удаляет самую старую.
func (q *DLQ) Push(entry DLQEntry) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.entries) >= q.capacity {
		// FIFO: удаляем самый старый.
		q.entries = q.entries[1:]
	}
	q.entries = append(q.entries, entry)
}

// Pop извлекает последнюю (самую новую) запись из DLQ.
func (q *DLQ) Pop() (DLQEntry, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.entries) == 0 {
		return DLQEntry{}, false
	}
	entry := q.entries[len(q.entries)-1]
	q.entries = q.entries[:len(q.entries)-1]
	return entry, true
}

// Len возвращает количество записей в DLQ.
func (q *DLQ) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.entries)
}

// Entries возвращает копию всех записей (для анализа).
func (q *DLQ) Entries() []DLQEntry {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]DLQEntry, len(q.entries))
	copy(out, q.entries)
	return out
}

// Clear очищает DLQ.
func (q *DLQ) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.entries = q.entries[:0]
}

// RetryAll повторно обрабатывает все записи из DLQ через Bus.
// Возвращает количество успешно обработанных записей.
func (q *DLQ) RetryAll(ctx context.Context, bus *Bus) int {
	q.mu.Lock()
	entries := make([]DLQEntry, len(q.entries))
	copy(entries, q.entries)
	q.mu.Unlock()

	successCount := 0
	var remaining []DLQEntry

	for _, entry := range entries {
		if ctx.Err() != nil {
			remaining = append(remaining, entry)
			continue
		}

		// Повторно публикуем событие (handler уже известен из entry).
		// Используем оригинальный handler — простой повторный вызов.
		err := bus.invokeWithRetry(ctx, handlerEntry{id: entry.HandlerID}, entry.Event)
		if err != nil {
			entry.RetryCount++
			remaining = append(remaining, entry)
			q.logger.Warn("DLQ retry failed",
				"event_type", entry.Event.EventType(),
				"handler_id", entry.HandlerID,
				"retry_count", entry.RetryCount,
				"error", err,
			)
		} else {
			successCount++
			q.logger.Info("DLQ retry succeeded",
				"event_type", entry.Event.EventType(),
				"handler_id", entry.HandlerID,
			)
		}
	}

	// Сохраняем оставшиеся записи.
	q.mu.Lock()
	q.entries = remaining
	q.mu.Unlock()

	return successCount
}
