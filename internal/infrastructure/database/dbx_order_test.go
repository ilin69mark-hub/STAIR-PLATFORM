package database

import (
	"context"
	"errors"
	"testing"

	"stairplatform/internal/application/order"
)

// DB-8 (forensic 2026-09-27) — терминальность статуса заказа.
//
// БЫЛО: UpdateStatus шёл без гарда, `completed → new` проходил. Выполненный
// или отменённый заказ возвращался в «новый», агрегаты по воронке считали его
// активным, повторный триггер переоткрывал заказ. У платежей такой гард был
// (terminalStatusGuardSQL в payment_repo) — расхождение и стало источником.
func TestDB8_OrderTerminalStatusGuard(t *testing.T) {
	r, pr := newOrderRepo(t)
	owner := testOwnerID(t, pr, testTenantID(t, pr))
	ctx := context.Background()
	tenant := testTenantID(t, pr)

	mk := func(status order.Status) *order.Order {
		o := &order.Order{
			TenantID: tenant, UserID: owner, Kind: order.KindOrder, Status: status,
			Contact:    order.Contact{Name: "DB-8", Email: "db8@test.dev"},
			ConfigJSON: []byte(`{"width_mm":900}`),
			// 000033 (DB-9) связал kind с заполненностью: обычный заказ
			// обязан иметь цену и владельца, а консультация — наоборот.
			PriceJSON: []byte(`{"final_price":100}`),
		}
		if err := r.Create(ctx, o); err != nil {
			t.Fatalf("Create(%s): %v", status, err)
		}
		return o
	}

	// Завершённый заказ нельзя вернуть в работу.
	done := mk(order.StatusCompleted)
	if err := r.UpdateStatus(ctx, tenant, done.ID, order.StatusNew); !errors.Is(err, order.ErrTerminalStatus) {
		t.Fatalf("completed → new должен быть отвергнут, got %v", err)
	}
	got, err := r.Get(ctx, tenant, done.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != order.StatusCompleted {
		t.Errorf("status = %q, want completed — заказ не должен возвращаться в работу", got.Status)
	}

	// Отменённый — тоже терминальный.
	cancelled := mk(order.StatusCancelled)
	if err := r.UpdateStatus(ctx, tenant, cancelled.ID, order.StatusInProgress); !errors.Is(err, order.ErrTerminalStatus) {
		t.Fatalf("cancelled → in_progress должен быть отвергнут, got %v", err)
	}

	// Легитимная цепочка до терминального работает.
	wip := mk(order.StatusNew)
	for _, next := range []order.Status{order.StatusPriced, order.StatusConfirmed, order.StatusInProgress, order.StatusCompleted} {
		if err := r.UpdateStatus(ctx, tenant, wip.ID, next); err != nil {
			t.Fatalf("легитимный переход в %s отвергнут: %v", next, err)
		}
	}
	// Повтор того же терминального статуса — идемпотентная доставка, не переход.
	if err := r.UpdateStatus(ctx, tenant, wip.ID, order.StatusCompleted); err != nil {
		t.Errorf("повтор completed должен быть разрешён (идемпотентность): %v", err)
	}

	// Отсутствующий заказ — 404, а не «терминальный статус».
	err = r.UpdateStatus(ctx, tenant, "00000000-0000-0000-0000-0000db8c0001", order.StatusNew)
	if !errors.Is(err, order.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if errors.Is(err, order.ErrTerminalStatus) {
		t.Error("отсутствующий заказ не должен выглядеть как терминальный")
	}
}
