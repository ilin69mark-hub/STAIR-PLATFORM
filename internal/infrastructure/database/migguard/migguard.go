// Package migguard — защита миграций от необратимого «грязного» состояния.
//
// ПРОБЛЕМА (DB-1, forensic 2026-09-27). golang-migrate помечает версию
// dirty=true ДО выполнения файла миграции и снимает отметку только после
// успеха. Поэтому падение на любом шаге — включая жёсткий отказ на
// нарушающих legacy-данных — оставляет базу в состоянии, из которого
// `migrate up` больше не выйдет:
//
//	migrate: database: migrate up: Dirty database version 31. Fix and force version.
//
// Собственные средства восстановления у проекта отсутствовали: в cmd/migrate
// не было ни -force, ни -steps, а cmd/api/main.go вызывал Migrate(up) при
// каждом старте и делал os.Exit(1). Итог — одна строка старых данных
// permanently блокировала подъём приложения, и оператор должен был писать
// SQL руками.
//
// ЧТО ДЕЛАЕТ ПАКЕТ. Три независимые меры:
//
//  1. Preflight — read-only проверка ДО вызова Migrate. Проверяет данные
//     против ограничений, которые собирается наложить миграция, и
//     печатает нарушителей с идентификаторами. Ничего не меняет, версия
//     не пачкается, оператор видит проблему до того, как она станет
//     необратимой.
//
//  2. Force — явное восстановление версии. Ровно то, чего не хватало.
//
//  3. ErrDirtyState — типизированная ошибка, чтобы вызывающий (cmd/api)
//     отличал «миграция не прошла из-за данных» от «миграция не прошла
//     из-за сбоя БД» и печатал оператору точную команду, а не общий
//     «database migrate failed».
package migguard

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDirtyState — версия схемы помечена dirty: обычный Migrate up отвергнут.
var ErrDirtyState = errors.New("migguard: dirty migration state")

// ErrPreflightFailed — данные нарушают ограничения будущих миграций.
var ErrPreflightFailed = errors.New("migguard: preflight found data violations")

// violation — одно найденное нарушение: какое правило, в какой таблице и
// сколько строк.
type violation struct {
	Constraint string
	Table      string
	Detail     string
	Rows       int64
	Samples    []string
}

// checks — набор правил, зеркалящих ограничения миграций 000029/000031.
// Держится в коде, а не в SQL-файле, потому что обязан быть read-only и
// работать ДО того, как версия станет dirty.
//
// Каждое правило — пара «имя ограничения» + «диагностический SELECT».
// Смысл совпадает с соответствующим CHECK; при расхождении правила и
// CHECK диагностика врёт, поэтому TestPreflightMatchesMigration сверяет
// список имён с текстом 000031.
var checks = []struct {
	Constraint string
	Table      string
	Query      string
}{
	{
		Constraint: "projects_status_check",
		Table:      "projects",
		Query: `SELECT id::text FROM projects
			WHERE status IS NOT NULL
			  AND status NOT IN ('draft','in_review','approved','changes_requested')
			LIMIT 10`,
	},
	{
		Constraint: "stair_configurations_flight_check",
		Table:      "stair_configurations",
		Query: `SELECT id::text FROM stair_configurations
			WHERE flight IS NOT NULL
			  AND flight NOT IN ('straight','l_shape','u_shape','spiral')
			LIMIT 10`,
	},
	{
		Constraint: "stair_configurations_positive_geometry",
		Table:      "stair_configurations",
		Query: `SELECT id::text FROM stair_configurations
			WHERE width_mm <= 0 OR height_mm <= 0 OR step_height_mm <= 0
			   OR stringer_thickness_mm <= 0 OR step_thickness_mm <= 0
			   OR clearance_mm < 0 OR railing_height_mm < 0 OR comfort_step_mm < 0
			   OR revision < 1
			LIMIT 10`,
	},
	{
		Constraint: "payment_intents_amount_check",
		Table:      "payment_intents",
		Query:      `SELECT id::text FROM payment_intents WHERE amount_minor < 0 LIMIT 10`,
	},
	{
		Constraint: "integration_events_attempts_check",
		Table:      "integration_events",
		Query:      `SELECT id::text FROM integration_events WHERE attempts < 0 LIMIT 10`,
	},
	{
		Constraint: "integration_endpoints_tenant_kind_key",
		Table:      "integration_endpoints",
		Query: `SELECT min(tenant_id)::text FROM integration_endpoints
			GROUP BY tenant_id, kind HAVING count(*) > 1
			LIMIT 1`,
	},
	{
		Constraint: "audit_events_action_check",
		Table:      "audit_events",
		Query: `SELECT id::text FROM audit_events
			WHERE action NOT IN (
				'auth.login','auth.login_denied','auth.logout','auth.register',
				'api_key.created','api_key.revoked','authz.denied','data.exported',
				'settings.updated','user.role_changed','user.status_changed',
				'sso.linked','sso.login','sso.login_denied','member.added',
				'member.removed','member.role_changed','project.created',
				'project.modified','config.approved','config.restored',
				'review.changes','review.requested','review.signed',
				'ai.assist.design','ai.assist.engineering','ai.assist.manufacturing',
				'ai.assist.memory.purged','ai.assist.pricing','stair.calculated',
				'stair.config_changed','stair.live_suggestion_applied',
				'stair.live_variation_applied','stair.suggestion_applied',
				'stair.variation_applied','order.status_changed','payment.refunded',
				'testimonial.created','testimonial.deleted','testimonial.updated')
			LIMIT 10`,
	},
}

