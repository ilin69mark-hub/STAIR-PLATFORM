package stair

import (
	"context"
	"testing"
)

func TestProbeCalcErr(t *testing.T) {
	svc := NewService()
	cfg := Config{}
	res, err := svc.calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("calculate err: %v", err)
	}
	_ = res
}
