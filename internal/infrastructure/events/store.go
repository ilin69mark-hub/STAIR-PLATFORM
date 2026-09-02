package events

import (
	"sync"
	"time"

	domevents "stairplatform/internal/domain/events"
)

// StoreEntry — запись в Event Store (append-only log).
type StoreEntry struct {
	Event     domevents.Event
	StoredAt  time.Time
	ReplayID  uint64 // порядковый номер для replay
}

// Store — append-only лог доменных событий с replay capability.
// Потокобезопасен. Zero value непригоден — используйте NewStore.
type Store struct {
	mu      sync.RWMutex
	entries []StoreEntry
	counter uint64
	capacity int
}

// NewStore создаёт Store с заданной ёмкостью (0 = без ограничений).
func NewStore(capacity int) *Store {
	return &Store{
		entries:  make([]StoreEntry, 0, capacity),
		capacity: capacity,
	}
}

// Append добавляет событие в store.
func (s *Store) Append(event domevents.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.capacity > 0 && len(s.entries) >= s.capacity {
		s.entries = s.entries[1:]
	}

	s.counter++
	s.entries = append(s.entries, StoreEntry{
		Event:    event,
		StoredAt: time.Now().UTC(),
		ReplayID: s.counter,
	})
}

// Len возвращает количество записей в store.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Events возвращает все события определённого типа.
func (s *Store) Events(typ domevents.EventType) []StoreEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []StoreEntry
	for _, e := range s.entries {
		if e.Event.EventType() == typ {
			out = append(out, e)
		}
	}
	return out
}

// All возвращает все записи (для replay).
func (s *Store) All() []StoreEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]StoreEntry, len(s.entries))
	copy(out, s.entries)
	return out
}

// ReplaySince replayит события с указанного ReplayID через callback.
// Используется для восстановления состояния из Event Store.
func (s *Store) ReplaySince(sinceReplayID uint64, fn func(StoreEntry) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, entry := range s.entries {
		if entry.ReplayID <= sinceReplayID {
			continue
		}
		if !fn(entry) {
			break
		}
	}
}

// ReplayAll replayит все события через callback.
func (s *Store) ReplayAll(fn func(StoreEntry) bool) {
	s.ReplaySince(0, fn)
}

// Clear очищает store.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = s.entries[:0]
	s.counter = 0
}
