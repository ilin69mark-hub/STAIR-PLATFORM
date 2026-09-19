// Package circuitbreaker реализует Circuit Breaker паттерн для защиты
// от каскадных сбоев при вызове внешних сервисов (Stripe, storage, etc).
// Состояния: Closed → Open → HalfOpen → Closed.
package circuitbreaker

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"stairplatform/internal/infrastructure/metrics"
)

var (
	// CBRegistry — глобальный реестр метрик circuit breaker.
	CBRegistry = metrics.NewRegistry()

	// CBState — gauge текущего состояния (0=closed, 1=open, 2=half_open).
	CBState = CBRegistry.Gauge("circuit_breaker_state", "Circuit breaker state (0=closed, 1=open, 2=half_open)")

	// CBTransitions — счётчик переходов между состояниями.
	CBTransitions = CBRegistry.Counter(
		"circuit_breaker_transitions_total",
		"Circuit breaker state transitions",
		"from", "to",
	)

	// CBRequests — счётчик запросов через circuit breaker.
	CBRequests = CBRegistry.Counter(
		"circuit_breaker_requests_total",
		"Total requests through circuit breaker",
		"name", "result",
	)
)

var (
	// ErrCircuitOpen возвращается, когда circuit breaker в состоянии Open.
	ErrCircuitOpen = errors.New("circuit breaker: open")
	// ErrTooManyRequests возвращается, когда circuit breaker в HalfOpen
	// и превышен лимит одновременных запросов.
	ErrTooManyRequests = errors.New("circuit breaker: too many requests")
)

// State — состояние circuit breaker.
type State int

const (
	StateClosed   State = iota // нормальная работа
	StateOpen                  // блокировка запросов
	StateHalfOpen              // пробный режим
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// Settings — настройки circuit breaker.
type Settings struct {
	// FailureThreshold — количество ошибок для перехода в Open.
	FailureThreshold int
	// SuccessThreshold — количество успешных для возврата в Closed.
	SuccessThreshold int
	// Timeout — время в Open перед переходом в HalfOpen.
	Timeout time.Duration
	// MaxRequests — максимум запросов в HalfOpen для тестирования.
	MaxRequests int
}

// DefaultSettings настройки по умолчанию.
var DefaultSettings = Settings{
	FailureThreshold: 5,
	SuccessThreshold: 3,
	Timeout:          30 * time.Second,
	MaxRequests:      3,
}

// CircuitBreaker — реализует Circuit Breaker паттерн.
type CircuitBreaker struct {
	mu               sync.Mutex
	settings         Settings
	state            State
	failureCount     int
	successCount     int
	lastFailureTime  time.Time
	halfOpenRequests int
	name             string
	onStateChange    func(name string, from, to State)
}

// New создаёт новый Circuit Breaker с заданными настройками.
func New(name string, settings Settings) *CircuitBreaker {
	return &CircuitBreaker{
		settings: settings,
		state:    StateClosed,
		name:     name,
	}
}

// OnStateChange устанавливает callback при смене состояния.
func (cb *CircuitBreaker) OnStateChange(fn func(name string, from, to State)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Execute выполняет операцию через circuit breaker.
// Если circuit open — возвращает ErrCircuitOpen без вызова fn.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.allowRequest(); err != nil {
		return err
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// ExecuteWithRetry выполняет операцию с повторными попытками.
// При circuit open — ждёт timeout и пробует снова.
func (cb *CircuitBreaker) ExecuteWithRetry(fn func() error, maxRetries int) error {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		err := cb.Execute(fn)
		if err == nil {
			return nil
		}
		lastErr = err

		if errors.Is(err, ErrCircuitOpen) && i < maxRetries {
			// Ждём timeout перед следующей попыткой
			time.Sleep(cb.settings.Timeout)
			continue
		}
		if errors.Is(err, ErrTooManyRequests) && i < maxRetries {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		// Другие ошибки — не retry
		if !errors.Is(err, ErrCircuitOpen) && !errors.Is(err, ErrTooManyRequests) {
			return err
		}
	}
	return lastErr
}

// State возвращает текущее состояние.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.checkStateTransition()
	return cb.state
}

// Counts возвращает текущие счётчики.
func (cb *CircuitBreaker) Counts() (failures, successes int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failureCount, cb.successCount
}

func (cb *CircuitBreaker) allowRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.checkStateTransition()

	switch cb.state {
	case StateClosed:
		return nil
	case StateOpen:
		return ErrCircuitOpen
	case StateHalfOpen:
		if cb.halfOpenRequests < cb.settings.MaxRequests {
			cb.halfOpenRequests++
			return nil
		}
		return ErrTooManyRequests
	}
	return nil
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()
		CBRequests.With(cb.name, "failure").Inc()

		switch cb.state {
		case StateClosed:
			if cb.failureCount >= cb.settings.FailureThreshold {
				cb.setState(StateOpen)
			}
		case StateHalfOpen:
			cb.setState(StateOpen)
		case StateOpen:
		}
	} else {
		CBRequests.With(cb.name, "success").Inc()

		switch cb.state {
		case StateHalfOpen:
			cb.successCount++
			if cb.successCount >= cb.settings.SuccessThreshold {
				cb.setState(StateClosed)
			}
		case StateClosed:
			cb.successCount++
			// Сброс счётчика ошибок при успехе
			if cb.successCount > cb.settings.SuccessThreshold {
				cb.failureCount = 0
			}
		case StateOpen:
		}
	}
}

func (cb *CircuitBreaker) checkStateTransition() {
	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.settings.Timeout {
		cb.setState(StateHalfOpen)
	}
}

func (cb *CircuitBreaker) setState(newState State) {
	if cb.state == newState {
		return
	}
	oldState := cb.state
	cb.state = newState

	// Сброс счётчиков при переходе
	switch newState {
	case StateClosed:
		cb.failureCount = 0
		cb.successCount = 0
		cb.halfOpenRequests = 0
	case StateOpen:
		cb.successCount = 0
		cb.halfOpenRequests = 0
	case StateHalfOpen:
		cb.successCount = 0
		cb.halfOpenRequests = 0
	}

	// Обновляем метрики
	stateFloat := float64(newState)
	CBState.Set(stateFloat)
	CBTransitions.With(oldState.String(), newState.String()).Inc()

	slog.Info("circuit breaker state changed",
		"name", cb.name,
		"from", oldState.String(),
		"to", newState.String(),
	)

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, oldState, newState)
	}
}
