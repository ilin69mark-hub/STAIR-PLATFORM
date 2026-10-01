package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/funnel"
)

// FunnelRepository — реализация funnel.Repository на PostgreSQL (BE-0005).
// Таблица web_events (миграция 000035). Персональных данных в ней нет по
// построению: visitor приходит уже хешем, IP и User-Agent не сохраняются.
type FunnelRepository struct {
	pool *pgxpool.Pool
}

// NewFunnelRepository создаёт репозиторий воронки поверх пула.
func NewFunnelRepository(pool *pgxpool.Pool) *FunnelRepository {
	return &FunnelRepository{pool: pool}
}

var _ funnel.Repository = (*FunnelRepository)(nil)

// InsertEvents сохраняет пачку в одной транзакции: отчёт воронки считается
// по сессиям, и частично записанный визит выглядел бы как «человек дошёл до
// середины и исчез».
//
// Insert, а не CopyFrom: props — jsonb, а pgx CopyFrom не умеет jsonb-параметры
// (bytea-энкодер лёг бы не туда). Пачка маленькая (≤ funnel.MaxBatchEvents),
// так что один многострочный INSERT дешевле копирования.
func (r *FunnelRepository) InsertEvents(ctx context.Context, events []funnel.Event) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("funnel: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const cols = 8
	args := make([]any, 0, len(events)*cols)
	rowsSQL := make([]string, 0, len(events))
	for i, e := range events {
		props, err := json.Marshal(e.Props)
		if err != nil {
			return fmt.Errorf("funnel: marshal props: %w", err)
		}
		b := i * cols
		args = append(args, e.SessionID, e.Name, props, e.Path, e.Visitor, e.Browser, e.ConsentVersion, e.OccurredAt)
		rowsSQL = append(rowsSQL, fmt.Sprintf(
			"($%d,$%d,$%d::jsonb,$%d,$%d,$%d,$%d,$%d)", b+1, b+2, b+3, b+4, b+5, b+6, b+7, b+8))
	}
	q := `INSERT INTO web_events
	        (session_id, name, props, path, visitor, browser, consent_version, created_at)
	      VALUES ` + strings.Join(rowsSQL, ",")
	if _, err := tx.Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("funnel: insert events: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("funnel: commit: %w", err)
	}
	return nil
}

// SessionCount — число сессий с любым событием в окне.
func (r *FunnelRepository) SessionCount(ctx context.Context, from, to time.Time) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT session_id) FROM web_events WHERE created_at BETWEEN $1 AND $2`,
		from, to,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("funnel: session count: %w", err)
	}
	return n, nil
}

// VisitorCount — число уникальных посетителей в окне. Считается по visitor
// (хеш IP+UA, сам IP не хранится). NULLIF нужен, чтобы пустая соль давала 0,
// а не «одного и того же анонимного посетителя»: без соли все строки имеют
// visitor=” и COUNT(DISTINCT) вернул бы 1 на любом трафике.
func (r *FunnelRepository) VisitorCount(ctx context.Context, from, to time.Time) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT NULLIF(visitor, '')) FROM web_events
		  WHERE created_at BETWEEN $1 AND $2`,
		from, to,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("funnel: visitor count: %w", err)
	}
	return n, nil
}

// EventCount — число событий в окне.
func (r *FunnelRepository) EventCount(ctx context.Context, from, to time.Time) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM web_events WHERE created_at BETWEEN $1 AND $2`,
		from, to,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("funnel: event count: %w", err)
	}
	return n, nil
}

// StepCounts — сессии по каждому имени события + средняя длительность сессии
// по этому шагу. Длительность берётся из props.session_seconds, который
// витрина проставляет в session.leave: считать её в SQL по разнице
// first/last события дорого и неверно для визитов с одним событием.
func (r *FunnelRepository) StepCounts(ctx context.Context, from, to time.Time) (map[string]funnel.StepStat, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT name,
		        COUNT(DISTINCT session_id) AS sessions,
		        COUNT(DISTINCT NULLIF(visitor, '')) AS visitors,
		        COALESCE(AVG(NULLIF(props->>'session_seconds', '')::double precision), 0) AS avg_sec
		   FROM web_events
		  WHERE created_at BETWEEN $1 AND $2
		  GROUP BY name`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("funnel: step counts: %w", err)
	}
	defer rows.Close()

	out := map[string]funnel.StepStat{}
	for rows.Next() {
		var name string
		var st funnel.StepStat
		if err := rows.Scan(&name, &st.Sessions, &st.Visitors, &st.AvgSecond); err != nil {
			return nil, fmt.Errorf("funnel: scan step counts: %w", err)
		}
		out[name] = st
	}
	return out, rows.Err()
}

