package assistant

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Backend — бэкенд генерации комментария (AI-0003). Первичный бэкенд —
// опциональный OpenAI-совместимый HTTP-клиент; локальный — детерминированный
// эксперт. Ни один бэкенд не влияет на структурный ответ (Response) —
// рекомендация строится только из результатов тулов, модель лишь оформляет
// пояснение.
type Backend interface {
	// Infer генерирует текстовый комментарий по материалу Prompt.
	Infer(ctx context.Context, p Prompt) (*Answer, error)
}

// Prompt — материал для генерации комментария (без аудируемых и персональных
// данных; тенант/актор в промпт не попадают).
type Prompt struct {
	Kind           string // kind ассистента (label)
	Instructions   string // системная инструкция
	UserTask       string // исходная задача пользователя
	Context        string // нарратив результатов тулов
	StructuredData []byte // строгий JSON-контекст для первичного бэкенда
}

// Answer — текст комментария модели.
type Answer struct {
	Text string
}

// LocalCommenter — детерминированный комментатор (default-бэкенд). Должен
// возвращать непустую строку; при ошибке ModelRouter проваливает ответ.
type LocalCommenter func(ctx context.Context, p Prompt, resp *Response) (string, error)

// ModelRouter — маршрутизация комментария с фолбэком (AI-0003):
// primary → local → ошибка. Детерминированность структурного ответа не
// зависит от выбранного бэкенда.
type ModelRouter struct {
	primary Backend
	local   LocalCommenter
}

// NewModelRouter создаёт роутер; primary может быть nil (чисто локальный режим).
func NewModelRouter(primary Backend, local LocalCommenter) ModelRouter {
	if local == nil {
		// LocalCommenter обязателен: локальный режим — дефолт (AI-0003).
		panic("assistant: ModelRouter requires a local commenter")
	}
	return ModelRouter{primary: primary, local: local}
}

// Infer формирует ответ модели: пробует первичный бэкенд, при отсутствии,
// сбое или пустом ответе — локальный.
func (r ModelRouter) Infer(ctx context.Context, it *intent, resp *Response) (*Answer, error) {
	if it == nil || resp == nil {
		return nil, fmt.Errorf("%w: nil intent or response", ErrInvalid)
	}
	p := Prompt{
		Kind:           it.Kind,
		Instructions:   it.Instructions,
		UserTask:       it.UserTask,
		Context:        it.Context,
		StructuredData: it.Data,
	}

	if r.primary != nil {
		ans, err := r.primary.Infer(ctx, p)
		if err == nil && ans != nil && strings.TrimSpace(ans.Text) != "" {
			return ans, nil
		}
		if err != nil {
			slog.WarnContext(ctx, "assistant: primary backend failed, falling back to local",
				"kind", it.Kind, "err", err)
		}
	}

	text, err := r.local(ctx, p, resp)
	if err != nil {
		return nil, fmt.Errorf("assistant: local commenter: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		text = localFallback(resp)
	}
	return &Answer{Text: text}, nil
}