// Report — результат preflight: нарушения плюс версия схемы.
type Report struct {
	Version    int
	Dirty      bool
	Violations []Violation
}

// Violation — публичное описание одного нарушения.
type Violation struct {
	Constraint string
	Table      string
	Rows       int64
	Samples    []string
}

// Preflight проверяет данные против ограничений будущих миграций.
// Полностью read-only: ни DDL, ни DML, версия схемы не меняется.
//
// Отсутствие нарушений — это НЕ гарантия прохождения миграции: правила
// здесь повторяют CHECK, но составлены независимо и могут отстать. Поэтому
// preflight — диагностика, а не замена проверке миграции.
func Preflight(ctx context.Context, pool *pgxpool.Pool) (*Report, error) {
	rep := &Report{}

	// Версия/dirty читаются мягко: на «свежей» БД таблицы может не быть.
	_ = pool.QueryRow(ctx,
		`SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&rep.Version, &rep.Dirty)

	for _, c := range checks {
		// Таблицы могу�� отсутствовать (частичная схема) — это не нарушение.
		var count int64
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM (`+c.Query+`) s`).Scan(&count); err != nil {
			if isMissingRelation(err) {
				continue
			}
			return nil, fmt.Errorf("migguard: preflight %s: %w", c.Constraint, err)
		}
		if count == 0 {
			continue
		}
		samples := make([]string, 0, 10)
		rows, err := pool.Query(ctx, c.Query)
		if err != nil {
			if isMissingRelation(err) {
				continue
			}
			return nil, fmt.Errorf("migguard: preflight samples %s: %w", c.Constraint, err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, fmt.Errorf("migguard: scan %s: %w", c.Constraint, err)
			}
			samples = append(samples, id)
		}
		rows.Close()
		rep.Violations = append(rep.Violations, Violation{
			Constraint: c.Constraint, Table: c.Table, Rows: count, Samples: samples,
		})
	}
	return rep, nil
}

func isMissingRelation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "relation") && strings.Contains(msg, "does not exist")
}

// Failed сообщает, есть ли нарушения.
func (r *Report) Failed() bool { return len(r.Violations) > 0 }

// String — текстовый отчёт для оператора. Пишет, что делать, а не только
// что не так: идентификаторы строк бесполезны без следующего шага.
func (r *Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "schema version: %d (dirty=%v)\n", r.Version, r.Dirty)
	if !r.Failed() {
		b.WriteString("preflight: нарушений не найдено\n")
		return b.String()
	}
	fmt.Fprintf(&b, "preflight: найдено %d нарушений ограничений\n", len(r.Violations))
	for _, v := range r.Violations {
		fmt.Fprintf(&b, "\n  %s (таблица %s): строк — %d\n", v.Constraint, v.Table, v.Rows)
		for i, s := range v.Samples {
			if i >= 5 {
				fmt.Fprintf(&b, "    … и ещё %d\n", len(v.Samples)-i)
				break
			}
			fmt.Fprintf(&b, "    id=%s\n", s)
		}
	}
	b.WriteString("\nМиграция будет отвергнута этими данными. " +
		"Варианты: исправить/удалить строки вручную, либо (осознанная потеря " +
		"данных) удалить их и повторить: см. -force в cmd/migrate.\n")
	b.WriteString("ДО выполнения миграции проверьте решение: -preflight.\n")
	return b.String()
}

// Constraints возвращает имена проверяемых ограничений (для тестов
// сверки с текстом миграции).
func Constraints() []string {
	out := make([]string, 0, len(checks))
	for _, c := range checks {
		out = append(out, c.Constraint)
	}
	sort.Strings(out)
	return out
}
