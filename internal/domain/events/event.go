// Package events реализует Event-Driven Architecture (ARCH-0012).
// Доменные события — единственный механизм уведомления других bounded contexts.
// Домен не зависит от HTTP, БД, ORM, UI, AI (ADR-0006).
package events

import (
	"crypto/rand"
	"fmt"
	"time"
)

// EventVersion — версия схемы события для обратной совместимости.
type EventVersion int

const CurrentVersion EventVersion = 1

// EventType — тип доменного события (уникальный строковый идентификатор).
type EventType string

// EventMetadata — стандартный конверт метаданных для всех событий (ARCH-0012 §3).
type EventMetadata struct {
	ID            string       // UUIDv7 — уникальный идентификатор события
	Type          EventType    // тип события
	CorrelationID string       // цепочка вызовов (трассировка через pipeline)
	CausationID   string       // ID события-причины (что породило это событие)
	Timestamp     time.Time    // UTC время создания
	Version       EventVersion // версия схемы события
	Source        string       // какой engine/domain создал событие
	Actor         string       // кто инициировал действие (user ID, system)
	Tenant        string       // tenant ID для мульти-тенантности
}

// Event — интерфейс доменного события (DOM-0007).
// Каждое событие immutable, неизменяемо после создания.
type Event interface {
	EventType() EventType
	Metadata() EventMetadata
}

// BaseEvent — базовая реализация Event с полным envelope метаданных.
// Используется как embedded struct в конкретных событиях.
type BaseEvent struct {
	Meta EventMetadata
}

func (e BaseEvent) EventType() EventType    { return e.Meta.Type }
func (e BaseEvent) Metadata() EventMetadata { return e.Meta }

// NewEventMetadata создаёт EventMetadata с заполненными ID и Timestamp.
func NewEventMetadata(typ EventType, source, actor, tenant, correlationID, causationID string) EventMetadata {
	return EventMetadata{
		ID:            generateID(),
		Type:          typ,
		CorrelationID: correlationID,
		CausationID:   causationID,
		Timestamp:     time.Now().UTC(),
		Version:       CurrentVersion,
		Source:        source,
		Actor:         actor,
		Tenant:        tenant,
	}
}

// NewCorrelationID генерирует новый CorrelationID для трассировки pipeline.
func NewCorrelationID() string {
	return generateID()
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
