package payments

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Регрессия CRITICAL-04 (2026-09-27): application-сервис не имеет права
// подменять статус интента при отклонённом переходе.
//
// ДО фикса applyVerifiedEvent делал `intent.Status = newStatus; return event, nil`
// сразу после вызова ApplyVerifiedEventTx, а тот на отклонённом переходе
// возвращал nil (инфраструктура писала только warn-лог и всё равно журналировала
// событие). Итог: вызывающий получал успех и событие для интента, который
// в БД остался в прежнем терминальном статусе.

// c004Repo — fakeRepo, который умеет транзакционный путь и умеет ОТКЛОНЯТЬ
// переход, как это делает реальный payment_repo.
type c004Repo struct {
	fakeRepo
	// reject — статус, который считается терминальным и не переопределяется.
	// Пусто → переход принимается.
	rejectFrom Status
	applied    int
	events     []*PaymentEvent
}

func (r *c004Repo) ApplyVerifiedEventTx(_ context.Context, tenantID, intentID string, s Status, paidAt *time.Time, e *PaymentEvent) error {
	for _, p := range r.intents {
		if p.ID == intentID && p.TenantID == tenantID {
			if r.rejectFrom != "" && p.Status == r.rejectFrom && p.Status != s {
				return errors.New("rejected")
			}
			p.Status = s
			p.PaidAt = paidAt
			r.applied++
			// Реальный репозиторий делает RETURNING id, created_at.
			if e.ID == "" {
				r.nextID++
				e.ID = "evt-" + itoa(r.nextID)
			}
			e.CreatedAt = time.Now().UTC()
			r.events = append(r.events, e)
			return nil
		}
	}
	return ErrNotFound
}

func c004Setup(t *testing.T, rejectFrom Status) (*Service, *c004Repo) {
	t.Helper()
	repo := &c004Repo{rejectFrom: rejectFrom}
	svc := NewService(repo, &fakeProvider{name: "mock"}, &fakeVerifier{}, 0)
	intent := &PaymentIntent{
		TenantID: "t-1", UserID: "u-1", AmountMinor: 5000, Currency: "USD",
		Status: StatusPending, Provider: "mock", ProviderCheckoutID: "c004-1",
	}
	if err := repo.CreateIntent(context.Background(), intent); err != nil {
		t.Fatalf("create intent: %v", err)
	}
	return svc, repo
}

func c004Event(status string) []byte {
	return []byte(`{"status":"` + status + `","amount_minor":5000,"currency":"USD"}`)
}

// TestCRIT004_RejectedTransitionPropagatesError — отказ репозитория обязан
// дойти до вызывающего, иначе он подставит в свой объект статус, которого в БД
// нет, и вернёт событие, которого нет в журнале.
func TestCRIT004_RejectedTransitionPropagatesError(t *testing.T) {
	svc, repo := c004Setup(t, StatusPaid)

	// Оплата проходит.
	ev, err := svc.applyVerifiedEvent(context.Background(), "mock", "c004-1",
		"paid", 5000, "USD", c004Event("paid"))
	if err != nil {
		t.Fatalf("first event must be applied: %v", err)
	}
	if ev == nil || ev.ID == "" {
		t.Fatal("first event must be returned")
	}

	intent, _ := repo.GetIntent(context.Background(), "t-1", "int-1")
	if intent.Status != StatusPaid {
		t.Fatalf("status = %s, want paid", intent.Status)
	}

	// Позднее событие: переход отклонён репозиторием.
	_, err = svc.applyVerifiedEvent(context.Background(), "mock", "c004-1",
		"failed", 5000, "USD", c004Event("failed"))
	if err == nil {
		t.Fatal("CRITICAL-04: rejected transition must not look like success")
	}

	// И возвращённый объект не должен утверждать, что статус изменился.
	intent, err = repo.GetIntent(context.Background(), "t-1", "int-1")
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if intent.Status != StatusPaid {
		t.Errorf("status = %s, want paid after a rejected transition", intent.Status)
	}
	// В журнал записано только первое, реально применённое событие.
	if len(repo.events) != 1 {
		t.Errorf("journaled events = %d, want 1", len(repo.events))
	}
}

// TestCRIT004_RejectedTransitionKeepsPaidAt — отказ не должен и стирать
// paid_at: интент оплачен, и его оплаченность обязана быть видна.
func TestCRIT004_RejectedTransitionKeepsPaidAt(t *testing.T) {
	svc, repo := c004Setup(t, StatusPaid)
	if _, err := svc.applyVerifiedEvent(context.Background(), "mock", "c004-1",
		"paid", 5000, "USD", c004Event("paid")); err != nil {
		t.Fatalf("first event: %v", err)
	}
	intent, _ := repo.GetIntent(context.Background(), "t-1", "int-1")
	paidAt := intent.PaidAt
	if paidAt == nil {
		t.Fatal("paid_at must be set after payment")
	}

	if _, err := svc.applyVerifiedEvent(context.Background(), "mock", "c004-1",
		"failed", 5000, "USD", c004Event("failed")); err == nil {
		t.Fatal("want error on rejected transition")
	}
	intent, _ = repo.GetIntent(context.Background(), "t-1", "int-1")
	if intent.PaidAt == nil || !intent.PaidAt.Equal(*paidAt) {
		t.Errorf("paid_at = %v, want %v (must be preserved)", intent.PaidAt, paidAt)
	}
}
