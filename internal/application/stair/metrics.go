package stair

import "stairplatform/internal/infrastructure/metrics"

// ServiceMetricsReg — реестр метрик прикладного слоя расчёта (B2,
// EDR-0033 §3.2, расширение EDR-0021). Выводится в /metrics вместе с
// HTTP-реестром; label-кардинальность низкая (flight/target/valid).
var ServiceMetricsReg = metrics.NewRegistry()

var (
	// calculateDuration — гистограмма времени полного конвейера
	// Calculate (solver → geometry → manufacturing → cost → price).
	calculateDuration = ServiceMetricsReg.Histogram(
		"stair_calculate_duration_seconds",
		"Stair pipeline calculation duration",
		nil, "flight", "valid",
	)

	// optimizeDuration — гистограмма времени поиска оптимума (EDR-0032).
	optimizeDuration = ServiceMetricsReg.Histogram(
		"stair_optimize_duration_seconds",
		"Stair optimization search duration",
		nil, "flight", "target",
	)
)
