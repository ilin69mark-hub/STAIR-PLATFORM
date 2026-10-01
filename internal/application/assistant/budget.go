package assistant

import (
	"sync"
	"time"
)

// Budget — глобальный дневной лимит попыток LLM-вызовов (S-148, S-141 №14,
// OWASP LLM10/CWE-400): per-user лимит (200 req/min) обходится
// мультиаккаунтами free-tier, месячного/глобального cap на расход OpenRouter
// нет. Превышение → только локальный бэкенд (circuit breaker) + метрика
// stair_assistant_llm_budget_denied_total (источник для алерта).
// Учёт — попытки вызова primary (консервативный прокси биллинга: попытка
// может упасть и уйти в local, но токены эмбеддера/ретраи уже потрачены).
// max <= 0 = без лимита. Потокобезопасен.
type Budget struct {
	max  int64
	mu   sync.Mutex
	day  string
	used int64
	now  func() time.Time
}

// NewBudget создаёт дневной бюджет на max попыток LLM-вызовов в сутки (UTC).
func NewBudget(max int64) *Budget {
	return &Budget{max: max, now: time.Now}
}

// Allow списывает одну попытку: true — primary вызывать можно; false —
// бюджет дня исчерпан (идём в local-бэкенд). Сутки — календарные UTC,
// rollover сбрасывает счётчик.
func (b *Budget) Allow() bool {
	if b == nil || b.max <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if today := b.now().UTC().Format("2006-01-02"); today != b.day {
		b.day, b.used = today, 0
	}
	if b.used >= b.max {
		return false
	}
	b.used++
	return true
}

// Used возвращает число списанных попыток текущих суток (наблюдаемость).
func (b *Budget) Used() int64 {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}
