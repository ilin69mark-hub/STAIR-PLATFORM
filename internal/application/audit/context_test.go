package audit

import (
	"context"
	"testing"
)

func TestWithMetaRoundTrip(t *testing.T) {
	meta := Meta{RequestID: "req-1", IP: "127.0.0.1"}
	ctx := WithMeta(context.Background(), meta)
	got := MetaFrom(ctx)
	if got.RequestID != "req-1" || got.IP != "127.0.0.1" {
		t.Fatalf("got %+v want %+v", got, meta)
	}
}

func TestMetaFromEmpty(t *testing.T) {
	got := MetaFrom(context.Background())
	if got.RequestID != "" || got.IP != "" {
		t.Fatalf("expected zero, got %+v", got)
	}
}

func TestMetaFromNilContext(t *testing.T) {
	// Passing context with wrong type stored
	ctx := context.WithValue(context.Background(), metaCtxKey{}, "not-meta")
	got := MetaFrom(ctx)
	if got.RequestID != "" || got.IP != "" {
		t.Fatalf("expected zero for wrong type, got %+v", got)
	}
}

func TestWithMetaOverwrite(t *testing.T) {
	ctx := WithMeta(context.Background(), Meta{RequestID: "a"})
	ctx = WithMeta(ctx, Meta{RequestID: "b", IP: "1.1.1.1"})
	got := MetaFrom(ctx)
	if got.RequestID != "b" || got.IP != "1.1.1.1" {
		t.Fatalf("got %+v", got)
	}
}
