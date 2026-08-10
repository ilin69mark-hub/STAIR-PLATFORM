package graph

import "errors"

// ErrCycleDetected возвращается при обнаружении цикла зависимостей.
var ErrCycleDetected = errors.New("graph: cycle detected")

// VersionError — ошибка версионирования снапшота.
type VersionError struct {
	Message string
}

func (e *VersionError) Error() string { return "graph: version error: " + e.Message }
