// Package queue реализует систему фоновых заданий (EDR-0020 §3): JobQueue с
// двумя бэкендами — Redis List (распределённый) и in-memory (fallback,
// single-instance). Воркер (cmd/worker) потребляет задания через Dequeue.
package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Типы заданий (EDR-0020 §3.3). Расширяется в реестре воркера.
const (
	JobCleanupSessions  = "sys.cleanup_sessions"
	JobCleanupSsoStates = "sys.cleanup_sso_states"
	JobCleanupAudit     = "sys.cleanup_audit"
	// JobQuoteSend — доставка коммерческого предложения в ERP (EDR-0023).
	// Payload: {event_id, endpoint_id}.
	JobQuoteSend = "erp.quote_send"
)

// DefaultMaxAttempts — предел попыток задания перед окончательным отказом.
const DefaultMaxAttempts = 3

// ErrJobInvalid — задание имеет некорректные поля (не ID/типа).
var ErrJobInvalid = errors.New("queue: invalid job")

// Job — единица работы в очереди (EDR-0020 §3.1).
type Job struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	CreatedAt   time.Time       `json:"created_at"`
}

// NewJob создаёт задание с генерированным ID и дефолтным MaxAttempts.
// Payload сериализуется в JSON (nil — пустой).
func NewJob(typ string, payload any) (Job, error) {
	if typ == "" {
		return Job{}, fmt.Errorf("%w: empty type", ErrJobInvalid)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Job{}, fmt.Errorf("queue: marshal payload: %w", err)
	}
	id, err := randomID()
	if err != nil {
		return Job{}, err
	}
	return Job{
		ID:          id,
		Type:        typ,
		Payload:     raw,
		MaxAttempts: DefaultMaxAttempts,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

// randomID возвращает 16 байт hex (крипто-стойкий ID задания).
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("queue: random id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Marshal сериализует задание (для Enqueue бэкенда).
func (j Job) Marshal() ([]byte, error) {
	return json.Marshal(j)
}

// UnmarshalJob десериализует задание из байтов.
func UnmarshalJob(b []byte) (Job, error) {
	var j Job
	if err := json.Unmarshal(b, &j); err != nil {
		return Job{}, fmt.Errorf("queue: unmarshal job: %w", err)
	}
	if j.ID == "" || j.Type == "" {
		return Job{}, fmt.Errorf("%w: missing id/type", ErrJobInvalid)
	}
	if j.MaxAttempts <= 0 {
		j.MaxAttempts = DefaultMaxAttempts
	}
	return j, nil
}

// JobQueue — интерфейс очереди заданий (EDR-0020 §3.2).
type JobQueue interface {
	// Enqueue кладёт задание в очередь.
	Enqueue(ctx context.Context, job Job) error
	// Dequeue извлекает следующее задание. ok=false — очередь пуста
	// (готов взять позже).
	Dequeue(ctx context.Context) (job Job, ok bool, err error)
}

// memoryQueue — FIFO-очередь в памяти (fallback, single-instance) на мьютексе.
type memoryQueue struct {
	mu    sync.Mutex
	queue [][]byte
}

// NewMemoryQueue создаёт in-memory бэкенд (EDR-0020 §3.2).
func NewMemoryQueue() JobQueue {
	return &memoryQueue{}
}

func (q *memoryQueue) Enqueue(_ context.Context, job Job) error {
	b, err := job.Marshal()
	if err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queue = append(q.queue, b)
	return nil
}

func (q *memoryQueue) Dequeue(_ context.Context) (Job, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.queue) == 0 {
		return Job{}, false, nil
	}
	b := q.queue[0]
	q.queue = q.queue[1:]
	j, err := UnmarshalJob(b)
	if err != nil {
		return Job{}, false, err
	}
	return j, true, nil
}
