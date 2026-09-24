package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/payments"
)

// PaymentRepository — реализация порта payments.Repository на PostgreSQL
// (EDR-0027 §3.3) поверх пула pgx.
type PaymentRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentRepository создаёт репозиторий платежей.
func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

var _ payments.Repository = (*PaymentRepository)(nil)

const intentCols = `id, tenant_id, project_id, user_id, amount_minor, currency, status, provider, provider_checkout_id, created_at, updated_at, paid_at`

func scanIntent(row pgx.Row) (*payments.PaymentIntent, error) {
	var p payments.PaymentIntent
	var projectID, userID *string
	if err := row.Scan(&p.ID, &p.TenantID, &projectID, &userID, &p.AmountMinor, &p.Currency,
		&p.Status, &p.Provider, &p.ProviderCheckoutID, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt); err != nil {
		return nil, err
	}
	if projectID != nil {
		p.ProjectID = *projectID
	}
	if userID != nil {
		p.UserID = *userID
	}
	return &p, nil
}

// CreateIntent сохраняет новый интент.
func (r *PaymentRepository) CreateIntent(ctx context.Context, p *payments.PaymentIntent) error {
	var projectID, userID any
	if p.ProjectID != "" {
		projectID = p.ProjectID
	}
	if p.UserID != "" {
		userID = p.UserID
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO payment_intents (tenant_id, project_id, user_id, amount_minor, currency, status, provider, provider_checkout_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, created_at, updated_at`,
		p.TenantID, projectID, userID, p.AmountMinor, p.Currency, string(p.Status), p.Provider, p.ProviderCheckoutID,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("payments: create intent: %w", err)
	}
	return nil
}

// GetIntent возвращает интент по ID внутри tenant.
func (r *PaymentRepository) GetIntent(ctx context.Context, tenantID, id string) (*payments.PaymentIntent, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+intentCols+` FROM payment_intents WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	p, err := scanIntent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, payments.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("payments: get intent: %w", err)
	}
	return p, nil
}

// GetIntentByProviderCheckout возвращает интент по (provider, checkout_id).
func (r *PaymentRepository) GetIntentByProviderCheckout(ctx context.Context, provider, checkoutID string) (*payments.PaymentIntent, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+intentCols+` FROM payment_intents WHERE provider = $1 AND provider_checkout_id = $2`, provider, checkoutID)
	p, err := scanIntent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, payments.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("payments: get intent by checkout: %w", err)
	}
	return p, nil
}

