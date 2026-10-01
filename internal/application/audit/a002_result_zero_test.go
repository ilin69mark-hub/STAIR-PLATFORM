package audit

import (
	"context"
	"strings"
	"testing"
)

// AUDIT-002 (2026-09-27): забытое поле Result больше не «съедает» событие
// аудита молча.
//
// ДО фикса цепочка была такой: забытый Result → пустая строка →
// ErrInvalidEvent из Event.Validate → девять вызовов Record из двенадцати
// глотали ошибку через `_ =` → действие происходило, ответ клиенту был 200, а
// в журнале аудита не появлялось НИЧЕГО. Обнаружить дыру было нечем: ни
// ошибки, ни ворнинга.
//
// Теперь нулевое значение Result означает «ок» (безопасное умолчание), а
// НЕПУСТОЙ мусор по-прежнему отвергается громко.

// c002Repo — порт, который записывает попытки и умеет падать.
type c002Repo struct {
	inserted []*Event
	err      error
}

func (r *c002Repo) Insert(_ context.Context, e *Event) error {
	if r.err != nil {
		return r.err
	}
	r.inserted = append(r.inserted, e)
	return nil
}

// Списочные методы не используются тестом, но обязаны существовать для
// удовлетворения порту.
func (r *c002Repo) ListByProject(context.Context, string, string, int) ([]*Event, error) {
	return nil, nil
}
func (r *c002Repo) ListByTenant(context.Context, string, int) ([]*Event, error) {
	return nil, nil
}

func c002Svc(r *c002Repo) *Service {
	return &Service{repo: r}
}

func c002Event() *Event {
	return &Event{
		TenantID: "t-1",
		ActorID:  "u-1",
		Action:   ActionProjectModified,
		// Result намеренно НЕ задан — это и есть исходный дефект.
	}
}

// TestAUDIT002_ForgottenResultIsRecordedAsOK — ядро фикса: событие с
// незаполненным Result обязано попасть в журнал.
func TestAUDIT002_ForgottenResultIsRecordedAsOK(t *testing.T) {
	repo := &c002Repo{}
	svc := c002Svc(repo)

	e := c002Event()
	if err := svc.Record(context.Background(), e); err != nil {
		t.Fatalf("AUDIT-002: событие с незаполненным Result потеряно: %v", err)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("recorded events = %d, want 1", len(repo.inserted))
	}
	if repo.inserted[0].Result != ResultOK {
		t.Errorf("result = %q, want %q — в БД должна попасть непустая строка", repo.inserted[0].Result, ResultOK)
	}
}

// TestAUDIT002_ForgottenResultPassesValidation — Validate больше не отвергает
// пустой Result, но по-прежнему отвергает МУСОР. Ослабление проверки должно
// быть ровно настолько узким, насколько безопасно.
func TestAUDIT002_ForgottenResultPassesValidation(t *testing.T) {
	if err := c002Event().Validate(); err != nil {
		t.Errorf("пустой Result обязан проходить валидацию, got %v", err)
	}
	// Непустой мусор — ошибка программиста, она обязана остаться громкой.
	bad := c002Event()
	bad.Result = Result("teapot")
	if err := bad.Validate(); err == nil {
		t.Error("мусорный Result обязан отвергаться")
	}
	// Пустое действие по-прежнему отвергается: про Result забыть можно,
	// а вот действие без записи в реестре — это уже подделка.
	noAction := c002Event()
	noAction.Action = Action("client.invented")
	if err := noAction.Validate(); err == nil {
		t.Error("неизвестное действие обязано отвергаться")
	}
}

// TestAUDIT002_ExplicitResultsAreNotRewritten — приведение не должно
// трогать осмысленные исходы: «отказ» обязан остаться отказом, иначе журнал
// врал бы в другую сторону.
func TestAUDIT002_ExplicitResultsAreNotRewritten(t *testing.T) {
	for _, want := range []Result{ResultOK, ResultDenied, ResultFailed} {
		repo := &c002Repo{}
		svc := c002Svc(repo)
		e := c002Event()
		e.Result = want
		if err := svc.Record(context.Background(), e); err != nil {
			t.Fatalf("Record(%q): %v", want, err)
		}
		if got := repo.inserted[0].Result; got != want {
			t.Errorf("result = %q, want %q (осмысленный исход переписан)", got, want)
		}
	}
}

// TestAUDIT002_OrOKIsIdempotent — OrOK не меняет уже-нормализованное
// значение, поэтому безопасен в повторном применении.
func TestAUDIT002_OrOKIsIdempotent(t *testing.T) {
	if got := ResultUnset.OrOK(); got != ResultOK {
		t.Errorf("Unset.OrOK() = %q, want ok", got)
	}
	if got := ResultOK.OrOK(); got != ResultOK {
		t.Errorf("OK.OrOK() = %q, want ok", got)
	}
	if got := ResultDenied.OrOK(); got != ResultDenied {
		t.Errorf("Denied.OrOK() = %q, want denied", got)
	}
	if got := ResultFailed.OrOK(); got != ResultFailed {
		t.Errorf("Failed.OrOK() = %q, want failed", got)
	}
}

// TestAUDIT002_OrOKNeverInventsDenialOrFailure — умолчание «ок» безопасно
// именно потому, что не может замаскировать реальный отказ. Проверяем, что
// пустое поле физически не превращается ни в «denied», ни в «failed».
func TestAUDIT002_OrOKNeverInventsDenialOrFailure(t *testing.T) {
	got := ResultUnset.OrOK()
	if got == ResultDenied || got == ResultFailed {
		t.Fatalf("пустое поле превратилось в %q — реальные отказы будут замаскированы", got)
	}
	if got != ResultOK {
		t.Errorf("пустое поле должно давать ok, получено %q", got)
	}
}

// TestAUDIT002_ResultUnsetNeverReachesRepository — репозиторий не должен
// получить пустой Result: в БД стоит CHECK result IN
// ('ok','denied','failed'), и пустая строка дала бы техническую ошибку
// 23514 вместо записи события. Проверяем, что нормализация видна порту.
func TestAUDIT002_ResultUnsetNeverReachesRepository(t *testing.T) {
	repo := &c002Repo{}
	svc := c002Svc(repo)
	if err := svc.Record(context.Background(), c002Event()); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if repo.inserted[0].Result == ResultUnset {
		t.Error("пустой Result дошёл до репозитория — CHECK БД его отвергнет")
	}
}

// TestAUDIT002_PortFailureStillPropagates — нормализация НЕ должна превращать
// Record в «всегда успех». Отказ порта обязан доходить до вызывающего: иначе
// логирование отказа (шаг 2 фикса) было бы бессмысленным.
func TestAUDIT002_PortFailureStillPropagates(t *testing.T) {
	repo := &c002Repo{err: context.DeadlineExceeded}
	svc := c002Svc(repo)
	err := svc.Record(context.Background(), c002Event())
	if err == nil {
		t.Fatal("сбой порта обязан возвращаться вызывающему")
	}
	if !strings.Contains(err.Error(), "deadline") {
		t.Errorf("ожидалась исходная ошибка порта, получено %v", err)
	}
}
