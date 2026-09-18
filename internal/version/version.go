// Package version предоставляет информацию о сборке STAIR PLATFORM.
// Значения Version/Commit/BuildTime заполняются через -ldflags (-X):
//
//	go build -ldflags \
//	  "-X stairplatform/internal/version.Version=v1.2.3 \
//	   -X stairplatform/internal/version.Commit=abc1234 \
//	   -X stairplatform/internal/version.BuildTime=2026-09-17T20:00:00Z"
package version

var (
	// Version — версия релиза (git tag / describe); по умолчанию "dev".
	Version = "dev"
	// Commit — короткий SHA коммита.
	Commit = "none"
	// BuildTime — время сборки (RFC3339 UTC).
	BuildTime = "unknown"
)

// String возвращает однострочное описание версии для логов метрик.
func String() string {
	return "stair-platform " + Version + " (" + Commit + ", " + BuildTime + ")"
}