// ListByProject возвращает интенты проекта в порядке создания.
func (r *PaymentRepository) ListByProject(ctx context.Context, tenantID, projectID string) ([]*payments.PaymentIntent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+intentCols+` FROM payment_intents WHERE tenant_id = $1 AND project_id = $2 ORDER BY created_at`, tenantID, projectID)
	if err != nil {
		return nil, fmt.Errorf("payments: list intents: %w", err)
	}
	defer rows.Close()
	var out []*payments.PaymentIntent
	for rows.Next() {
		p, err := scanIntent(rows)
		if err != nil {
			return nil, fmt.Errorf("payments: scan intent: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("payments: list intents rows: %w", err)
	}
	return out, nil
}

// terminalStatusGuardSQL — SQL-литерал терминальных статусов интента (см.
// payments.TerminalStatuses): событие PSP не может перевести интент из
// терминального статуса в другой статус — разрешён только повтор того же
// статуса (идемпотентная доставка, «succeeded→succeeded»). Защита от
// регрессии: поздний checkout.session.expired перетирал paid в failed
// (S-141 №4, CWE-20). Список собран из констант application-уровня, чтобы
// строки не дублировались.
var terminalStatusGuardSQL = func() string {
	statuses := payments.TerminalStatuses()
	q := make([]string, len(statuses))
	for i, s := range statuses {
		q[i] = "'" + string(s) + "'"
	}
	return strings.Join(q, ", ")
}()

// UpdateStatus обновляет статус и paid_at (nil — не менять). Терминальный
// статус нельзя перезаписать другим статусом: при отклонённом переходе
// (RowsAffected == 0) статус и paid_at не трогаются — только warn-лог
// (intent_id, from → to); метрик для этого случая нет (не выдумываем).
func (r *PaymentRepository) UpdateStatus(ctx context.Context, tenantID, id string, s payments.Status, paidAt *time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE payment_intents SET status = $1, paid_at = COALESCE($2, paid_at), updated_at = now()
		 WHERE tenant_id = $3 AND id = $4
		   AND NOT (status IN (`+terminalStatusGuardSQL+`) AND status <> $1)`,
		string(s), paidAt, tenantID, id)
	if err != nil {
		return fmt.Errorf("payments: update intent status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		warnRejectedStatusUpdate(ctx, r.pool, tenantID, id, s)
	}
	return nil
}

// ApplyVerifiedEventTx атомарно меняет статус интента и пишет событие в
// журнал payment_events (DB-002, forensic 2026-09-24). Одна транзакция:
// падение на AppendEvent откатывает и статус — «оплачено, но без следа» в
// финансовом аудите больше невозможно. SQL-guard терминальных статусов тот
// же, что в UpdateStatus.
func (r *PaymentRepository) ApplyVerifiedEventTx(ctx context.Context, tenantID, intentID string, s payments.Status, paidAt *time.Time, e *payments.PaymentEvent) error {
	return WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE payment_intents SET status = $1, paid_at = COALESCE($2, paid_at), updated_at = now()
			 WHERE tenant_id = $3 AND id = $4
			   AND NOT (status IN (`+terminalStatusGuardSQL+`) AND status <> $1)`,
			string(s), paidAt, tenantID, intentID)
		if err != nil {
			return fmt.Errorf("payments: update intent status: %w", err)
		}
		if tag.RowsAffected() == 0 {
			warnRejectedStatusUpdate(ctx, r.pool, tenantID, intentID, s)
		}
		return appendEventTx(ctx, tx, e)
	})
}

// appendEventTx пишет событие в журнал в рамках уже открытой транзакции.
func appendEventTx(ctx context.Context, tx pgx.Tx, e *payments.PaymentEvent) error {
	if err := tx.QueryRow(ctx,
		`INSERT INTO payment_events (tenant_id, intent_id, event_type, payload)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		e.TenantID, e.IntentID, e.EventType, e.Payload,
	).Scan(&e.ID, &e.CreatedAt); err != nil {
		return fmt.Errorf("payments: append event: %w", err)
	}
	return nil
}

// warnRejectedStatusUpdate логирует отклонённый переход статуса: интент не
// найден либо терминальный статус не может быть перезаписан другим статусом.
func warnRejectedStatusUpdate(ctx context.Context, pool *pgxpool.Pool, tenantID, id string, to payments.Status) {
	var from string
	err := pool.QueryRow(ctx,
		`SELECT status FROM payment_intents WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&from)
	switch {
	case err == nil:
		slog.Warn("payments: intent status transition rejected (terminal status)",
			"intent_id", id, "from", from, "to", string(to))
	case errors.Is(err, pgx.ErrNoRows):
		slog.Warn("payments: intent not found for status update",
			"intent_id", id, "to", string(to))
	default:
		slog.Warn("payments: failed to read intent status for rejected transition",
			"intent_id", id, "to", string(to), "error", err)
	}
}

// AppendEvent пишет событие в журнал payment_events.
func (r *PaymentRepository) AppendEvent(ctx context.Context, e *payments.PaymentEvent) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO payment_events (tenant_id, intent_id, event_type, payload)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		e.TenantID, e.IntentID, e.EventType, e.Payload,
	).Scan(&e.ID, &e.CreatedAt)
	if err != nil {
		return fmt.Errorf("payments: append event: %w", err)
	}
	return nil
}
