package funnel

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"
)

// Service принимает клиентские события и строит отчёты воронки.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService создаёт сервис. now внедряется, чтобы тесты не зависели от
// часов (отчёт строится по окну, и сдвиг времени ломает ожидания).
func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}
}

// Ingest проверяет и сохраняет пачку событий.
//
// Проверка строгая и ПОТОЧНАЯ: неизвестное имя, длинный пропс или отсутствие
// согласия отбрасывают конкретное событие, а не всю пачку. Иначе один
// сломанный вызов (например, после смены каталога на клиенте) выбросил бы
// весь визит, и в отчёте появился бы провал, которого не было.
func (s *Service) Ingest(ctx context.Context, sessionID string, events []Event, consentVersion int) (IngestResult, error) {
	if len(events) == 0 {
		return IngestResult{}, nil
	}
	if len(events) > MaxBatchEvents {
		return IngestResult{}, ErrBatchTooLarge
	}
	// Согласие проверяется ДО обработки и по всей пачке: версия одна на
	// визит, и подмешивать записи без согласия к согласованным нельзя.
	if consentVersion <= 0 || !CurrentConsentVersion(consentVersion) {
		return IngestResult{}, ErrConsentRequired
	}
	if !validSessionID(sessionID) {
		return IngestResult{}, ErrInvalidSession
	}

	res := IngestResult{}
	accepted := make([]Event, 0, len(events))
	now := s.now().UTC()
	for _, e := range events {
		clean, ok := s.sanitize(e, sessionID, consentVersion, now)
		if !ok {
			res.Dropped++
			continue
		}
		accepted = append(accepted, clean)
	}
	if len(accepted) == 0 {
		return res, nil
	}
	if err := s.repo.InsertEvents(ctx, accepted); err != nil {
		return IngestResult{}, err
	}
	res.Accepted = len(accepted)
	return res, nil
}

// sanitize приводит событие к тому виду, в котором оно может лечь в базу.
func (s *Service) sanitize(e Event, sessionID string, consentVersion int, now time.Time) (Event, bool) {
	if !KnownEvent(e.Name) {
		slog.Debug("funnel: неизвестное событие отброшено", "name", e.Name)
		return Event{}, false
	}
	props, ok := sanitizeProps(e.Props)
	if !ok {
		slog.Debug("funnel: пропы отброшены", "name", e.Name)
		return Event{}, false
	}
	ts := e.OccurredAt.UTC()
	if ts.IsZero() {
		ts = now
	}
	// Клиентские часы — в пределах суток от серверных, иначе событие уехало бы
	// в прошлое/будущее и выпало бы из окна отчёта.
	if delta := ts.Sub(now); delta > 24*time.Hour || delta < -24*time.Hour {
		ts = now
	}
	return Event{
		SessionID:      sessionID,
		Name:           e.Name,
		Props:          props,
		Path:           truncate(e.Path, MaxPathLen),
		Visitor:        truncate(e.Visitor, 32),
		Browser:        truncate(e.Browser, MaxBrowserLen),
		ConsentVersion: consentVersion,
		OccurredAt:     ts,
	}, true
}

// sanitizeProps оставляет только скаляры и режет длины. Ключи с непечатными
// символами и длинные значения выбрасываются — так в отчёт не попадёт то, что
// человек ввёл в поле формы.
func sanitizeProps(in map[string]any) (map[string]any, bool) {
	if len(in) == 0 {
		return map[string]any{}, true
	}
	if len(in) > MaxPropCount {
		return nil, false
	}
	out := make(map[string]any, len(in))
	budget := MaxPropBytes
	for k, v := range in {
		key := strings.TrimSpace(k)
		if key == "" || utf8.RuneCountInString(key) > MaxPropKeyLen {
			return nil, false
		}
		switch val := v.(type) {
		case string:
			if utf8.RuneCountInString(val) > MaxPropValueLen {
				return nil, false
			}
			budget -= len(val)
			out[key] = val
		case bool:
			out[key] = val
		case float64:
			// Числа ограничены по модулю: координаты и id не нужны, а
			// «бесконечность» в jsonb ложится некрасиво.
			if val > 1e15 || val < -1e15 || val != val {
				return nil, false
			}
			out[key] = val
		case float32:
			out[key] = float64(val)
		case int:
			out[key] = float64(val)
		case int64:
			out[key] = float64(val)
		case nil:
			// пустое значение — не причина терять событие
		default:
			// объект/массив в пропсе — это уже не метрика, а мусор
			return nil, false
		}
		if budget < 0 {
			return nil, false
		}
	}
	return out, true
}

// validSessionID принимает UUID. Мусорные строки из публичного эндпоинта не
// должны попадать в базу: колонка UUID их всё равно не примет, а ошибка
// пойдёт клиенту вместо тихого отказа.
func validSessionID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				return false
			}
		}
	}
	return true
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

// CurrentConsentVersion — версия политики, под которой согласие считается
// действующим. Временная (1×1=1), чтобы дёшево было её поднять.
func CurrentConsentVersion(v int) bool { return v == 1 }

// Report строит воронку за окно [from, to].
func (s *Service) Report(ctx context.Context, from, to time.Time) (*FunnelReport, error) {
	if to.Before(from) {
		return nil, fmt.Errorf("%w: to < from", ErrInvalidRange)
	}
	rep := &FunnelReport{From: from.UTC(), To: to.UTC(), Steps: []FunnelStep{}, Blockers: []Blocker{}, Abandons: []AbandonPoint{}}

	sessions, err := s.repo.SessionCount(ctx, from, to)
	if err != nil {
		return nil, err
	}
	rep.Sessions = sessions

	events, err := s.repo.EventCount(ctx, from, to)
	if err != nil {
		return nil, err
	}
	rep.Events = events

	stats, err := s.repo.StepCounts(ctx, from, to)
	if err != nil {
		return nil, err
	}
	prev := 0
	for _, name := range StepOrder() {
		st, ok := stats[name]
		if !ok {
			continue
		}
		share := 0.0
		stepShare := 0.0
		if sessions > 0 {
			share = float64(st.Sessions) / float64(sessions)
		}
		if prev > 0 {
			stepShare = float64(st.Sessions) / float64(prev)
		}
		rep.Steps = append(rep.Steps, FunnelStep{
			Name:      name,
			Sessions:  st.Sessions,
			Share:     share3(share),
			StepShare: share3(stepShare),
		})
		prev = st.Sessions
	}

	blockers, err := s.repo.TopBlockers(ctx, from, to, 10)
	if err != nil {
		return nil, err
	}
	rep.Blockers = blockers

	abandons, err := s.repo.AbandonPoints(ctx, from, to, 10)
	if err != nil {
		return nil, err
	}
	rep.Abandons = abandons

	avg, err := s.repo.AvgSessionSeconds(ctx, from, to)
	if err != nil {
		return nil, err
	}
	rep.AvgSecAll = share3(avg)

	return rep, nil
}

// ErrInvalidRange — окно отчёта перепутано.
var ErrInvalidRange = errRange{}

type errRange struct{}

func (errRange) Error() string { return "funnel: invalid time range" }

func share3(v float64) float64 {
	// Три знака достаточно для чтения процентов и убирает плавающий хвост в
	// JSON, который в UI выглядит как «33,333333333333336%».
	return float64(int64(v*1000+0.5)) / 1000
}
