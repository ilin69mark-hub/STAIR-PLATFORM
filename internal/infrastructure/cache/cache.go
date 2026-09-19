// Package cache реализует in-memory TTL кэш для часто запрашиваемых данных.
package cache

import (
	"sync"
	"time"
)

// Entry — запись в кэше с TTL.
type Entry[T any] struct {
	Value     T
	ExpiresAt time.Time
}

// Cache — потокобезопасный in-memory кэш с LRU eviction.
type Cache[T any] struct {
	mu         sync.RWMutex
	items      map[string]Entry[T]
	maxSize    int
	defaultTTL time.Duration
}

// New создаёт новый кэш с указанным maxSize и TTL.
func New[T any](maxSize int, defaultTTL time.Duration) *Cache[T] {
	return &Cache[T]{
		items:      make(map[string]Entry[T]),
		maxSize:    maxSize,
		defaultTTL: defaultTTL,
	}
}

// Get возвращает значение из кэша. Если ключ не найден или истёк TTL — возвращает zero value и false.
func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		var zero T
		return zero, false
	}
	return entry.Value, true
}

// Set добавляет/обновляет значение в кэше с дефолтным TTL.
func (c *Cache[T]) Set(key string, value T) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL добавляет/обновляет значение с указанным TTL.
func (c *Cache[T]) SetWithTTL(key string, value T, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Если кэш полон — удаляем самую старую запись
	if len(c.items) >= c.maxSize && !c.exists(key) {
		c.evictOldest()
	}

	c.items[key] = Entry[T]{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Delete удаляет ключ из кэша.
func (c *Cache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// InvalidatePrefix удаляет все ключи с указанным префиксом.
func (c *Cache[T]) InvalidatePrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.items {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.items, key)
		}
	}
}

// Clear очищает весь кэш.
func (c *Cache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]Entry[T])
}

// Size возвращает количество записей в кэше.
func (c *Cache[T]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats возвращает статистику кэша.
func (c *Cache[T]) Stats() (size, maxSize int, hitRate float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items), c.maxSize, 0
}

func (c *Cache[T]) exists(key string) bool {
	_, ok := c.items[key]
	return ok
}

// evictOldest удаляет самую старую запись (по ExpiresAt).
// Должен вызываться под мьютексом.
func (c *Cache[T]) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.items {
		if oldestKey == "" || entry.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.ExpiresAt
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}
