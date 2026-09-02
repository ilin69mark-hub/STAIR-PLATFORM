package events

import "time"

// RetryPolicy определяет политику повторных попыток для failed event handlers.
type RetryPolicy struct {
	MaxAttempts int           // максимальное количество попыток (включая первую)
	MinDelay    time.Duration // начальная задержка
	MaxDelay    time.Duration // максимальная задержка
	Multiplier  float64       // множитель backoff
}

// DefaultRetryPolicy — дефолтная политика: 3 попытки, exponential backoff.
var DefaultRetryPolicy = RetryPolicy{
	MaxAttempts: 3,
	MinDelay:    50 * time.Millisecond,
	MaxDelay:    5 * time.Second,
	Multiplier:  2.0,
}

// Backoff вычисляет задержку для попытки attempt (0-indexed, первая попытка = 0).
func (p RetryPolicy) Backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	delay := float64(p.MinDelay)
	for i := 1; i < attempt; i++ {
		delay *= p.Multiplier
	}
	if time.Duration(delay) > p.MaxDelay {
		return p.MaxDelay
	}
	return time.Duration(delay)
}
