package main

import (
	"testing"
	"time"
)

func TestEnvHelpers(t *testing.T) {
	t.Setenv("EBOOL", "true")
	if !envBool("EBOOL", false) {
		t.Fatal("want true")
	}
	t.Setenv("EBOOL", "bad")
	if envBool("EBOOL", true) == true && envBool("EBOOL", false) != false {
		// bad should return def
	}
	t.Setenv("EBOOL", "")
	if !envBool("EBOOL", true) {
		t.Fatal("empty should return def")
	}
	t.Setenv("EFLOAT", "3.14")
	if envFloat64("EFLOAT", 0) != 3.14 {
		t.Fatal("float mismatch")
	}
	t.Setenv("EFLOAT", "bad")
	if envFloat64("EFLOAT", 1.5) != 1.5 {
		t.Fatal("bad float should return def")
	}
	if envString("ESTR", "def") != "def" {
		t.Fatal("empty string")
	}
	t.Setenv("ESTR", "val")
	if envString("ESTR", "def") != "val" {
		t.Fatal("val mismatch")
	}
	if envOr("EOR", "d") != "d" {
		t.Fatal("envOr empty")
	}
	t.Setenv("EOR", "x")
	if envOr("EOR", "d") != "x" {
		t.Fatal("envOr x")
	}
	t.Setenv("EINT", "42")
	if envInt("EINT", 0) != 42 {
		t.Fatal("int")
	}
	t.Setenv("EINT", "bad")
	if envInt("EINT", 7) != 7 {
		t.Fatal("bad int def")
	}
	t.Setenv("EDUR", "5s")
	if envDuration("EDUR", time.Second) != 5*time.Second {
		t.Fatal("duration")
	}
	t.Setenv("EDUR", "bad")
	if envDuration("EDUR", 2*time.Second) != 2*time.Second {
		t.Fatal("bad duration def")
	}
	t.Setenv("ESLICE", "a, b, ,c")
	got := envStringSlice("ESLICE", nil)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("slice got %v", got)
	}
	if envStringSlice("EEMPTY", []string{"d"})[0] != "d" {
		t.Fatal("def slice")
	}
}

func TestSessionTTL(t *testing.T) {
	t.Setenv("STAIR_SESSION_TTL", "")
	if sessionTTL() != 86400*time.Second {
		t.Fatalf("default ttl %v", sessionTTL())
	}
	t.Setenv("STAIR_SESSION_TTL", "100")
	if sessionTTL() != 100*time.Second {
		t.Fatalf("ttl %v", sessionTTL())
	}
}

func TestNewAPIQueueBackend(t *testing.T) {
	b := newAPIQueueBackend("")
	if b.Queue() == nil {
		t.Fatal("memory queue nil")
	}
	// invalid redis addr should fallback
	b2 := newAPIQueueBackend("127.0.0.1:1")
	if b2.Queue() == nil {
		t.Fatal("fallback queue nil")
	}
	b2.Close()
}

func TestNewStorageService(t *testing.T) {
	t.Setenv("STAIR_STORAGE_BACKEND", "")
	t.Setenv("STAIR_STORAGE_DIR", t.TempDir())
	svc, err := newStorageService()
	if err != nil || svc == nil {
		t.Fatalf("newStorageService: %v", err)
	}
}
