package assistant

import (
	"context"
	"fmt"
	"strings"
)

// localComment — локальный детерминированный комментатор (дефолтный бэкенд,
// AI-0003). Не использует модель: оформляет русскоязычный комментарий из
// структуры Response. Используется как запасной бэкенд, а также когда
// первичный LLM недоступен или вернул пустоту.
func localComment(_ context.Context, _ Prompt, resp *Response) (string, error) {
	if resp == nil {
		return "", fmt.Errorf("%w: nil response", ErrInvalid)
	}
	var b strings.Builder

	if resp.Recommendation != "" {
		b.WriteString(resp.Recommendation)
		b.WriteString("\n")
	}

	if len(resp.Suggestions) > 0 {
		b.WriteString("\nДействия:\n")
		for i, sg := range resp.Suggestions {
			fmt.Fprintf(&b, "%d. %s", i+1, sg.Message)
			if sg.Rationale != "" {
				b.WriteString(" (" + sg.Rationale + ")")
			}
			b.WriteString("\n")
		}
	}

	warn, errNote := 0, 0
	for _, f := range resp.Findings {
		switch f.Severity {
		case "warning":
			warn++
		case "info":
			errNote++
		}
	}
	if warn > 0 || errNote > 0 {
		b.WriteString("\nЗамечания: ")
		parts := []string{}
		if warn > 0 {
			parts = append(parts, fmt.Sprintf("%d предупреждений", warn))
		}
		if errNote > 0 {
			parts = append(parts, fmt.Sprintf("%d уведомлений", errNote))
		}
		b.WriteString(strings.Join(parts, ", "))
		b.WriteString(".\n")
	}

	if len(resp.Alternatives) > 0 {
		b.WriteString("\nАльтернативные варианты (от лучшего к худшему):\n")
		for _, alt := range resp.Alternatives {
			fmt.Fprintf(&b, "- %s", alt.Title)
			if alt.Reason != "" {
				b.WriteString(" — " + alt.Reason)
			}
			b.WriteString("\n")
		}
	}

	if len(resp.Tradeoffs) > 0 {
		b.WriteString("\nКомпромиссы:\n")
		for _, to := range resp.Tradeoffs {
			b.WriteString("- " + to + "\n")
		}
	}

	return b.String(), nil
}

// localFallback — гарантированный минимальный комментарий, если локальный
// комментатор вернул пусто (страховка от пустого ответа).
func localFallback(resp *Response) string {
	if resp == nil {
		return "Рекомендация недоступна."
	}
	if resp.Recommendation == "" {
		return "Рекомендация сформирована; входная конфигурация требует уточнения."
	}
	return resp.Recommendation
}
