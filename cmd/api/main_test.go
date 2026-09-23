package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeAccessor — мок configAccessor для pipelineSubscriberAuthorizer (S-132c).
type fakeAccessor struct {
	ok  bool
	err error
}

func (f *fakeAccessor) HasConfigAccess(_ context.Context, _, _ string) (bool, error) {
	if f.err != nil && !f.ok {
		return false, f.err
	}
	return f.ok, nil
}

// TestPipelineSubscriberAuthorizerAdminBypass: admin пропускается без
// обращения к сервису (S-132c) — даже если база недоступна.
func TestPipelineSubscriberAuthorizerAdminBypass(t *testing.T) {
	a := &pipelineSubscriberAuthorizer{svc: &fakeAccessor{ok: false, err: errors.New("db down")}}
	if !a.CanSubscribe("admin-1", "admin", "pipeline:123e4567-e89b-12d3-a456-426614174000") {
		t.Fatal("admin must bypass config access check")
	}
}

// TestPipelineSubscriberAuthorizerMember: не-админ — по членству; не-pipeline
// комната и пустой конфиг — отказ.
func TestPipelineSubscriberAuthorizerMember(t *testing.T) {
	a := &pipelineSubscriberAuthorizer{svc: &fakeAccessor{ok: true}}
	if !a.CanSubscribe("u-1", "user", "pipeline:123e4567-e89b-12d3-a456-426614174000") {
		t.Fatal("member with access must be allowed")
	}
	a = &pipelineSubscriberAuthorizer{svc: &fakeAccessor{ok: false}}
	if a.CanSubscribe("u-2", "user", "pipeline:123e4567-e89b-12d3-a456-426614174000") {
		t.Fatal("non-member must be denied")
	}
	if a.CanSubscribe("u-1", "user", "admin:news") {
		t.Fatal("non-pipeline room must be denied")
	}
	if a.CanSubscribe("u-1", "user", "pipeline:") {
		t.Fatal("empty config id must be denied")
	}
}

// TestNewRootHandlerPProfDisabled: pprof выключен по умолчанию —
// /debug/pprof/ отдаёт 404 через корневой catch-all (B2, EDR-0033 §3.3).
func TestNewRootHandlerPProfDisabled(t *testing.T) {
	t.Setenv("STAIR_PPROF_ENABLED", "")
	root := newRootHandler(http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("pprof must be disabled by default, got %d", rec.Code)
	}
}

// TestNewRootHandlerPProfEnabled: при STAIR_PPROF_ENABLED=true индекс
// pprof доступен.
func TestNewRootHandlerPProfEnabled(t *testing.T) {
	t.Setenv("STAIR_PPROF_ENABLED", "true")
	root := newRootHandler(http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.RemoteAddr = "127.0.0.1:12345" // InternalOnlyMiddleware пропускает только внутренние IP
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pprof index expected 200, got %d", rec.Code)
	}
}