// TopBlockers — самые частые блокировки. reason берётся из props.reason:
// для blocker.field_invalid это имя поля, для blocker.api_error — код ответа.
func (r *FunnelRepository) TopBlockers(ctx context.Context, from, to time.Time, limit int) ([]funnel.Blocker, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx,
		`SELECT name, COALESCE(props->>'reason', '') AS reason, COUNT(*) AS cnt
		   FROM web_events
		  WHERE created_at BETWEEN $1 AND $2
		    AND name IN ('blocker.field_invalid', 'blocker.api_error')
		  GROUP BY name, reason
		  ORDER BY cnt DESC, name
		  LIMIT $3`,
		from, to, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("funnel: top blockers: %w", err)
	}
	defer rows.Close()

	out := []funnel.Blocker{}
	for rows.Next() {
		var b funnel.Blocker
		if err := rows.Scan(&b.Event, &b.Reason, &b.Count); err != nil {
			return nil, fmt.Errorf("funnel: scan blockers: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// AbandonPoints — «где бросают»: последнее содержательное событие перед
// session.leave. Событие ищет САМ сервер (LATERAL по индексу
// (session_id, created_at)), а не берёт из пропа, который прислала витрина:
// единственный источник истины должен быть на нашей стороне, иначе отчёт
// «где отток» можно нарисовать клиентским кодом.
func (r *FunnelRepository) AbandonPoints(ctx context.Context, from, to time.Time, limit int) ([]funnel.AbandonPoint, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx,
		`SELECT COALESCE(prev.name, '(сразу ушёл)') AS last_event,
		        COUNT(*) AS sessions,
		        COALESCE(AVG(NULLIF(le.props->>'session_seconds', '')::double precision), 0) AS avg_sec
		   FROM web_events le
		   LEFT JOIN LATERAL (
		        SELECT e.name
		          FROM web_events e
		         WHERE e.session_id = le.session_id
		           AND (e.created_at, e.id) < (le.created_at, le.id)
		           AND e.name <> 'session.leave'
		         ORDER BY e.created_at DESC, e.id DESC
		         LIMIT 1
		   ) prev ON TRUE
		  WHERE le.created_at BETWEEN $1 AND $2
		    AND le.name = 'session.leave'
		  GROUP BY 1
		  ORDER BY sessions DESC, last_event
		  LIMIT $3`,
		from, to, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("funnel: abandon points: %w", err)
	}
	defer rows.Close()

	out := []funnel.AbandonPoint{}
	for rows.Next() {
		var a funnel.AbandonPoint
		if err := rows.Scan(&a.LastEvent, &a.Sessions, &a.AvgSec); err != nil {
			return nil, fmt.Errorf("funnel: scan abandon points: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AvgSessionSeconds — средняя длительность визита.
func (r *FunnelRepository) AvgSessionSeconds(ctx context.Context, from, to time.Time) (float64, error) {
	var v *float64
	err := r.pool.QueryRow(ctx,
		`SELECT AVG(NULLIF(props->>'session_seconds', '')::double precision)
		   FROM web_events
		  WHERE created_at BETWEEN $1 AND $2 AND name = 'session.leave'`,
		from, to,
	).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("funnel: avg session seconds: %w", err)
	}
	if v == nil {
		return 0, nil
	}
	return *v, nil
}

// Cleanup удаляет события старше порога.
func (r *FunnelRepository) Cleanup(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM web_events WHERE created_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("funnel: cleanup: %w", err)
	}
	return tag.RowsAffected(), nil
}
