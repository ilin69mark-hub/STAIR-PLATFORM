package assistant

import "stairplatform/internal/infrastructure/metrics"

// ServiceMetricsReg — реестр метрик AI Layer (Phase D, EDR-0036).
// Выводится в /metrics вместе с остальными реестрами; label-кардинальность
// низкая (kind/valid).
var ServiceMetricsReg = metrics.NewRegistry()

var (
	// assistantDuration — гистограмма времени подготовки ответа ассистента
	// (planning → tool calls → commentary), label kind/valid.
	assistantDuration = ServiceMetricsReg.Histogram(
		"stair_assistant_duration_seconds",
		"AI assistant response duration",
		nil, "kind", "valid",
	)

	// assistantLLMBudgetDenied — отказы дневного LLM-бюджета (S-148, S-141
	// №14): попытка ушла в local-бэкенд. Источник для алерта на бесконтрольный
	// расход OpenRouter.
	assistantLLMBudgetDenied = ServiceMetricsReg.Counter(
		"stair_assistant_llm_budget_denied_total",
		"AI assistant LLM calls denied by daily budget (local backend used)",
	)
)
